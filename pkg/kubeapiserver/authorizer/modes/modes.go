/*
Copyright 2017 The Kubernetes Authors.

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

package modes

import "k8s.io/apimachinery/pkg/util/sets"

const (
	// ModeAlwaysAllow is the mode to set all requests as authorized
	ModeAlwaysAllow string = "AlwaysAllow"
	// ModeAlwaysDeny is the mode to set no requests as authorized
	ModeAlwaysDeny string = "AlwaysDeny"
)

// AuthorizationModeChoices is the list of supported authorization modes
var AuthorizationModeChoices = []string{ModeAlwaysAllow, ModeAlwaysDeny}

// IsValidAuthorizationMode returns true if the given authorization mode is a valid one for the apiserver
func IsValidAuthorizationMode(authzMode string) bool {
	return sets.NewString(AuthorizationModeChoices...).Has(authzMode)
}
