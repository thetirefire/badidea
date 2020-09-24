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
	"os"
	"strings"
	"time"

	"github.com/thetirefire/badidea/apis/badidea"
	"github.com/thetirefire/badidea/apis/badidea/install"
	"github.com/thetirefire/badidea/apis/badidea/v1alpha1"
	"github.com/thetirefire/badidea/pkg/generated/openapi"
	extensionsapiserver "k8s.io/apiextensions-apiserver/pkg/apiserver"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	openapinamer "k8s.io/apiserver/pkg/endpoints/openapi"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/filters"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/apiserver/pkg/server/options/encryptionconfig"
	"k8s.io/apiserver/pkg/server/resourceconfig"
	"k8s.io/apiserver/pkg/server/storage"
	"k8s.io/apiserver/pkg/storage/etcd3/preflight"
	"k8s.io/apiserver/pkg/storage/storagebackend"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	aggregatorapiserver "k8s.io/kube-aggregator/pkg/apiserver"
	aggregatorscheme "k8s.io/kube-aggregator/pkg/apiserver/scheme"
)

const (
	defaultEtcdPathPrefix = "/registry/badidea.x-k8s.io"
	etcdRetryLimit        = 60
	etcdRetryInterval     = 1 * time.Second
)

var (
	// Scheme defines methods for serializing and deserializing API objects.
	Scheme = runtime.NewScheme()
	// Codecs provides methods for retrieving codecs and serializers for specific
	// versions and content types.
	Codecs = serializer.NewCodecFactory(Scheme)
)

func init() {
	install.Install(Scheme)

	// we need to add the options to empty v1
	// TODO fix the server code to avoid this
	metav1.AddToGroupVersion(Scheme, schema.GroupVersion{Version: "v1"})

	// TODO: keep the generic API server from wanting this
	unversioned := schema.GroupVersion{Group: "", Version: "v1"}
	Scheme.AddUnversionedTypes(unversioned,
		&metav1.Status{},
		&metav1.APIVersions{},
		&metav1.APIGroupList{},
		&metav1.APIGroup{},
		&metav1.APIResourceList{},
	)
}

// ExtraConfig holds custom apiserver config.
type ExtraConfig struct {
	// Place you custom config here.
	APIResourceConfigSource storage.APIResourceConfigSource
	StorageFactory          storage.StorageFactory
	VersionedInformers      informers.SharedInformerFactory
}

// Config defines configuration for the apiserver.
type BadIdeaServerConfig struct {
	GenericConfig *genericapiserver.Config
	ExtraConfig   ExtraConfig
}

