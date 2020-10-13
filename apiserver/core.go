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
	"time"

	"github.com/thetirefire/badidea/apis/core"
	v1 "github.com/thetirefire/badidea/apis/core/v1"
	corescheme "github.com/thetirefire/badidea/apiserver/scheme"
	"github.com/thetirefire/badidea/controllers/namespace"
	coreopenapi "github.com/thetirefire/badidea/generated/openapi"
	"github.com/thetirefire/badidea/registry"
	namespacestorage "github.com/thetirefire/badidea/registry/core/namespace"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/version"
	openapinamer "k8s.io/apiserver/pkg/endpoints/openapi"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/filters"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/apiserver/pkg/server/resourceconfig"
	serverstorage "k8s.io/apiserver/pkg/server/storage"
	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/metadata"
)

// ExtraConfig holds custom apiserver config.
type ExtraConfig struct {
	// Place you custom config here.
}

// Config defines the config for the apiserver.
type Config struct {
	GenericConfig *genericapiserver.RecommendedConfig
	ExtraConfig   ExtraConfig
}

// CoreServer contains state for a Kubernetes cluster api server.
type CoreServer struct {
	GenericAPIServer *genericapiserver.GenericAPIServer
}

type completedConfig struct {
	GenericConfig genericapiserver.CompletedConfig
	ExtraConfig   *ExtraConfig
}

// CompletedConfig embeds a private pointer that cannot be instantiated outside of this package.
type CompletedConfig struct {
	*completedConfig
}

// Complete fills in any fields not set that are required to have valid data. It's mutating the receiver.
func (cfg *Config) Complete() CompletedConfig {
	c := completedConfig{
		cfg.GenericConfig.Complete(),
		&cfg.ExtraConfig,
	}

	c.GenericConfig.Version = &version.Info{
		Major: "0",
		Minor: "1",
	}

	return CompletedConfig{&c}
}

// New returns a new instance of CoreServer from the given config.
func (c completedConfig) NewWithDelegate(delegateAPIServer genericapiserver.DelegationTarget) (*CoreServer, error) {
	genericServer, err := c.GenericConfig.New("core-apiserver", delegateAPIServer)
	if err != nil {
		return nil, err
	}

	s := &CoreServer{
		GenericAPIServer: genericServer,
	}

	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(core.GroupName, corescheme.Scheme, metav1.ParameterCodec, corescheme.Codecs)

	v1storage := map[string]rest.Storage{}
	v1storage["namespaces"] = registry.RESTInPeace(namespacestorage.NewREST(corescheme.Scheme, c.GenericConfig.RESTOptionsGetter))
	apiGroupInfo.VersionedResourcesStorageMap["v1"] = v1storage

	if err := s.GenericAPIServer.InstallLegacyAPIGroup(genericapiserver.DefaultLegacyAPIPrefix, &apiGroupInfo); err != nil {
		return nil, err
	}

	coreClient, err := clientset.NewForConfig(c.GenericConfig.LoopbackClientConfig)
	if err != nil {
		return nil, err
	}

	metadataClient, err := metadata.NewForConfig(c.GenericConfig.LoopbackClientConfig)
	if err != nil {
		return nil, err
	}

	discoverResourcesFn := coreClient.Discovery().ServerPreferredNamespacedResources

	informerFactory := informers.NewSharedInformerFactory(coreClient, 5*time.Minute)

	namespaceController := namespace.NewNamespaceController(
		coreClient,
		metadataClient,
		discoverResourcesFn,
		informerFactory.Core().V1().Namespaces(),
		5*time.Minute,
		corev1.FinalizerKubernetes,
	)

	err = genericServer.AddPostStartHook("namespace-controller", func(context genericapiserver.PostStartHookContext) error {
		go namespaceController.Run(10, context.StopCh)

		return nil
	})
	if err != nil {
		return nil, err
	}

	return s, nil
}

func CreateCoreConfig(sharedConfig genericapiserver.Config, sharedEtcdOptions genericoptions.EtcdOptions) (*Config, error) {
	// make a shallow copy to let us twiddle a few things
	// most of the config actually remains the same.  We only need to mess with a couple items related to the particulars of the aggregator
	genericConfig := sharedConfig
	genericConfig.PostStartHooks = map[string]genericapiserver.PostStartHookConfigEntry{}
	genericConfig.RESTOptionsGetter = nil

	genericConfig.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(coreopenapi.GetOpenAPIDefinitions, openapinamer.NewDefinitionNamer(corescheme.Scheme, corescheme.Scheme))
	genericConfig.OpenAPIConfig.Info.Title = "BadIdea"
	genericConfig.OpenAPIConfig.Info.Version = "0.1"
	genericConfig.LongRunningFunc = filters.BasicLongRunningRequestCheck(
		sets.NewString("watch"),
		sets.NewString(),
	)

	// copy the etcd options so we don't mutate originals.
	etcdOptions := sharedEtcdOptions
	etcdOptions.StorageConfig.Codec = corescheme.Codecs.LegacyCodec(v1.SchemeGroupVersion)
	etcdOptions.StorageConfig.EncodeVersioner = runtime.NewMultiGroupVersioner(core.SchemeGroupVersion, schema.GroupKind{Group: core.GroupName})
	genericConfig.RESTOptionsGetter = &genericoptions.SimpleRestOptionsFactory{Options: etcdOptions}

	// override MergedResourceConfig with core defaults and registry
	// trying nil, since this is sourced from k8s.io/component-base/cli/flag.ConfigurationMap
	mergedResourceConfig, err := resourceconfig.MergeAPIResourceConfigs(DefaultAPIResourceConfigSource(), nil, corescheme.Scheme)
	if err != nil {
		return nil, err
	}

	genericConfig.MergedResourceConfig = mergedResourceConfig

	config := &Config{
		GenericConfig: &genericapiserver.RecommendedConfig{
			Config: genericConfig,
		},
	}

	// we need to clear the poststarthooks so we don't add them multiple times to all the servers (that fails)
	config.GenericConfig.PostStartHooks = map[string]genericapiserver.PostStartHookConfigEntry{}

	return config, nil
}

// DefaultAPIResourceConfigSource returns default configuration for an APIResource.
func DefaultAPIResourceConfigSource() *serverstorage.ResourceConfig {
	ret := serverstorage.NewResourceConfig()
	// NOTE: GroupVersions listed here will be enabled by default. Don't put alpha versions in the list.
	ret.EnableVersions(
		v1.SchemeGroupVersion,
	)

	return ret
}
