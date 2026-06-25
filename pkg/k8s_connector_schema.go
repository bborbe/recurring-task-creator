// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pkg

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// recurrenceEnum is the closed set of valid recurrence strings on the
// CRD wire. Capitalized to match Go's time.Weekday.String() casing and
// Spec 6/Spec 9's period-token output. Mirrors the post-Spec-9 6-kind
// Go-level RecurrenceKind set (pkg/schedule/recurrence.go): Weekly is
// always-fire ("YYYYWww") while Weekday targets a specific weekday
// ("YYYYWww-<3-letter-abbrev>"). Lives in this package so the schema
// is self-contained; do NOT import pkg/schedule.RecurrenceKind (those
// constants are lowercase Go-internal values; the CRD enum is a
// separate API contract).
var recurrenceEnum = []string{"Daily", "Weekly", "Weekday", "Monthly", "Quarterly", "Yearly"}

// weekdayLongEnum is the closed set of valid strings for the single
// `weekday` field — 7 long forms only (Monday..Sunday, matching
// time.Weekday.String()). This is the pre-Spec-012 enum, restored so a
// single-day CR keeps its exact backward-compatible shape. Short forms
// are NOT accepted on this field; multi-day or short-form usage moves to
// the `weekdays` list field below.
var weekdayLongEnum = []string{
	"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday",
}

// weekdayAllEnum is the closed set of valid item strings for the new
// `weekdays` list field — both long forms (Monday..Sunday) and short
// forms (Mon..Sun), 14 strings total, freely mixable in one list. Short
// forms are normalized to long form Go-side at parse time (Prompt 2).
// Locked in v1 — typos like "Satuday" or "FunDay" are rejected at the
// API-server boundary by the item enum.
var weekdayAllEnum = []string{
	"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday",
	"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun",
}

// vaultPattern is the regex the API server enforces on spec.vault.
// Matches the slug convention used in pkg/schedule/inventory.go.
const vaultPattern = "^[a-z][a-z0-9-]*$"

// weekdayXorRule is the CEL rule encoded in schedule.XValidations. self
// is bound to the ScheduleTrigger object. On 'Weekday' recurrence,
// exactly one of weekday/weekdays must be set (XOR via inequality of the
// two has() booleans). On every other recurrence kind, neither may be
// set. This replaces the pre-Spec-012 single-field presence rule with a
// two-field equivalent of the same intent.
const weekdayXorRule = "self.recurrence == 'Weekday' ? " +
	"(has(self.weekday) != has(self.weekdays)) : " +
	"(!has(self.weekday) && !has(self.weekdays))"

// weekdayXorMessage is the human-readable error the API server emits
// when the rule fails. Surfaced to the operator via kubectl apply output.
const weekdayXorMessage = "exactly one of weekday or weekdays is required when recurrence is 'Weekday', and both are forbidden otherwise"

// periodOffsetOnlyForPeriodKindsRule rejects non-zero periodOffset on
// date-anchored recurrence kinds (Daily/Weekly/Weekday). Those kinds
// don't carry a period concept distinct from the fire date; a date
// shift is the user-visible knob there, not an offset. Only Monthly,
// Quarterly, Yearly accept a non-zero offset.
const periodOffsetOnlyForPeriodKindsRule = "!has(self.periodOffset) || self.periodOffset == 0 || self.recurrence in ['Monthly', 'Quarterly', 'Yearly']"

// periodOffsetOnlyForPeriodKindsMessage is the operator-facing error
// for the rule above.
const periodOffsetOnlyForPeriodKindsMessage = "periodOffset is only allowed for Monthly/Quarterly/Yearly recurrence"

