// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pkg_test

import (
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"

	"github.com/bborbe/recurring-task-creator/pkg"
)

// buildSelfForCEL returns the map[string]string that the CEL rule
// expects as its `self` binding. When weekday is absent the key is
// omitted from the map so has(self.weekday) returns false.
func buildSelfForCEL(recurrence string, weekday interface{}) map[string]interface{} {
	out := map[string]interface{}{
		"recurrence": recurrence,
	}
	if w, ok := weekday.(string); ok {
		out["weekday"] = w
	}
	return out
}

var _ = Describe("scheduleSpecSchema CEL validation", func() {
	// evalRule runs the CEL rule from XValidations[0] against the given
	// vars and returns the human-readable error message (or "" when the
	// rule passes).
	evalRule := func(vars map[string]interface{}) string {
		rule := pkg.WeekdayRequiredIfWeekdayRuleForTest()
		// The CEL rule is bound to a `self` value of type message with
		// optional string fields. Map types satisfy CEL's map semantics
		// well enough for this test: `has(self.field)` is checked via
		// the map's key-set test that the eval helper unwraps to "absent"
		// when the value is nil and "present" otherwise.
		env, err := cel.NewEnv(
			cel.Variable("self", cel.MapType(cel.StringType, cel.StringType)),
		)
		Expect(err).NotTo(HaveOccurred())
		ast, issues := env.Compile(rule)
		Expect(issues.Err()).NotTo(HaveOccurred(), "compile %q", rule)
		program, err := env.Program(ast)
		Expect(err).NotTo(HaveOccurred())
		out, _, err := program.Eval(map[string]interface{}{"self": vars})
		Expect(err).NotTo(HaveOccurred())
		if b, ok := out.(types.Bool); ok && bool(b) {
			return "" // rule passed
		}
		return pkg.WeekdayRequiredIfWeekdayMessageForTest()
	}

	// validateSpec runs the regex / enum / CEL checks against a
	// map[string]interface{} representation of a Schedule spec. Mirrors
	// what the API server does at admission time.
	validateSpec := func(spec map[string]interface{}) error {
		vault, _ := spec["vault"].(string)
		if !pkg.VaultRegexForTest.MatchString(vault) {
			return fmt.Errorf("vault %q does not match %s", vault, pkg.VaultPatternForTest())
		}
		sched, _ := spec["schedule"].(map[string]interface{})
		recurrence, _ := sched["recurrence"].(string)
		var found bool
		for _, r := range pkg.RecurrenceEnumForTest() {
			if r == recurrence {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("recurrence %q is not in the closed set", recurrence)
		}
		if weekday, ok := sched["weekday"].(string); ok && weekday != "" {
			var weekdayFound bool
			for _, w := range pkg.WeekdayEnumForTest() {
				if w == weekday {
					weekdayFound = true
					break
				}
			}
			if !weekdayFound {
				return fmt.Errorf("weekday %q is not in the closed set", weekday)
			}
		}
		if msg := evalRule(buildSelfForCEL(recurrence, sched["weekday"])); msg != "" {
			return fmt.Errorf("%s", msg)
		}
		return nil
	}

	It("accepts the canonical weekly-review example", func() {
		spec := map[string]interface{}{
			"vault": "personal",
			"title": "Weekly Review",
			"schedule": map[string]interface{}{
				"recurrence": "Weekday",
				"weekday":    "Saturday",
			},
			"template": map[string]interface{}{"body": "Reflect."},
		}
		Expect(validateSpec(spec)).To(Succeed())
	})

	It("accepts a Weekly (always-fire) schedule without weekday", func() {
		spec := map[string]interface{}{
			"vault": "personal",
			"title": "Monday Standup",
			"schedule": map[string]interface{}{
				"recurrence": "Weekly",
				// weekday absent — Weekly is always-fire per Spec 9
			},
			"template": map[string]interface{}{"body": "."},
		}
		Expect(validateSpec(spec)).To(Succeed())
	})

	It("rejects an unknown recurrence value", func() {
		spec := map[string]interface{}{
			"vault": "personal",
			"title": "Foo",
			"schedule": map[string]interface{}{
				"recurrence": "weekly", // lowercase — not in the capital-case enum
			},
		}
		err := validateSpec(spec)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("recurrence"))
	})

	It("rejects a Weekday schedule without weekday", func() {
		spec := map[string]interface{}{
			"vault": "personal",
			"title": "Foo",
			"schedule": map[string]interface{}{
				"recurrence": "Weekday",
				// weekday absent
			},
		}
		err := validateSpec(spec)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("weekday"))
	})

	It("rejects a Weekly schedule that sets weekday (Weekly is always-fire)", func() {
		spec := map[string]interface{}{
			"vault": "personal",
			"title": "Foo",
			"schedule": map[string]interface{}{
				"recurrence": "Weekly",
				"weekday":    "Saturday",
			},
		}
		err := validateSpec(spec)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("weekday"))
	})

	It("rejects a non-weekday schedule that sets weekday", func() {
		spec := map[string]interface{}{
			"vault": "personal",
			"title": "Foo",
			"schedule": map[string]interface{}{
				"recurrence": "Monthly",
				"weekday":    "Saturday",
			},
		}
		err := validateSpec(spec)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("weekday"))
	})

	It("rejects a typo'd weekday value", func() {
		spec := map[string]interface{}{
			"vault": "personal",
			"title": "Foo",
			"schedule": map[string]interface{}{
				"recurrence": "Weekday",
				"weekday":    "Satuday", // typo: missing the 'r'
			},
		}
		err := validateSpec(spec)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("weekday"))
	})

	It("rejects a vault slug that does not match the regex", func() {
		spec := map[string]interface{}{
			"vault": "Bad Vault", // space + uppercase
			"title": "Foo",
			"schedule": map[string]interface{}{
				"recurrence": "Daily",
			},
		}
		err := validateSpec(spec)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("vault"))
	})
})

