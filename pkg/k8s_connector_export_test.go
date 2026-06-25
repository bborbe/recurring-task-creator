// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pkg

import (
	"regexp"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/bborbe/recurring-task-creator/k8s/apis/task.benjamin-borbe.de/v1"
)

// VaultPatternForTest returns the regex the schema applies to spec.vault.
func VaultPatternForTest() string { return vaultPattern }

// RecurrenceEnumForTest returns the closed set of valid recurrence strings.
func RecurrenceEnumForTest() []string { return recurrenceEnum }

// WeekdayLongEnumForTest returns the closed set of valid strings for the
// single `weekday` field (7 long forms).
func WeekdayLongEnumForTest() []string { return weekdayLongEnum }

// WeekdayAllEnumForTest returns the closed set of valid item strings for
// the `weekdays` list field (14 long + short forms).
func WeekdayAllEnumForTest() []string { return weekdayAllEnum }

// WeekdayXorRuleForTest returns the CEL XOR rule from XValidations[0].
func WeekdayXorRuleForTest() string { return weekdayXorRule }

// WeekdayXorMessageForTest returns the operator-facing XOR error message.
func WeekdayXorMessageForTest() string { return weekdayXorMessage }

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

// WeekdayNoDuplicateRuleForTest returns the CEL rule rejecting duplicate weekday entries.
func WeekdayNoDuplicateRuleForTest() string { return weekdayNoDuplicateRule }

// WeekdayNoDuplicateMessageForTest returns the operator-facing duplicate-day message.
func WeekdayNoDuplicateMessageForTest() string { return weekdayNoDuplicateMessage }

// ScheduleCRSchemaPtrForTest returns the OpenAPI v3 schema for the whole
// Schedule CR. Exposed for the structural-schema round-trip test in pkg_test.
func ScheduleCRSchemaPtrForTest() *apiextensionsv1.JSONSchemaProps { return scheduleCRSchemaPtr() }

// DesiredScheduleCRDForTest returns the fully-assembled v1 CustomResourceDefinition
// the connector installs — ObjectMeta.Name, group, names, versions, and the
// OpenAPIV3Schema with all x-kubernetes-validations rules. Exposed so the CEL
// cost-budget regression-lock test can run the exact object through the API
// server's admission validator.
func DesiredScheduleCRDForTest() *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: v1.Plural + "." + v1.GroupName},
		Spec:       (&k8sConnector{}).desiredCRDSpec(),
	}
}