// BadIdeaServer contains state for a Kubernetes cluster master/api server.
type BadIdeaServer struct {
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
func (cfg *BadIdeaServerConfig) Complete() CompletedConfig {
	c := completedConfig{
		cfg.GenericConfig.Complete(cfg.ExtraConfig.VersionedInformers),
		&cfg.ExtraConfig,
	}

	return CompletedConfig{&c}
}

// New returns a new instance of BadIdeaServer from the given config.
func (c completedConfig) New(delegationTarget genericapiserver.DelegationTarget) (*BadIdeaServer, error) {
	genericServer, err := c.GenericConfig.New("badidea", delegationTarget)
	if err != nil {
		return nil, err
	}

	s := &BadIdeaServer{
		GenericAPIServer: genericServer,
	}

	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(badidea.GroupName, Scheme, metav1.ParameterCodec, Codecs)

	v1alpha1storage := map[string]rest.Storage{}
	apiGroupInfo.VersionedResourcesStorageMap["v1alpha1"] = v1alpha1storage

	if err := s.GenericAPIServer.InstallAPIGroup(&apiGroupInfo); err != nil {
		return nil, err
	}

	return s, nil
}

// ServerRunOptions runs a badidea api server.
type ServerRunOptions struct {
	GenericServerRunOptions *genericoptions.ServerRunOptions
	Etcd                    *genericoptions.EtcdOptions
	SecureServing           *genericoptions.SecureServingOptionsWithLoopback

	EnableAggregatorRouting bool
}

// NewServerRunOptions creates a new ServerRunOptions object with default parameters.
func NewServerRunOptions() *ServerRunOptions {
	s := ServerRunOptions{
		GenericServerRunOptions: genericoptions.NewServerRunOptions(),
		Etcd:                    genericoptions.NewEtcdOptions(storagebackend.NewDefaultConfig(defaultEtcdPathPrefix, nil)),
		SecureServing:           genericoptions.NewSecureServingOptions().WithLoopback(),
	}

	s.Etcd.StorageConfig.EncodeVersioner = runtime.NewMultiGroupVersioner(v1alpha1.SchemeGroupVersion, schema.GroupKind{Group: v1alpha1.GroupName})

	return &s
}

// completedServerRunOptions is a private wrapper that enforces a call of Complete() before Run can be invoked.
type completedServerRunOptions struct {
	*ServerRunOptions
}

// Complete set default ServerRunOptions.
// Should be called after badidea flags parsed.
func Complete(s *ServerRunOptions) (completedServerRunOptions, error) {
	s.SecureServing.BindPort = 6443
	s.Etcd.StorageConfig.Transport.ServerList = []string{"http://127.0.0.1:2379"}

	var options completedServerRunOptions
	// set defaults
	if err := s.GenericServerRunOptions.DefaultAdvertiseAddress(s.SecureServing.SecureServingOptions); err != nil {
		return options, err
	}

	if err := s.SecureServing.MaybeDefaultWithSelfSignedCerts(s.GenericServerRunOptions.AdvertiseAddress.String(), nil, []net.IP{net.ParseIP("127.0.0.1")}); err != nil {
		return options, fmt.Errorf("error creating self-signed certificates: %w", err)
	}

	if len(s.GenericServerRunOptions.ExternalHost) == 0 {
		if len(s.GenericServerRunOptions.AdvertiseAddress) > 0 {
			s.GenericServerRunOptions.ExternalHost = s.GenericServerRunOptions.AdvertiseAddress.String()
		} else {
			if hostname, err := os.Hostname(); err == nil {
				s.GenericServerRunOptions.ExternalHost = hostname
			} else {
				return options, fmt.Errorf("error finding host name: %w", err)
			}
		}

		klog.Infof("external host was not specified, using %v", s.GenericServerRunOptions.ExternalHost)
	}

	if s.Etcd.EnableWatchCache {
		// Ensure that overrides parse correctly.
		if _, err := genericoptions.ParseWatchCacheSizes(s.Etcd.WatchCacheSizes); err != nil {
			return options, err
		}
	}

	options.ServerRunOptions = s

	return options, nil
}

// Validate checks ServerRunOptions and return a slice of found errs.
func (s *ServerRunOptions) Validate() []error {
	var errs []error
	errs = append(errs, s.Etcd.Validate()...)
	errs = append(errs, s.SecureServing.Validate()...)

	return errs
}

// CreateServerChain creates the apiservers connected via delegation.
func CreateServerChain(completedOptions completedServerRunOptions, stopCh <-chan struct{}) (*aggregatorapiserver.APIAggregator, error) {
	badIdeaAPIServerConfig, err := CreateBadIdeaAPIServerConfig(completedOptions)
	if err != nil {
		return nil, err
	}

	// // If additional API servers are added, they should be gated.
	// apiExtensionsConfig := createAPIExtensionsConfig(*badIdeaAPIServerConfig.GenericConfig, badIdeaAPIServerConfig.ExtraConfig.VersionedInformers, completedOptions.ServerRunOptions)

	// apiExtensionsServer, err := createAPIExtensionsServer(apiExtensionsConfig, genericapiserver.NewEmptyDelegate())
	// if err != nil {
	// 	klog.Fatal(err)
	// 	return nil, err
	// }

	badIdeaAPIServer, err := CreateBadIdeaAPIServer(badIdeaAPIServerConfig, genericapiserver.NewEmptyDelegate()) //, apiExtensionsServer.GenericAPIServer)
	if err != nil {
		return nil, err
	}

	// aggregator comes last in the chain
	aggregatorConfig := createAggregatorConfig(*badIdeaAPIServerConfig.GenericConfig, completedOptions.ServerRunOptions, badIdeaAPIServerConfig.ExtraConfig.VersionedInformers)

	aggregatorServer, err := createAggregatorServer(aggregatorConfig, badIdeaAPIServer.GenericAPIServer, nil) //, apiExtensionsServer.Informers)
	if err != nil {
		// we don't need special handling for innerStopCh because the aggregator server doesn't create any go routines
		return nil, err
	}

	return aggregatorServer, nil
}

// CreateBadIdeaAPIServerConfig creates all the resources for running the API server, but runs none of them.
func CreateBadIdeaAPIServerConfig(
	s completedServerRunOptions,
) (
	*BadIdeaServerConfig,
	error,
) {
	genericConfig, versionedInformers, storageFactory, err := buildGenericConfig(s.ServerRunOptions)
	if err != nil {
		return nil, err
	}

	if _, port, err := net.SplitHostPort(s.Etcd.StorageConfig.Transport.ServerList[0]); err == nil && port != "0" && len(port) != 0 {
		if err := wait.PollImmediate(etcdRetryInterval, etcdRetryLimit*etcdRetryInterval, preflight.EtcdConnection{ServerList: s.Etcd.StorageConfig.Transport.ServerList}.CheckEtcdServers); err != nil {
			return nil, fmt.Errorf("error waiting for etcd connection: %w", err)
		}
	}

	config := &BadIdeaServerConfig{
		GenericConfig: genericConfig,
		ExtraConfig: ExtraConfig{
			APIResourceConfigSource: storageFactory.APIResourceConfigSource,
			StorageFactory:          storageFactory,
			VersionedInformers:      versionedInformers,
		},
	}

	return config, nil
}

// BuildGenericConfig takes the master server options and produces the genericapiserver.Config associated with it.
func buildGenericConfig(
	s *ServerRunOptions,
) (
	genericConfig *genericapiserver.Config,
	versionedInformers informers.SharedInformerFactory,
	storageFactory *storage.DefaultStorageFactory,
	lastErr error,
) {
	genericConfig = genericapiserver.NewConfig(Codecs)

	var mergedConfig *storage.ResourceConfig

	mergedConfig, lastErr = resourceconfig.MergeAPIResourceConfigs(storage.NewResourceConfig(), map[string]string{}, aggregatorscheme.Scheme)
	if lastErr != nil {
		return
	}

	genericConfig.MergedResourceConfig = mergedConfig

	if lastErr = s.GenericServerRunOptions.ApplyTo(genericConfig); lastErr != nil {
		return
	}

	if lastErr = s.SecureServing.ApplyTo(&genericConfig.SecureServing, &genericConfig.LoopbackClientConfig); lastErr != nil {
		return
	}

	// genericConfig.Version = &version.Info{
	// 	Major: "0",
	// 	Minor: "1",
	// }

	genericConfig.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(openapi.GetOpenAPIDefinitions, openapinamer.NewDefinitionNamer(Scheme, extensionsapiserver.Scheme, aggregatorscheme.Scheme))
	genericConfig.OpenAPIConfig.Info.Title = "BadIdea"
	genericConfig.OpenAPIConfig.Info.Version = "0.1"
	genericConfig.LongRunningFunc = filters.BasicLongRunningRequestCheck(
		sets.NewString("watch"),
		sets.NewString(),
	)

	storageFactoryConfig := NewStorageFactoryConfig()
	storageFactoryConfig.APIResourceConfig = genericConfig.MergedResourceConfig

	completedStorageFactoryConfig, err := storageFactoryConfig.Complete(s.Etcd)
	if err != nil {
		lastErr = err
		return
	}

	storageFactory, lastErr = completedStorageFactoryConfig.New()
	if lastErr != nil {
		return
	}

	genericConfig.LoopbackClientConfig.DisableCompression = true

	kubeClientConfig := genericConfig.LoopbackClientConfig

	clientgoExternalClient, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		lastErr = fmt.Errorf("failed to create real external clientset: %w", err)

		return
	}

	versionedInformers = informers.NewSharedInformerFactory(clientgoExternalClient, 10*time.Minute)

	return
}