var _ = Describe("periodOffset CEL validation", func() {
	evalRule := func(self map[string]interface{}) string {
		rule := pkg.PeriodOffsetOnlyForPeriodKindsRuleForTest()
		env, err := cel.NewEnv(
			cel.Variable("self", cel.MapType(cel.StringType, cel.DynType)),
		)
		Expect(err).NotTo(HaveOccurred())
		ast, issues := env.Compile(rule)
		Expect(issues.Err()).NotTo(HaveOccurred(), "compile %q", rule)
		program, err := env.Program(ast)
		Expect(err).NotTo(HaveOccurred())
		out, _, err := program.Eval(map[string]interface{}{"self": self})
		Expect(err).NotTo(HaveOccurred())
		if b, ok := out.(types.Bool); ok && bool(b) {
			return ""
		}
		return pkg.PeriodOffsetOnlyForPeriodKindsMessageForTest()
	}

	DescribeTable(
		"accepts/rejects (recurrence, periodOffset) combos",
		func(recurrence string, withOffset bool, offset int, expectPass bool) {
			self := map[string]interface{}{"recurrence": recurrence}
			if withOffset {
				self["periodOffset"] = offset
			}
			result := evalRule(self)
			if expectPass {
				Expect(result).To(BeEmpty())
			} else {
				Expect(result).To(ContainSubstring("periodOffset"))
			}
		},
		Entry("Monthly + offset=-1 → accept", "Monthly", true, -1, true),
		Entry("Quarterly + offset=-1 → accept", "Quarterly", true, -1, true),
		Entry("Yearly + offset=-1 → accept", "Yearly", true, -1, true),
		Entry("Monthly + offset=+1 → accept", "Monthly", true, 1, true),
		Entry("Monthly + offset=0 → accept", "Monthly", true, 0, true),
		Entry("Daily + offset=0 → accept", "Daily", true, 0, true),
		Entry("Daily without periodOffset → accept", "Daily", false, 0, true),
		Entry("Weekly without periodOffset → accept", "Weekly", false, 0, true),
		Entry("Weekday without periodOffset → accept", "Weekday", false, 0, true),
		Entry("Daily + offset=-1 → reject", "Daily", true, -1, false),
		Entry("Weekly + offset=1 → reject", "Weekly", true, 1, false),
		Entry("Weekday + offset=-1 → reject", "Weekday", true, -1, false),
	)
})

