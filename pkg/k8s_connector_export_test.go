// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pkg

import "regexp"

// VaultPatternForTest returns the regex the schema applies to spec.vault.
func VaultPatternForTest() string { return vaultPattern }

// RecurrenceEnumForTest returns the closed set of valid recurrence strings.
func RecurrenceEnumForTest() []string { return recurrenceEnum }

// WeekdayRequiredIfWeeklyRuleForTest returns the CEL rule from XValidations[0].
func WeekdayRequiredIfWeeklyRuleForTest() string { return weekdayRequiredIfWeeklyRule }

// WeekdayRequiredIfWeeklyMessageForTest returns the human-readable error
// message the API server emits when the CEL rule fails.
func WeekdayRequiredIfWeeklyMessageForTest() string { return weekdayRequiredIfWeeklyMessage }

// VaultRegexForTest returns a pre-compiled *regexp.Regexp matching vaultPattern.
// Used by the validation test's validateSpec helper.
var VaultRegexForTest = regexp.MustCompile(vaultPattern)
