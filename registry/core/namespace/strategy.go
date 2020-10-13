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

package namespace

import (
	"context"
	"fmt"

	"github.com/thetirefire/badidea/apis/core"
	"github.com/thetirefire/badidea/apis/core/validation"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/names"
)

// NewStrategy creates and returns a namespaceStrategy instance.
func NewStrategy(typer runtime.ObjectTyper) namespaceStrategy {
	return namespaceStrategy{typer, names.SimpleNameGenerator}
}

// namespaceStrategy implements behavior for Namespaces.
type namespaceStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// NamespaceScoped is false for namespaces.
func (namespaceStrategy) NamespaceScoped() bool {
	return false
}

// AllowCreateOnUpdate is false for namespaces.
func (namespaceStrategy) AllowCreateOnUpdate() bool {
	return false
}

// AllowCreateOnUpdate is true for namespaces.
func (namespaceStrategy) AllowUnconditionalUpdate() bool {
	return true
}

// Canonicalize normalizes the object after validation.
func (namespaceStrategy) Canonicalize(obj runtime.Object) {
}

// PrepareForCreate clears fields that are not allowed to be set by end users on creation.
func (namespaceStrategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	// on create, status is active
	namespace := obj.(*core.Namespace)
	namespace.Status = core.NamespaceStatus{
		Phase: core.NamespaceActive,
	}

	// on create, we require the kubernetes value
	// we cannot use this in defaults conversion because we let it get removed over life of object
	hasKubeFinalizer := false

	for i := range namespace.Spec.Finalizers {
		if namespace.Spec.Finalizers[i] == core.FinalizerKubernetes {
			hasKubeFinalizer = true

			break
		}
	}

	if !hasKubeFinalizer {
		if len(namespace.Spec.Finalizers) == 0 {
			namespace.Spec.Finalizers = []core.FinalizerName{core.FinalizerKubernetes}
		} else {
			namespace.Spec.Finalizers = append(namespace.Spec.Finalizers, core.FinalizerKubernetes)
		}
	}
}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update.
func (namespaceStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newNamespace := obj.(*core.Namespace)
	oldNamespace := old.(*core.Namespace)
	newNamespace.Spec.Finalizers = oldNamespace.Spec.Finalizers
	newNamespace.Status = oldNamespace.Status
}

// Validate validates a new namespace.
func (namespaceStrategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	namespace := obj.(*core.Namespace)

	return validation.ValidateNamespace(namespace)
}

// ValidateUpdate is the default update validation for an end user.
func (namespaceStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	errorList := validation.ValidateNamespace(obj.(*core.Namespace))

	return append(errorList, validation.ValidateNamespaceUpdate(obj.(*core.Namespace), old.(*core.Namespace))...)
}

// GetAttrs returns labels.Set, fields.Set, and error in case the given runtime.Object is not a Namespace.
func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	apiserver, ok := obj.(*core.Namespace)
	if !ok {
		return nil, nil, fmt.Errorf("given object is not a Namespace")
	}

	return labels.Set(apiserver.ObjectMeta.Labels), SelectableFields(apiserver), nil
}

// MatchNamespace is the filter used by the generic etcd backend to watch events
// from etcd to clients of the apiserver only interested in specific labels/fields.
func MatchNamespace(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{
		Label:    label,
		Field:    field,
		GetAttrs: GetAttrs,
	}
}

// SelectableFields returns a field set that represents the object.
func SelectableFields(obj *core.Namespace) fields.Set {
	objectMetaFieldsSet := generic.ObjectMetaFieldsSet(&obj.ObjectMeta, false)
	specificFieldsSet := fields.Set{
		"status.phase": string(obj.Status.Phase),
		// This is a bug, but we need to support it for backward compatibility.
		"name": obj.Name,
	}

	return generic.MergeFieldsSets(objectMetaFieldsSet, specificFieldsSet)
}

type namespaceStatusStrategy struct {
	namespaceStrategy
}

// NewStatusStrategy creates and returns a namespaceStrategy instance.
func NewStatusStrategy(typer runtime.ObjectTyper) namespaceStatusStrategy {
	return namespaceStatusStrategy{NewStrategy(typer)}
}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update.
func (namespaceStatusStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newNamespace := obj.(*core.Namespace)
	oldNamespace := old.(*core.Namespace)
	newNamespace.Spec = oldNamespace.Spec
}

// ValidateUpdate is the default update validation for an end user.
func (namespaceStatusStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateNamespaceStatusUpdate(obj.(*core.Namespace), old.(*core.Namespace))
}

type namespaceFinalizeStrategy struct {
	namespaceStrategy
}

// NewFinalizeStrategy creates and returns a namespaceStrategy instance.
func NewFinalzeStrategy(typer runtime.ObjectTyper) namespaceFinalizeStrategy {
	return namespaceFinalizeStrategy{NewStrategy(typer)}
}

// ValidateUpdate is the default update validation for an end user.
func (namespaceFinalizeStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateNamespaceFinalizeUpdate(obj.(*core.Namespace), old.(*core.Namespace))
}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update.
func (namespaceFinalizeStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newNamespace := obj.(*core.Namespace)
	oldNamespace := old.(*core.Namespace)
	newNamespace.Status = oldNamespace.Status
}