var _ = Describe("weekday enum — all 14 accepted values", func() {
	// validateSpec is duplicated here with local scope to keep the test
	// self-contained (same logic as the outer validateSpec above).
	localValidateSpec := func(spec map[string]interface{}) error {
		vault, _ := spec["vault"].(string)
		if !pkg.VaultRegexForTest.MatchString(vault) {
			return fmt.Errorf("vault %q does not match %s", vault, pkg.VaultPatternForTest())
		}
		sched, _ := spec["schedule"].(map[string]interface{})
		recurrence, _ := sched["recurrence"].(string)
		var found bool
		for _, r := range pkg.RecurrenceEnumForTest() {
			if r == recurrence {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("recurrence %q is not in the closed set", recurrence)
		}
		if weekday, ok := sched["weekday"].(string); ok && weekday != "" {
			var weekdayFound bool
			for _, w := range pkg.WeekdayEnumForTest() {
				if w == weekday {
					weekdayFound = true
					break
				}
			}
			if !weekdayFound {
				return fmt.Errorf("weekday %q is not in the closed set", weekday)
			}
		}
		rule := pkg.WeekdayRequiredIfWeekdayRuleForTest()
		env, err := cel.NewEnv(cel.Variable("self", cel.MapType(cel.StringType, cel.StringType)))
		if err != nil {
			return err
		}
		ast, issues := env.Compile(rule)
		if issues.Err() != nil {
			return issues.Err()
		}
		program, err := env.Program(ast)
		if err != nil {
			return err
		}
		celSelf := map[string]interface{}{"recurrence": recurrence}
		if wd, ok := sched["weekday"].(string); ok {
			celSelf["weekday"] = wd
		}
		out, _, err := program.Eval(map[string]interface{}{"self": celSelf})
		if err != nil {
			return err
		}
		if b, ok := out.(types.Bool); ok && !bool(b) {
			return fmt.Errorf("%s", pkg.WeekdayRequiredIfWeekdayMessageForTest())
		}
		return nil
	}

	DescribeTable(
		"accepts each of the 14 day strings as single-string weekday on Weekday recurrence",
		func(day string) {
			spec := map[string]interface{}{
				"vault": "personal",
				"title": "Task",
				"schedule": map[string]interface{}{
					"recurrence": "Weekday",
					"weekday":    day,
				},
				"template": map[string]interface{}{"body": "."},
			}
			Expect(localValidateSpec(spec)).To(Succeed())
		},
		Entry("Monday", "Monday"),
		Entry("Tuesday", "Tuesday"),
		Entry("Wednesday", "Wednesday"),
		Entry("Thursday", "Thursday"),
		Entry("Friday", "Friday"),
		Entry("Saturday", "Saturday"),
		Entry("Sunday", "Sunday"),
		Entry("Mon", "Mon"),
		Entry("Tue", "Tue"),
		Entry("Wed", "Wed"),
		Entry("Thu", "Thu"),
		Entry("Fri", "Fri"),
		Entry("Sat", "Sat"),
		Entry("Sun", "Sun"),
	)
})

var _ = Describe("weekday list — non-empty CEL rule", func() {
	// evalListRule evaluates a CEL rule where self.weekday may be a list.
	// Uses cel.DynType for map values to accommodate both string and list.
	evalListRule := func(rule string, failMsg string, self map[string]interface{}) string {
		env, err := cel.NewEnv(
			cel.Variable("self", cel.MapType(cel.StringType, cel.DynType)),
		)
		Expect(err).NotTo(HaveOccurred())
		ast, issues := env.Compile(rule)
		Expect(issues.Err()).NotTo(HaveOccurred(), "compile %q", rule)
		program, err := env.Program(ast)
		Expect(err).NotTo(HaveOccurred())
		out, _, err := program.Eval(map[string]interface{}{"self": self})
		Expect(err).NotTo(HaveOccurred())
		if b, ok := out.(types.Bool); ok && bool(b) {
			return ""
		}
		return failMsg
	}

	DescribeTable(
		"non-empty list rule",
		func(weekday interface{}, expectPass bool) {
			self := map[string]interface{}{}
			if weekday != nil {
				self["weekday"] = weekday
			}
			result := evalListRule(
				pkg.WeekdayListNonEmptyRuleForTest(),
				pkg.WeekdayListNonEmptyMessageForTest(),
				self,
			)
			if expectPass {
				Expect(result).To(BeEmpty(), "expected rule to pass for weekday=%v", weekday)
			} else {
				Expect(result).To(ContainSubstring("non-empty"), "expected rule to fail for weekday=%v", weekday)
			}
		},
		Entry("absent weekday → pass (rule short-circuits)", nil, true),
		Entry("empty list [] → reject", []interface{}{}, false),
		Entry("single-element list [Mon] → accept", []interface{}{"Mon"}, true),
		Entry("multi-element list [Mon,Wed] → accept", []interface{}{"Mon", "Wed"}, true),
		Entry("single string Monday → accept (non-list, rule short-circuits)", "Monday", true),
	)
})

var _ = Describe("weekday list — no-duplicate CEL rule", func() {
	evalListRule := func(rule string, failMsg string, self map[string]interface{}) string {
		env, err := cel.NewEnv(
			cel.Variable("self", cel.MapType(cel.StringType, cel.DynType)),
		)
		Expect(err).NotTo(HaveOccurred())
		ast, issues := env.Compile(rule)
		Expect(issues.Err()).NotTo(HaveOccurred(), "compile %q", rule)
		program, err := env.Program(ast)
		Expect(err).NotTo(HaveOccurred())
		out, _, err := program.Eval(map[string]interface{}{"self": self})
		Expect(err).NotTo(HaveOccurred())
		if b, ok := out.(types.Bool); ok && bool(b) {
			return ""
		}
		return failMsg
	}

	DescribeTable(
		"no-duplicate rule rejects same logical day in any form",
		func(weekday interface{}, expectPass bool) {
			self := map[string]interface{}{}
			if weekday != nil {
				self["weekday"] = weekday
			}
			result := evalListRule(
				pkg.WeekdayNoDuplicateRuleForTest(),
				pkg.WeekdayNoDuplicateMessageForTest(),
				self,
			)
			if expectPass {
				Expect(
					result,
				).To(BeEmpty(), "expected no-duplicate rule to pass for weekday=%v", weekday)
			} else {
				Expect(result).To(ContainSubstring("duplicate"), "expected no-duplicate rule to fail for weekday=%v", weekday)
			}
		},
		Entry(
			"[Mon,Monday] → reject (cross-form duplicate)",
			[]interface{}{"Mon", "Monday"},
			false,
		),
		Entry(
			"[Monday,Mon] → reject (cross-form duplicate reversed)",
			[]interface{}{"Monday", "Mon"},
			false,
		),
		Entry("[Tue,Tue] → reject (same-form duplicate)", []interface{}{"Tue", "Tue"}, false),
		Entry(
			"[Wednesday,Wednesday] → reject (same-form long duplicate)",
			[]interface{}{"Wednesday", "Wednesday"},
			false,
		),
		Entry("[Mon,Wed,Fri] → accept", []interface{}{"Mon", "Wed", "Fri"}, true),
		Entry(
			"[Mon,Tue,Wednesday,Thu,Fri] → accept (mixed forms)",
			[]interface{}{"Mon", "Tue", "Wednesday", "Thu", "Fri"},
			true,
		),
		Entry("single string Monday → accept (non-list, rule short-circuits)", "Monday", true),
		Entry("absent weekday → accept (rule short-circuits)", nil, true),
	)
})

var _ = Describe("weekday enum — FunDay rejected", func() {
	It("FunDay is not in the 14-element weekday enum", func() {
		Expect(pkg.WeekdayEnumForTest()).To(HaveLen(14))
		Expect(pkg.WeekdayEnumForTest()).NotTo(ContainElement("FunDay"))
	})

	It("rejects a list containing an unknown day value", func() {
		// The array-items enum check rejects FunDay before CEL rules run.
		// Verify via the enum slice directly: a list entry not in the enum
		// would be rejected by the array branch of the OneOf schema.
		funDay := "FunDay"
		found := false
		for _, w := range pkg.WeekdayEnumForTest() {
			if w == funDay {
				found = true
				break
			}
		}
		Expect(found).To(BeFalse(), "FunDay must not be in the weekday enum")
	})
})

var _ = Describe("Schedule CRD structural schema round-trip", func() {
	It("converts the full CR schema to structural schema without error", func() {
		// Mirrors the path the apiserver uses when admitting a CRD. Any
		// structural-schema violation in the OneOf weekday shape would surface
		// here before it surfaces in production.
		v1Schema := pkg.ScheduleCRSchemaPtrForTest()
		Expect(v1Schema).NotTo(BeNil())

		var internalSchema apiextensions.JSONSchemaProps
		err := apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(
			v1Schema, &internalSchema, nil,
		)
		Expect(err).NotTo(HaveOccurred(), "v1→internal conversion must succeed")

		_, err = structuralschema.NewStructural(&internalSchema)
		Expect(err).NotTo(HaveOccurred(), "NewStructural must accept the converted schema")
	})
})
