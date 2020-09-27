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

package cmd

import (
	"github.com/spf13/cobra"
	"github.com/thetirefire/badidea/server"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/component-base/logs"
	"k8s.io/klog"
)

func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "badidea",
		Short:   "badidea",
		Version: "0.1",
		RunE: func(cmd *cobra.Command, args []string) error {
			logs.InitLogs()

			// if _, err := logs.GlogSetter("8"); err != nil {
			// 	klog.Fatal(err)
			// }

			defer logs.FlushLogs()

			stopCh := genericapiserver.SetupSignalHandler()

			if err := server.RunBadIdeaServer(stopCh); err != nil {
				klog.Fatal(err)
			}

			return nil
		},
	}

	return rootCmd
}
