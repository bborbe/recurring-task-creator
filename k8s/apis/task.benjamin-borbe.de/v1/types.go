// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

package v1

import (
	"bytes"
	"encoding/json"

	lib "github.com/bborbe/agent/lib"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// WeekdayList is the spec.schedule.weekday value. The CRD wire shape is
// a single day string OR a non-empty list of day strings (long form
// Monday..Sunday or short form Mon..Sun, freely mixed) — see the OpenAPI
// OneOf in pkg/k8s_connector_schema.go. WeekdayList normalizes both wire
// shapes to a []string on decode: a bare string decodes to a one-element
// slice. Canonicalization of short->long form and the mapping to
// time.Weekday happens Go-side in the store adapter, not here — this type
// only unifies the two JSON shapes.
type WeekdayList []string

// UnmarshalJSON accepts either a JSON string ("Monday") or a JSON array
// (["Mon","Wed"]) and stores the result as a []string. A null or absent
// value yields a nil slice.
func (w *WeekdayList) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		*w = nil
		return nil
	}
	if trimmed[0] == '[' {
		var list []string
		if err := json.Unmarshal(trimmed, &list); err != nil {
			return err
		}
		*w = list
		return nil
	}
	var single string
	if err := json.Unmarshal(trimmed, &single); err != nil {
		return err
	}
	*w = []string{single}
	return nil
}

// MarshalJSON renders a one-element list back as a bare string (so an
// applied single-day CR round-trips to the same wire form) and a
// multi-element list as a JSON array. An empty list renders as null.
func (w WeekdayList) MarshalJSON() ([]byte, error) {
	switch len(w) {
	case 0:
		return []byte("null"), nil
	case 1:
		return json.Marshal(w[0])
	default:
		return json.Marshal([]string(w))
	}
}

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
	// Wire shape is a single day string OR a non-empty list (long form
	// Monday..Sunday or short form Mon..Sun, mixable). Enforced by the
	// OpenAPI OneOf + CEL rules in scheduleSpecSchema. Normalized to a
	// canonical time.Weekday set Go-side by the store adapter.
	Weekday WeekdayList `json:"weekday,omitempty"`

	// PeriodOffset shifts the period-anchored token by N periods. Default 0
	// (current period). Negative values name a prior period; positive values
	// name a future period. The shift applies to the period token suffix
	// appended to the task title AND the UUID5 input — so a Monthly schedule
	// firing on 2026-07-01 with PeriodOffset=-1 produces token "2026-06" and
	// the task is named "<title> - 2026-06". Body placeholders ({{current_month}}
	// etc.) render against the unshifted fire date — this is intentional.
	//
	// Only valid for Recurrence in {Monthly, Quarterly, Yearly}. Non-zero
	// values for Daily/Weekly/Weekday are rejected by the CEL rule in
	// scheduleSpecSchema (semantics undefined; date-anchored kinds don't
	// have a meaningful period offset distinct from a date shift).
	PeriodOffset int `json:"periodOffset,omitempty"`
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