// CreateBadIdeaAPIServer creates and wires a workable badidea-apiserver.
func CreateBadIdeaAPIServer(badIdeaAPIServerConfig *BadIdeaServerConfig, delegateAPIServer genericapiserver.DelegationTarget) (*BadIdeaServer, error) {
	badIdeaAPIServer, err := badIdeaAPIServerConfig.Complete().New(delegateAPIServer)
	if err != nil {
		return nil, err
	}

	return badIdeaAPIServer, nil
}

// StorageFactoryConfig is a configuration for creating storage factory.
type StorageFactoryConfig struct {
	StorageConfig                    storagebackend.Config
	APIResourceConfig                *storage.ResourceConfig
	DefaultResourceEncoding          *storage.DefaultResourceEncodingConfig
	DefaultStorageMediaType          string
	Serializer                       runtime.StorageSerializer
	ResourceEncodingOverrides        []schema.GroupVersionResource
	EtcdServersOverrides             []string
	EncryptionProviderConfigFilepath string
}

// NewStorageFactoryConfig returns a new StorageFactoryConfig set up with necessary resource overrides.
func NewStorageFactoryConfig() *StorageFactoryConfig {
	resources := []schema.GroupVersionResource{}

	return &StorageFactoryConfig{
		Serializer:                Codecs,
		DefaultResourceEncoding:   storage.NewDefaultResourceEncodingConfig(Scheme),
		ResourceEncodingOverrides: resources,
	}
}

