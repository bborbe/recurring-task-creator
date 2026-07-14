// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

package v1

import (
	lib "github.com/bborbe/agent"
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

	// Weekday is a single weekday (long form Monday..Sunday). Set when
	// Recurrence == "Weekday" and the schedule targets exactly one day;
	// mutually exclusive with Weekdays (the CEL XOR rule in
	// scheduleTriggerSchema enforces exactly-one-of on Weekday recurrence,
	// neither otherwise). Normalized to a canonical time.Weekday Go-side by
	// the store adapter.
	Weekday string `json:"weekday,omitempty"`

	// Weekdays is a non-empty list of weekdays (long form Monday..Sunday or
	// short form Mon..Sun, mixable). Set when Recurrence == "Weekday" and the
	// schedule targets multiple days; mutually exclusive with Weekday.
	// Normalized and deduplicated to a canonical time.Weekday set Go-side by
	// the store adapter.
	Weekdays []string `json:"weekdays,omitempty"`

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

	// Month is the calendar month (1-12) an OnDate schedule fires in. Set
	// when Recurrence == "OnDate"; forbidden on every other kind (the CEL
	// rule in scheduleTriggerSchema enforces month+day required-iff-OnDate).
	// Mapped to schedule.TaskDefinition.Month (as time.Month) by the store
	// adapter.
	Month int `json:"month,omitempty"`

	// Day is the day-of-month (1-31) an OnDate schedule fires on. Set when
	// Recurrence == "OnDate"; forbidden on every other kind. A static 1-31
	// range only — 02-30 is admitted but never fires. Mapped to
	// schedule.TaskDefinition.Day by the store adapter.
	Day int `json:"day,omitempty"`

	// AutoAbortPrior is an opt-in flag (default false when unset) marking
	// this Schedule as one whose prior-period instance MAY be auto-aborted
	// by the downstream task-controller when the next instance materializes.
	// A pointer so an unset field (nil → effective false) is distinguishable
	// from an explicit false. The publisher resolves the pointer to a plain
	// bool and stamps it as the `auto_abort_prior` frontmatter key on every
	// materialized task; the controller reads that key as its eligibility
	// gate (controller-side gate flip ships in a separate PR). Optional —
	// never required by the CRD schema.
	AutoAbortPrior *bool `json:"autoAbortPrior,omitempty"`
}

// ScheduleTemplate is the body + frontmatter the generated task carries.
type ScheduleTemplate struct {
	// Body is raw markdown. Free-form; not validated.
	Body string `json:"body,omitempty"`

	// Frontmatter is the YAML frontmatter of the generated vault file.
	// Reuses lib.TaskFrontmatter from github.com/bborbe/agent.
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
