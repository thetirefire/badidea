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
	aggregatorapiserver "k8s.io/kube-aggregator/pkg/apiserver"
)

// CreateServerChain creates the chained aggregated server.
func CreateServerChain() (*aggregatorapiserver.APIAggregator, error) {
	genericConfig, genericEtcdOptions, extensionServer, err := CreateExtensions()
	if err != nil {
		return nil, err
	}

	config, err := CreateAggregatorConfig(genericConfig, genericEtcdOptions)
	if err != nil {
		return nil, err
	}

	return CreateAggregatorServer(config, extensionServer.GenericAPIServer, extensionServer.Informers)
}