// weekdayNoDuplicateRule rejects a weekdays list that names the same
// logical day twice, including cross-form duplicates ([Mon, Monday]).
// Each entry is canonicalized to its long form via a literal map, then
// the rule asserts every canonical value appears exactly once: for each
// canonical day c, the count of list entries that canonicalize to c must
// be 1. Bounded by the weekdays MaxItems:7 cap, this form's estimated
// cost stays under the API server's per-rule CEL cost budget.
// (The prior map().all().exists_one() form blew the budget because, with
// no maxItems, the cost estimator assumed n up to int32 max.) The bare
// cel.NewEnv the tests build does not load the Kubernetes Lists library,
// so index-range forms like 0.until(size(...)) do NOT compile here; this
// map().all().filter().size() form does. Only applies when weekdays is
// present.
const weekdayNoDuplicateRule = "!has(self.weekdays) || type(self.weekdays) != list || " +
	"self.weekdays.map(d, " +
	"{'Mon':'Monday','Tue':'Tuesday','Wed':'Wednesday','Thu':'Thursday'," +
	"'Fri':'Friday','Sat':'Saturday','Sun':'Sunday'," +
	"'Monday':'Monday','Tuesday':'Tuesday','Wednesday':'Wednesday'," +
	"'Thursday':'Thursday','Friday':'Friday','Saturday':'Saturday'," +
	"'Sunday':'Sunday'}[d])" +
	".all(c, self.weekdays.map(d2, " +
	"{'Mon':'Monday','Tue':'Tuesday','Wed':'Wednesday','Thu':'Thursday'," +
	"'Fri':'Friday','Sat':'Saturday','Sun':'Sunday'," +
	"'Monday':'Monday','Tuesday':'Tuesday','Wednesday':'Wednesday'," +
	"'Thursday':'Thursday','Friday':'Friday','Saturday':'Saturday'," +
	"'Sunday':'Sunday'}[d2]).filter(x, x == c).size() == 1)"

// weekdayNoDuplicateMessage is the operator-facing error when a weekday
// list contains the same logical day more than once.
const weekdayNoDuplicateMessage = "weekday list must not contain the same day twice (including cross-form duplicates like [Mon, Monday])"

// scheduleCRSchemaPtr returns the OpenAPI v3 schema for the WHOLE Schedule
// custom resource (the top-level object with apiVersion/kind/metadata/spec).
// This is what gets registered as the CRD's OpenAPIV3Schema — registering
// scheduleSpecSchema directly would reject every CR because `spec`,
// `apiVersion`, `kind`, and `metadata` would all be unknown top-level fields.
func scheduleCRSchemaPtr() *apiextensionsv1.JSONSchemaProps {
	return &apiextensionsv1.JSONSchemaProps{
		Type:        "object",
		Description: "Schedule recurring-task CR.",
		Required:    []string{"spec"},
		Properties: map[string]apiextensionsv1.JSONSchemaProps{
			"apiVersion": {Type: "string"},
			"kind":       {Type: "string"},
			"metadata":   {Type: "object"},
			"spec":       scheduleSpecSchema(),
		},
	}
}

// scheduleTriggerSchema returns the OpenAPI v3 schema for spec.schedule.
// Extracted from scheduleSpecSchema to satisfy funlen. All CEL rules on the
// schedule object live here alongside the field definitions they guard.
func scheduleTriggerSchema() apiextensionsv1.JSONSchemaProps {
	return apiextensionsv1.JSONSchemaProps{
		Type:        "object",
		Description: "Recurrence trigger. The weekday-required-iff-weekday invariant is enforced by the CEL x-kubernetes-validations rule below.",
		Required:    []string{"recurrence"},
		Properties: map[string]apiextensionsv1.JSONSchemaProps{
			"recurrence": {
				Type:        "string",
				Description: "One of: Daily, Weekly, Weekday, Monthly, Quarterly, Yearly.",
				Enum:        jsonEnumValues(recurrenceEnum),
			},
			"weekday": {
				Type:        "string",
				Description: "A single weekday (long form Monday..Sunday). Required-XOR with weekdays when recurrence is 'Weekday'; both fields forbidden otherwise. Normalized to a canonical time.Weekday Go-side at parse time.",
				Enum:        jsonEnumValues(weekdayLongEnum),
			},
			"weekdays": {
				Type:        "array",
				Description: "A non-empty list of weekdays, at most 7 (the number of distinct days in a week). Each entry is one of the 14 accepted day strings (long form Monday..Sunday or short form Mon..Sun); the two forms may be mixed in one list. Required-XOR with weekday when recurrence is 'Weekday'; both fields forbidden otherwise. Normalized to canonical time.Weekday values Go-side at parse time.",
				MinItems:    ptrInt64(1),
				MaxItems:    ptrInt64(7),
				Items: &apiextensionsv1.JSONSchemaPropsOrArray{
					Schema: &apiextensionsv1.JSONSchemaProps{
						Type:      "string",
						MaxLength: ptrInt64(9),
						Enum:      jsonEnumValues(weekdayAllEnum),
					},
				},
			},
			"periodOffset": {
				Type: "integer",
				Description: "Shifts the period-anchored token by N periods. Default 0 (current period). " +
					"Use -1 for prior period (e.g. review-style schedules that fire on month-start but name " +
					"the just-completed month). Only valid for Monthly/Quarterly/Yearly.",
			},
		},
		XValidations: apiextensionsv1.ValidationRules{
			{Rule: weekdayXorRule, Message: weekdayXorMessage},
			{
				Rule:    periodOffsetOnlyForPeriodKindsRule,
				Message: periodOffsetOnlyForPeriodKindsMessage,
			},
			{Rule: weekdayNoDuplicateRule, Message: weekdayNoDuplicateMessage},
		},
	}
}

