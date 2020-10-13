/*
Copyright 2020 The Kubernetes Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package apiserver

import (
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
	corescheme "github.com/thetirefire/badidea/apiserver/scheme"
	coreopenapi "github.com/thetirefire/badidea/generated/openapi"
	apiextensionsapiserver "k8s.io/apiextensions-apiserver/pkg/apiserver"
	apiextensionsopenapi "k8s.io/apiextensions-apiserver/pkg/generated/openapi"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	openapinamer "k8s.io/apiserver/pkg/endpoints/openapi"
	"k8s.io/apiserver/pkg/server"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/dynamiccertificates"
	"k8s.io/apiserver/pkg/server/filters"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/apiserver/pkg/storage/storagebackend"
	"k8s.io/apiserver/pkg/util/feature"
	clientgoinformers "k8s.io/client-go/informers"
	clientgoclientset "k8s.io/client-go/kubernetes"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/klog/v2"
	v1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	aggregatorapiserver "k8s.io/kube-aggregator/pkg/apiserver"
	aggregatorscheme "k8s.io/kube-aggregator/pkg/apiserver/scheme"
	aggregatoropenapi "k8s.io/kube-aggregator/pkg/generated/openapi"
	"k8s.io/kube-openapi/pkg/common"
)

// CreateServerChain creates the chained aggregated server.
func CreateServerChain() (*aggregatorapiserver.APIAggregator, error) {
	etcdOpts := genericoptions.NewEtcdOptions(storagebackend.NewDefaultConfig("/registry", corescheme.Codecs.LegacyCodec(v1.SchemeGroupVersion)))
	etcdOpts.StorageConfig.Transport.ServerList = []string{"unix://etcd-socket:2379"}

	secureServingOpts := genericoptions.NewSecureServingOptions().WithLoopback()
	secureServingOpts.BindAddress = net.ParseIP("127.0.0.1")
	secureServingOpts.BindPort = 6443

	authnOpts := genericoptions.NewDelegatingAuthenticationOptions()
	authnOpts.RemoteKubeConfigFileOptional = true

	authzOpts := genericoptions.NewDelegatingAuthorizationOptions()
	authzOpts.RemoteKubeConfigFileOptional = true
	authzOpts.AlwaysAllowPaths = []string{"*"}
	authzOpts.AlwaysAllowGroups = []string{"system:unauthenticated"}

	admissionOpts := genericoptions.NewAdmissionOptions()

	recommendedConfig := genericapiserver.NewRecommendedConfig(corescheme.Codecs)

	errs := []error{}
	errs = append(errs, etcdOpts.Validate()...)
	errs = append(errs, secureServingOpts.Validate()...)
	errs = append(errs, authnOpts.Validate()...)
	errs = append(errs, authzOpts.Validate()...)
	errs = append(errs, admissionOpts.Validate()...)

	if len(errs) > 0 {
		return nil, utilerrors.NewAggregate(errs)
	}

	if err := etcdOpts.ApplyTo(&recommendedConfig.Config); err != nil {
		return nil, err
	}

	secureServingOpts.SecureServingOptions.ApplyTo(&recommendedConfig.SecureServing)

	// create self-signed cert+key with the fake server.LoopbackClientServerNameOverride and
	// let the server return it when the loopback client connects.
	certPem, keyPem, err := certutil.GenerateSelfSignedCertKey(server.LoopbackClientServerNameOverride, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate self-signed certificate for loopback connection: %v", err)
	}

	certProvider, err := dynamiccertificates.NewStaticSNICertKeyContent("self-signed loopback", certPem, keyPem, server.LoopbackClientServerNameOverride)
	if err != nil {
		return nil, fmt.Errorf("failed to generate self-signed certificate for loopback connection: %v", err)
	}

	recommendedConfig.SecureServing.SNICerts = []dynamiccertificates.SNICertKeyContentProvider{certProvider}

	secureLoopbackClientConfig, err := recommendedConfig.SecureServing.NewLoopbackClientConfig(uuid.New().String(), certPem)
	if err != nil {
		return nil, err
	}

	recommendedConfig.LoopbackClientConfig = secureLoopbackClientConfig

	getOpenAPIConfig := func(ref common.ReferenceCallback) map[string]common.OpenAPIDefinition {
		result := coreopenapi.GetOpenAPIDefinitions(ref)
		for k, v := range apiextensionsopenapi.GetOpenAPIDefinitions(ref) {
			result[k] = v
		}

		for k, v := range aggregatoropenapi.GetOpenAPIDefinitions(ref) {
			result[k] = v
		}

		return result
	}

	recommendedConfig.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(getOpenAPIConfig, openapinamer.NewDefinitionNamer(corescheme.Scheme, apiextensionsapiserver.Scheme, aggregatorscheme.Scheme))
	recommendedConfig.OpenAPIConfig.Info.Title = "BadIdea"
	recommendedConfig.OpenAPIConfig.Info.Version = "0.1"
	recommendedConfig.LongRunningFunc = filters.BasicLongRunningRequestCheck(
		sets.NewString("watch"),
		sets.NewString(),
	)

	if err := authnOpts.ApplyTo(&recommendedConfig.Authentication, recommendedConfig.SecureServing, recommendedConfig.OpenAPIConfig); err != nil {
		return nil, err
	}

	if err := authzOpts.ApplyTo(&recommendedConfig.Authorization); err != nil {
		return nil, err
	}

	kubeClientConfig := recommendedConfig.LoopbackClientConfig

	clientgoExternalClient, err := clientgoclientset.NewForConfig(kubeClientConfig)
	if err != nil {
		return nil, err
	}

	versionedInformers := clientgoinformers.NewSharedInformerFactory(clientgoExternalClient, 10*time.Minute)

	if err := admissionOpts.ApplyTo(&recommendedConfig.Config, versionedInformers, kubeClientConfig, feature.DefaultFeatureGate); err != nil {
		return nil, err
	}

	completed := recommendedConfig.Complete()

	new, err := completed.New("test", genericapiserver.NewEmptyDelegate())
	if err != nil {
		return nil, err
	}

	klog.Infof("new: %v", new)

	genericConfig, genericEtcdOptions, extensionServer, err := CreateExtensions()
	if err != nil {
		return nil, err
	}

	coreConfig, err := CreateCoreConfig(genericConfig, genericEtcdOptions)
	if err != nil {
		return nil, err
	}

	coreServer, err := coreConfig.Complete().NewWithDelegate(extensionServer.GenericAPIServer)
	if err != nil {
		return nil, err
	}

	aggregatorConfig, err := CreateAggregatorConfig(genericConfig, genericEtcdOptions)
	if err != nil {
		return nil, err
	}

	return CreateAggregatorServer(aggregatorConfig, coreServer.GenericAPIServer, extensionServer.Informers)
}
