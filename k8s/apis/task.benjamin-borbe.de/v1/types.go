// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

package v1

import (
	lib "github.com/bborbe/agent/lib"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Schedule is the Schema for the Schedule CRD. Names are frozen for the
// life of v1 (spec 008). No status subresource in v1 — Status field
// exists on the Go type for future Spec B controller writes but the CRD
// schema does not register `/status`.
type Schedule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ScheduleSpec   `json:"spec,omitempty"`
	Status ScheduleStatus `json:"status,omitempty"`
}

// ScheduleSpec is the spec of a Schedule.
type ScheduleSpec struct {
	// Vault is the Obsidian vault slug the generated task lives in.
	// Constrained to ^[a-z][a-z0-9-]*$ by the OpenAPI schema (enforced at
	// the API-server boundary via SetupCustomResourceDefinition in
	// pkg/k8s_connector.go).
	Vault string `json:"vault"`

	// Title is the title shown to the user.
	Title string `json:"title"`

	// Schedule describes when the task fires. The weekday-required-iff-weekday
	// invariant is encoded as a CEL x-kubernetes-validations rule in
	// scheduleSpecSchema.
	Schedule ScheduleTrigger `json:"schedule"`

	// Template is the body + frontmatter stamped onto the generated task.
	Template ScheduleTemplate `json:"template"`
}

// ScheduleTrigger is the recurrence subnode.
type ScheduleTrigger struct {
	// Recurrence is one of: "Daily", "Weekly", "Weekday", "Monthly", "Quarterly", "Yearly"
	// (capitalized, matching Go's time.Weekday.String() style and Spec 6/9's
	// period-token output). "Weekly" is always-fire (no weekday); "Weekday"
	// fires only on its target weekday. Constrained by the OpenAPI enum in
	// scheduleSpecSchema.
	Recurrence string `json:"recurrence"`

	// Weekday is required when Recurrence == "Weekday"; forbidden otherwise.
	// Values are time.Weekday.String() form: "Monday", "Tuesday", "Wednesday",
	// "Thursday", "Friday", "Saturday", "Sunday". Encoded as the CEL rule in
	// scheduleSpecSchema. The Go type is `string` (not `*string`) so JSON
	// omits the field cleanly when unset; the schema's presence check is
	// `has(self.weekday)`. Optionality is encoded by `omitempty` + the CEL
	// rule — no separate `+optional` marker is needed.
	Weekday string `json:"weekday,omitempty"`
}

// ScheduleTemplate is the body + frontmatter the generated task carries.
type ScheduleTemplate struct {
	// Body is raw markdown. Free-form; not validated.
	Body string `json:"body,omitempty"`

	// Frontmatter is the YAML frontmatter of the generated vault file.
	// Reuses lib.TaskFrontmatter from github.com/bborbe/agent/lib.
	Frontmatter lib.TaskFrontmatter `json:"frontmatter,omitempty"`
}

// ScheduleStatus describes the observed state of a Schedule.
type ScheduleStatus struct {
	// LastTickedAt is the wall-clock time of the most recent successful tick
	// for this Schedule. +optional.
	// +optional
	LastTickedAt metav1.Time `json:"lastTickedAt,omitempty"`

	// LastPublishedTaskIdentifier is the deterministic UUID5 of the most
	// recently published task for this Schedule. +optional.
	// +optional
	LastPublishedTaskIdentifier string `json:"lastPublishedTaskIdentifier,omitempty"`
}

// ScheduleList is a list of Schedules.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ScheduleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Schedule `json:"items"`
}

// Placeholders is the documented placeholder set the rendered task
// template can reference. Rendered by the publisher (spec 002) using
// the same strings. Closed set — adding a new placeholder is a new spec.
var Placeholders = []string{
	"Date",      // Renders YYYY-MM-DD.
	"ISOWeek",   // Renders YYYYWNN (uppercase W, two-digit week).
	"MonthYear", // Renders YYYY-MM.
	"Quarter",   // Renders YYYYQNN (uppercase Q, two-digit quarter).
	"Year",      // Renders YYYY.
}
