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
	"net/url"
	"os"
	"time"

	apiextensionsapiserver "k8s.io/apiextensions-apiserver/pkg/apiserver"
	apiextensionsserveroptions "k8s.io/apiextensions-apiserver/pkg/cmd/server/options"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/apiserver/pkg/util/proxy"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	corev1 "k8s.io/client-go/listers/core/v1"
)

// CreateExtensions creates the Exensions Server.
func CreateExtensions() (genericapiserver.Config, genericoptions.EtcdOptions, *apiextensionsapiserver.CustomResourceDefinitions, error) {
	o := apiextensionsserveroptions.NewCustomResourceDefinitionsServerOptions(os.Stdout, os.Stderr)
	o.RecommendedOptions.Etcd.StorageConfig.Transport.ServerList = []string{"http://127.0.0.1:2379"}
	o.RecommendedOptions.SecureServing.BindPort = 6443
	o.RecommendedOptions.Authentication.RemoteKubeConfigFileOptional = true
	o.RecommendedOptions.Authorization.RemoteKubeConfigFileOptional = true
	o.RecommendedOptions.Authorization.AlwaysAllowPaths = []string{"*"}
	o.RecommendedOptions.Authorization.AlwaysAllowGroups = []string{"system:unauthenticated"}
	o.RecommendedOptions.CoreAPI = nil
	o.RecommendedOptions.Admission = nil

	if err := o.Complete(); err != nil {
		return genericapiserver.Config{}, *o.RecommendedOptions.Etcd, nil, err
	}

	if err := o.Validate(); err != nil {
		return genericapiserver.Config{}, *o.RecommendedOptions.Etcd, nil, err
	}

	// TODO have a "real" external address
	if err := o.RecommendedOptions.SecureServing.MaybeDefaultWithSelfSignedCerts("localhost", nil, []net.IP{net.ParseIP("127.0.0.1")}); err != nil {
		return genericapiserver.Config{}, *o.RecommendedOptions.Etcd, nil, fmt.Errorf("error creating self-signed certificates: %w", err)
	}

	serverConfig := genericapiserver.NewRecommendedConfig(apiextensionsapiserver.Codecs)
	if err := o.RecommendedOptions.ApplyTo(serverConfig); err != nil {
		return genericapiserver.Config{}, *o.RecommendedOptions.Etcd, nil, err
	}

	if err := o.APIEnablement.ApplyTo(&serverConfig.Config, apiextensionsapiserver.DefaultAPIResourceConfigSource(), apiextensionsapiserver.Scheme); err != nil {
		return serverConfig.Config, *o.RecommendedOptions.Etcd, nil, err
	}

	// TODO: fake it until we make it
	serverConfig.SharedInformerFactory = informers.NewSharedInformerFactory(fake.NewSimpleClientset(), 10*time.Minute)

	config := &apiextensionsapiserver.Config{
		GenericConfig: serverConfig,
		ExtraConfig: apiextensionsapiserver.ExtraConfig{
			CRDRESTOptionsGetter: apiextensionsserveroptions.NewCRDRESTOptionsGetter(*o.RecommendedOptions.Etcd),
			ServiceResolver:      &serviceResolver{serverConfig.SharedInformerFactory.Core().V1().Services().Lister()},
			MasterCount:          1,
		},
	}

	server, err := config.Complete().New(genericapiserver.NewEmptyDelegate())
	if err != nil {
		return serverConfig.Config, *o.RecommendedOptions.Etcd, nil, err
	}

	return serverConfig.Config, *o.RecommendedOptions.Etcd, server, nil
}

type serviceResolver struct {
	services corev1.ServiceLister
}

func (r *serviceResolver) ResolveEndpoint(namespace, name string, port int32) (*url.URL, error) {
	return proxy.ResolveCluster(r.services, namespace, name, port)
}
