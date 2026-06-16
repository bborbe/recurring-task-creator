// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +kubebuilder:object:generate=true
// +groupName=task.benjamin-borbe.de

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GroupName is the API group for the Schedule CRD. Frozen.
const GroupName = "task.benjamin-borbe.de"

// Version is the API version (v1). Frozen.
const Version = "v1"

// SchemeGroupVersion is the group/version pair used by the typed clientset.
var SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: Version}

// Resource takes an unqualified resource name and returns a GroupResource.
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

var (
	// SchemeBuilder collects the functions that add types to the scheme.
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)

	// AddToScheme applies the registered functions to a runtime.Scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

// Kind is the resource Kind. Frozen.
const Kind = "Schedule"

// ListKind is the resource ListKind. Frozen.
const ListKind = "ScheduleList"

// Plural is the resource plural name. Frozen.
const Plural = "schedules"

// Singular is the resource singular name. Frozen.
const Singular = "schedule"

// ShortNames are short names for the resource. Frozen.
var ShortNames = []string{"ts"}

// addKnownTypes registers the Schedule + ScheduleList types with the scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&Schedule{},
		&ScheduleList{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
