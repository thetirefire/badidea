/*
Copyright 2018 The Kubernetes Authors.

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

package options

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"

	kubeauthenticator "github.com/thetirefire/badidea/pkg/kubeapiserver/authenticator"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/authenticatorfactory"
	"k8s.io/apiserver/pkg/authentication/request/headerrequest"
	apiserveroptions "k8s.io/apiserver/pkg/server/options"
)

func TestToAuthenticationConfig(t *testing.T) {
	testOptions := &BuiltInAuthenticationOptions{
		Anonymous: &AnonymousAuthenticationOptions{
			Allow: false,
		},
		RequestHeader: &apiserveroptions.RequestHeaderAuthenticationOptions{
			UsernameHeaders:     []string{"x-remote-user"},
			GroupHeaders:        []string{"x-remote-group"},
			ExtraHeaderPrefixes: []string{"x-remote-extra-"},
			ClientCAFile:        "testdata/root.pem",
			AllowedNames:        []string{"kube-aggregator"},
		},
	}

	expectConfig := kubeauthenticator.Config{
		APIAudiences: authenticator.Audiences{"http://foo.bar.com"},
		Anonymous:    false,

		RequestHeaderConfig: &authenticatorfactory.RequestHeaderConfig{
			UsernameHeaders:     headerrequest.StaticStringSlice{"x-remote-user"},
			GroupHeaders:        headerrequest.StaticStringSlice{"x-remote-group"},
			ExtraHeaderPrefixes: headerrequest.StaticStringSlice{"x-remote-extra-"},
			CAContentProvider:   nil, // this is nil because you can't compare functions
			AllowedClientNames:  headerrequest.StaticStringSlice{"kube-aggregator"},
		},
	}

	resultConfig, err := testOptions.ToAuthenticationConfig()
	if err != nil {
		t.Fatal(err)
	}

	// nil these out because you cannot compare pointers.  Ensure they are non-nil first
	if resultConfig.RequestHeaderConfig.CAContentProvider == nil {
		t.Error("missing requestheader verify")
	}
	resultConfig.RequestHeaderConfig.CAContentProvider = nil

	if !reflect.DeepEqual(resultConfig, expectConfig) {
		t.Error(cmp.Diff(resultConfig, expectConfig))
	}
}
