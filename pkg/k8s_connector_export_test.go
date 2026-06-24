// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pkg

import (
	"regexp"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// VaultPatternForTest returns the regex the schema applies to spec.vault.
func VaultPatternForTest() string { return vaultPattern }

// RecurrenceEnumForTest returns the closed set of valid recurrence strings.
func RecurrenceEnumForTest() []string { return recurrenceEnum }

// WeekdayEnumForTest returns the closed set of valid weekday strings (14 values: long + short forms).
func WeekdayEnumForTest() []string { return weekdayEnum }

// WeekdayRequiredIfWeekdayRuleForTest returns the CEL rule from XValidations[0].
func WeekdayRequiredIfWeekdayRuleForTest() string { return weekdayRequiredIfWeekdayRule }

// WeekdayRequiredIfWeekdayMessageForTest returns the human-readable error
// message the API server emits when the CEL rule fails.
func WeekdayRequiredIfWeekdayMessageForTest() string { return weekdayRequiredIfWeekdayMessage }

// VaultRegexForTest returns a pre-compiled *regexp.Regexp matching vaultPattern.
// Used by the validation test's validateSpec helper.
var VaultRegexForTest = regexp.MustCompile(vaultPattern)

// PeriodOffsetOnlyForPeriodKindsRuleForTest returns the CEL rule that
// rejects non-zero periodOffset on date-anchored recurrence kinds.
func PeriodOffsetOnlyForPeriodKindsRuleForTest() string {
	return periodOffsetOnlyForPeriodKindsRule
}

// PeriodOffsetOnlyForPeriodKindsMessageForTest returns the operator-facing
// error message for the rule above.
func PeriodOffsetOnlyForPeriodKindsMessageForTest() string {
	return periodOffsetOnlyForPeriodKindsMessage
}

// WeekdayListNonEmptyRuleForTest returns the CEL rule rejecting empty weekday lists.
func WeekdayListNonEmptyRuleForTest() string { return weekdayListNonEmptyRule }

// WeekdayListNonEmptyMessageForTest returns the operator-facing empty-list message.
func WeekdayListNonEmptyMessageForTest() string { return weekdayListNonEmptyMessage }

// WeekdayNoDuplicateRuleForTest returns the CEL rule rejecting duplicate weekday entries.
func WeekdayNoDuplicateRuleForTest() string { return weekdayNoDuplicateRule }

// WeekdayNoDuplicateMessageForTest returns the operator-facing duplicate-day message.
func WeekdayNoDuplicateMessageForTest() string { return weekdayNoDuplicateMessage }

// ScheduleCRSchemaPtrForTest returns the OpenAPI v3 schema for the whole
// Schedule CR. Exposed for the structural-schema round-trip test in pkg_test.
func ScheduleCRSchemaPtrForTest() *apiextensionsv1.JSONSchemaProps { return scheduleCRSchemaPtr() }
