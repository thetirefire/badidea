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
	"net"

	"github.com/spf13/cobra"
	"github.com/thetirefire/badidea/apis/badidea/v1alpha1"
	"github.com/thetirefire/badidea/apiserver"
	badideaopenapi "github.com/thetirefire/badidea/pkg/generated/openapi"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apiserver/pkg/endpoints/openapi"
	"k8s.io/apiserver/pkg/features"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
)

const defaultEtcdPathPrefix = "/registry/badidea.x-k8s.io"

// BadIdeaServerOptions contains state for master/api server.
type BadIdeaServerOptions struct {
	RecommendedOptions *genericoptions.RecommendedOptions

	// SharedInformerFactory informers.SharedInformerFactory
	StdOut io.Writer
	StdErr io.Writer
}

// NewBadIdeaServerOptions returns a new BadIdeaServerOptions.
func NewBadIdeaServerOptions(out, errOut io.Writer) *BadIdeaServerOptions {
	o := &BadIdeaServerOptions{
		RecommendedOptions: genericoptions.NewRecommendedOptions(
			defaultEtcdPathPrefix,
			apiserver.Codecs.LegacyCodec(v1alpha1.SchemeGroupVersion),
		),

		StdOut: out,
		StdErr: errOut,
	}
	o.RecommendedOptions.Etcd.StorageConfig.EncodeVersioner = runtime.NewMultiGroupVersioner(v1alpha1.SchemeGroupVersion, schema.GroupKind{Group: v1alpha1.GroupName})

	return o
}

// NewCommandStartBadIdeaServer provides a CLI handler for 'start master' command
// with a default BadIdeaServerOptions.
func NewCommandStartBadIdeaServer(defaults *BadIdeaServerOptions, stopCh <-chan struct{}) *cobra.Command {
	o := *defaults
	cmd := &cobra.Command{
		Short: "Launch a badidea API server",
		Long:  "Launch a badidea API server",
		RunE: func(c *cobra.Command, args []string) error {
			if err := o.Complete(); err != nil {
				return err
			}
			if err := o.Validate(args); err != nil {
				return err
			}
			if err := o.RunBadIdeaServer(stopCh); err != nil {
				return err
			}

			return nil
		},
	}

	flags := cmd.Flags()
	o.RecommendedOptions.AddFlags(flags)
	utilfeature.DefaultMutableFeatureGate.AddFlag(flags)

	return cmd
}

// Validate validates BadIdeaServerOptions.
func (o BadIdeaServerOptions) Validate(args []string) error {
	errors := []error{}
	errors = append(errors, o.RecommendedOptions.Validate()...)

	return utilerrors.NewAggregate(errors)
}

// Complete fills in fields required to have valid data.
func (o *BadIdeaServerOptions) Complete() error {
	return nil
}

// Config returns config for the api server given BadIdeaServerOptions.
func (o *BadIdeaServerOptions) Config() (*apiserver.Config, error) {
	// TODO have a "real" external address
	if err := o.RecommendedOptions.SecureServing.MaybeDefaultWithSelfSignedCerts("localhost", nil, []net.IP{net.ParseIP("127.0.0.1")}); err != nil {
		return nil, fmt.Errorf("error creating self-signed certificates: %w", err)
	}

	o.RecommendedOptions.Etcd.StorageConfig.Paging = utilfeature.DefaultFeatureGate.Enabled(features.APIListChunking)

	serverConfig := genericapiserver.NewRecommendedConfig(apiserver.Codecs)

	serverConfig.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(badideaopenapi.GetOpenAPIDefinitions, openapi.NewDefinitionNamer(apiserver.Scheme))
	serverConfig.OpenAPIConfig.Info.Title = "BadIdea"
	serverConfig.OpenAPIConfig.Info.Version = "0.1"

	if err := o.RecommendedOptions.ApplyTo(serverConfig); err != nil {
		return nil, err
	}

	config := &apiserver.Config{
		GenericConfig: serverConfig,
		ExtraConfig:   apiserver.ExtraConfig{},
	}

	return config, nil
}

// RunBadIdeaServer starts a new BadIdeaServer given BadIdeaServerOptions.
func (o BadIdeaServerOptions) RunBadIdeaServer(stopCh <-chan struct{}) error {
	config, err := o.Config()
	if err != nil {
		return err
	}

	server, err := config.Complete().New()
	if err != nil {
		return err
	}

	// server.GenericAPIServer.AddPostStartHookOrDie("start-badidea-informers", func(context genericapiserver.PostStartHookContext) error {
	// 	config.GenericConfig.SharedInformerFactory.Start(context.StopCh)
	// 	o.SharedInformerFactory.Start(context.StopCh)

	// 	return nil
	// })

	return server.GenericAPIServer.PrepareRun().Run(stopCh)
}