// Complete completes the StorageFactoryConfig with provided etcdOptions returning completedStorageFactoryConfig.
func (c *StorageFactoryConfig) Complete(etcdOptions *genericoptions.EtcdOptions) (*completedStorageFactoryConfig, error) {
	c.StorageConfig = etcdOptions.StorageConfig
	c.DefaultStorageMediaType = etcdOptions.DefaultStorageMediaType
	c.EtcdServersOverrides = etcdOptions.EtcdServersOverrides
	c.EncryptionProviderConfigFilepath = etcdOptions.EncryptionProviderConfigFilepath

	return &completedStorageFactoryConfig{c}, nil
}

// completedStorageFactoryConfig is a wrapper around StorageFactoryConfig completed with etcd options.
//
// Note: this struct is intentionally unexported so that it can only be constructed via a StorageFactoryConfig.Complete
// call. The implied consequence is that this does not comply with golint.
type completedStorageFactoryConfig struct {
	*StorageFactoryConfig
}

// New returns a new storage factory created from the completed storage factory configuration.
func (c *completedStorageFactoryConfig) New() (*storage.DefaultStorageFactory, error) {
	resourceEncodingConfig := resourceconfig.MergeResourceEncodingConfigs(c.DefaultResourceEncoding, c.ResourceEncodingOverrides)
	storageFactory := storage.NewDefaultStorageFactory(
		c.StorageConfig,
		c.DefaultStorageMediaType,
		c.Serializer,
		resourceEncodingConfig,
		c.APIResourceConfig,
		map[schema.GroupResource]string{})

	for _, override := range c.EtcdServersOverrides {
		tokens := strings.Split(override, "#")
		apiresource := strings.Split(tokens[0], "/")

		group := apiresource[0]
		resource := apiresource[1]
		groupResource := schema.GroupResource{Group: group, Resource: resource}

		servers := strings.Split(tokens[1], ";")
		storageFactory.SetEtcdLocation(groupResource, servers)
	}

	if len(c.EncryptionProviderConfigFilepath) != 0 {
		transformerOverrides, err := encryptionconfig.GetTransformerOverrides(c.EncryptionProviderConfigFilepath)
		if err != nil {
			return nil, err
		}

		for groupResource, transformer := range transformerOverrides {
			storageFactory.SetTransformer(groupResource, transformer)
		}
	}

	return storageFactory, nil
}
