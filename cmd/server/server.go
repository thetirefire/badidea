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

package server

import (
	"fmt"
	"io"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"
	openapinamer "k8s.io/apiserver/pkg/endpoints/openapi"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/filters"
	clientgoinformers "k8s.io/client-go/informers"
	clientgofake "k8s.io/client-go/kubernetes/fake"
	aggregatorapiserver "k8s.io/kube-aggregator/pkg/apiserver"
	aggregatorscheme "k8s.io/kube-aggregator/pkg/apiserver/scheme"
	aggregatorserver "k8s.io/kube-aggregator/pkg/cmd/server"
	aggregatoropenapi "k8s.io/kube-aggregator/pkg/generated/openapi"
)

// BadIdeaServerOptions contains state for master/api server.
type BadIdeaServerOptions struct {
	// SharedInformerFactory informers.SharedInformerFactory
	StdOut io.Writer
	StdErr io.Writer
}

// RunBadIdeaServer starts a new BadIdeaServer given BadIdeaServerOptions.
func (o BadIdeaServerOptions) RunBadIdeaServer(stopCh <-chan struct{}) error {
	// TODO: Add apiextensions server
	// TODO: Add badidea server
	return RunAggregator(stopCh)
}

// RunAggregator runs the API Aggregator.
func RunAggregator(stopCh <-chan struct{}) error {
	o := aggregatorserver.NewDefaultOptions(os.Stdout, os.Stderr)

	o.RecommendedOptions.Etcd.StorageConfig.Transport.ServerList = []string{"http://127.0.0.1:2379"}
	o.RecommendedOptions.SecureServing.BindPort = 6443
	o.RecommendedOptions.Authentication.RemoteKubeConfigFileOptional = true
	o.RecommendedOptions.Authorization.RemoteKubeConfigFileOptional = true
	o.RecommendedOptions.Authorization.AlwaysAllowPaths = []string{"*"}
	o.RecommendedOptions.Authorization.AlwaysAllowGroups = []string{"*"}
	o.RecommendedOptions.CoreAPI = nil
	o.RecommendedOptions.Admission = nil

	if err := o.Validate(nil); err != nil {
		return err
	}

	if err := o.Complete(); err != nil {
		return err
	}

	// TODO have a "real" external address
	if err := o.RecommendedOptions.SecureServing.MaybeDefaultWithSelfSignedCerts("localhost", nil, nil); err != nil {
		return fmt.Errorf("error creating self-signed certificates: %w", err)
	}

	serverConfig := genericapiserver.NewRecommendedConfig(aggregatorscheme.Codecs)

	if err := o.ServerRunOptions.ApplyTo(&serverConfig.Config); err != nil {
		return err
	}

	if err := o.RecommendedOptions.ApplyTo(serverConfig); err != nil {
		return err
	}

	if err := o.APIEnablement.ApplyTo(&serverConfig.Config, aggregatorapiserver.DefaultAPIResourceConfigSource(), aggregatorscheme.Scheme); err != nil {
		return err
	}

	// TODO: fake it until we make it
	serverConfig.SharedInformerFactory = clientgoinformers.NewSharedInformerFactory(clientgofake.NewSimpleClientset(), 10*time.Minute)

	serverConfig.LongRunningFunc = filters.BasicLongRunningRequestCheck(
		sets.NewString("watch"),
		sets.NewString(),
	)
	serverConfig.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(aggregatoropenapi.GetOpenAPIDefinitions, openapinamer.NewDefinitionNamer(aggregatorscheme.Scheme))
	serverConfig.OpenAPIConfig.Info.Title = "kube-aggregator"

	serviceResolver := aggregatorapiserver.NewClusterIPServiceResolver(serverConfig.SharedInformerFactory.Core().V1().Services().Lister())

	config := aggregatorapiserver.Config{
		GenericConfig: serverConfig,
		ExtraConfig: aggregatorapiserver.ExtraConfig{
			ServiceResolver: serviceResolver,
		},
	}

	server, err := config.Complete().NewWithDelegate(genericapiserver.NewEmptyDelegate())
	if err != nil {
		return err
	}

	prepared, err := server.PrepareRun()
	if err != nil {
		return err
	}

	return prepared.Run(stopCh)
}