// scheduleSpecSchema returns the OpenAPI v3 schema for spec.*. The
// schema is built as Go code (no CRD YAML manifest is generated or
// committed) and applied on every binary boot via
// SetupCustomResourceDefinition. Single source of truth: this file.
func scheduleSpecSchema() apiextensionsv1.JSONSchemaProps {
	return apiextensionsv1.JSONSchemaProps{
		Type:        "object",
		Description: "Schedule spec — defines when a recurring task fires and what to publish.",
		Required:    []string{"vault", "title", "schedule", "template"},
		Properties: map[string]apiextensionsv1.JSONSchemaProps{
			"vault": {
				Type:        "string",
				Description: "Obsidian vault slug. Must match ^[a-z][a-z0-9-]*$.",
				Pattern:     vaultPattern,
			},
			"title": {
				Type: "string",
				Description: "Title shown to the user in the generated vault task. " +
					"Go text/template — placeholders rendered with the period token.",
			},
			"schedule": scheduleTriggerSchema(),
			"template": {
				Type:        "object",
				Description: "Body and frontmatter stamped onto the generated task. Per spec design pins, body is optional (some recurring tasks only need a title).",
				Properties: map[string]apiextensionsv1.JSONSchemaProps{
					"body": {
						Type:        "string",
						Description: "Raw markdown body of the generated task. Go text/template.",
					},
					"frontmatter": {
						Type: "object",
						Description: "YAML frontmatter stamped onto the generated task. Free-form map of " +
							"operator-defined keys (assignee, priority, goals, category, ...). The publisher " +
							"merges these with three built-in keys (status, page_type, created_by) that always " +
							"win on collision.",
						// XPreserveUnknownFields: frontmatter is operator-defined free-form; the publisher
						// wires it through verbatim (lib.TaskFrontmatter is map[string]interface{}).
						XPreserveUnknownFields: ptrTrue(),
					},
				},
			},
		},
	}
}

// ptrTrue returns a pointer to true; the k8s OpenAPI schema represents
// boolean toggles as *bool so an unset pointer is distinguishable from
// an explicit false.
func ptrTrue() *bool {
	t := true
	return &t
}

// ptrInt64 returns a pointer to the given int64; the k8s OpenAPI schema
// represents MinItems and similar numeric bounds as *int64.
func ptrInt64(n int64) *int64 {
	return &n
}

// jsonEnumValues wraps each string in an apiextensionsv1.JSON so the
// schema's Enum field (which is a []JSON for the raw-JSON-value form)
// accepts strings. The OpenAPI serializer renders the Raw bytes verbatim.
func jsonEnumValues(values []string) []apiextensionsv1.JSON {
	out := make([]apiextensionsv1.JSON, 0, len(values))
	for _, v := range values {
		out = append(out, apiextensionsv1.JSON{Raw: []byte(`"` + v + `"`)})
	}
	return out
}
