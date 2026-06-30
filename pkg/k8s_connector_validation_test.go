// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pkg_test

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsvalidation "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/validation"
	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"

	"github.com/bborbe/recurring-task-creator/pkg"
)

// buildSelfForXor builds the self map for the weekday XOR CEL rule.
// weekday is a string or nil (absent); weekdays is a []interface{} or nil (absent).
func buildSelfForXor(
	recurrence string,
	weekday interface{},
	weekdays interface{},
) map[string]interface{} {
	out := map[string]interface{}{
		"recurrence": recurrence,
	}
	if s, ok := weekday.(string); ok {
		out["weekday"] = s
	}
	if l, ok := weekdays.([]interface{}); ok {
		out["weekdays"] = l
	}
	return out
}

// evalXorRule runs the weekday XOR CEL rule and returns "" on pass or the
// error message on failure. Uses DynType to support both string weekday and
// list weekdays fields in the same self map.
func evalXorRule(vars map[string]interface{}) string {
	rule := pkg.WeekdayXorRuleForTest()
	env, err := cel.NewEnv(
		cel.Variable("self", cel.MapType(cel.StringType, cel.DynType)),
	)
	Expect(err).NotTo(HaveOccurred())
	ast, issues := env.Compile(rule)
	Expect(issues.Err()).NotTo(HaveOccurred(), "compile %q", rule)
	program, err := env.Program(ast)
	Expect(err).NotTo(HaveOccurred())
	out, _, err := program.Eval(map[string]interface{}{"self": vars})
	Expect(err).NotTo(HaveOccurred())
	if b, ok := out.(types.Bool); ok && bool(b) {
		return ""
	}
	return pkg.WeekdayXorMessageForTest()
}

// containsString returns true when needle is present in haystack.
func containsString(haystack []string, needle string) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}

// validateWeekdaysItems checks every item in a weekdays list against the 14-value enum.
func validateWeekdaysItems(weekdays []interface{}) error {
	for _, item := range weekdays {
		s, _ := item.(string)
		if !containsString(pkg.WeekdayAllEnumForTest(), s) {
			return fmt.Errorf("weekdays item %q is not in the 14-value weekday enum", s)
		}
	}
	return nil
}

// validateSpec runs vault-regex / recurrence-enum / weekday-enum / XOR-CEL
// checks against a map[string]interface{} representation of a Schedule spec.
// Mirrors what the API server does at admission time for the two-field shape.
func validateSpec(spec map[string]interface{}) error {
	vault, _ := spec["vault"].(string)
	if !pkg.VaultRegexForTest.MatchString(vault) {
		return fmt.Errorf("vault %q does not match %s", vault, pkg.VaultPatternForTest())
	}
	sched, _ := spec["schedule"].(map[string]interface{})
	recurrence, _ := sched["recurrence"].(string)
	if !containsString(pkg.RecurrenceEnumForTest(), recurrence) {
		return fmt.Errorf("recurrence %q is not in the closed set", recurrence)
	}
	if weekday, ok := sched["weekday"].(string); ok && weekday != "" {
		if !containsString(pkg.WeekdayLongEnumForTest(), weekday) {
			return fmt.Errorf("weekday %q is not in the weekday long-form enum", weekday)
		}
	}
	if weekdays, ok := sched["weekdays"].([]interface{}); ok {
		if err := validateWeekdaysItems(weekdays); err != nil {
			return err
		}
	}
	if msg := evalXorRule(buildSelfForXor(recurrence, sched["weekday"], sched["weekdays"])); msg != "" {
		return fmt.Errorf("%s", msg)
	}
	return nil
}

var _ = Describe("scheduleSpecSchema CEL validation", func() {
	It("accepts Weekday + single long-form weekday: Saturday", func() {
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

	It("accepts Weekday + weekdays list [Mon, Wed, Fri]", func() {
		spec := map[string]interface{}{
			"vault": "personal",
			"title": "Standup",
			"schedule": map[string]interface{}{
				"recurrence": "Weekday",
				"weekdays":   []interface{}{"Mon", "Wed", "Fri"},
			},
			"template": map[string]interface{}{"body": "."},
		}
		Expect(validateSpec(spec)).To(Succeed())
	})

	It("accepts Weekday + weekdays list with single element [Monday]", func() {
		spec := map[string]interface{}{
			"vault": "personal",
			"title": "Task",
			"schedule": map[string]interface{}{
				"recurrence": "Weekday",
				"weekdays":   []interface{}{"Monday"},
			},
			"template": map[string]interface{}{"body": "."},
		}
		Expect(validateSpec(spec)).To(Succeed())
	})

	It("accepts Weekly (always-fire) with neither weekday nor weekdays", func() {
		spec := map[string]interface{}{
			"vault": "personal",
			"title": "Monday Standup",
			"schedule": map[string]interface{}{
				"recurrence": "Weekly",
			},
			"template": map[string]interface{}{"body": "."},
		}
		Expect(validateSpec(spec)).To(Succeed())
	})

	It("accepts Daily with neither weekday nor weekdays", func() {
		spec := map[string]interface{}{
			"vault": "personal",
			"title": "Daily Task",
			"schedule": map[string]interface{}{
				"recurrence": "Daily",
			},
			"template": map[string]interface{}{"body": "."},
		}
		Expect(validateSpec(spec)).To(Succeed())
	})

	It("rejects Weekday with neither weekday nor weekdays set (XOR)", func() {
		spec := map[string]interface{}{
			"vault": "personal",
			"title": "Foo",
			"schedule": map[string]interface{}{
				"recurrence": "Weekday",
			},
		}
		err := validateSpec(spec)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("exactly one"))
	})

	It("rejects Weekday with both weekday and weekdays set (XOR)", func() {
		spec := map[string]interface{}{
			"vault": "personal",
			"title": "Foo",
			"schedule": map[string]interface{}{
				"recurrence": "Weekday",
				"weekday":    "Monday",
				"weekdays":   []interface{}{"Mon"},
			},
		}
		err := validateSpec(spec)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("exactly one"))
	})

	It("rejects Daily + weekday: Monday (field on non-Weekday)", func() {
		spec := map[string]interface{}{
			"vault": "personal",
			"title": "Foo",
			"schedule": map[string]interface{}{
				"recurrence": "Daily",
				"weekday":    "Monday",
			},
		}
		err := validateSpec(spec)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("exactly one"))
	})

	It("rejects Daily + weekdays: [Mon] (field on non-Weekday)", func() {
		spec := map[string]interface{}{
			"vault": "personal",
			"title": "Foo",
			"schedule": map[string]interface{}{
				"recurrence": "Daily",
				"weekdays":   []interface{}{"Mon"},
			},
		}
		err := validateSpec(spec)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("exactly one"))
	})

	It("rejects Weekly + weekdays: [Mon, Wed] (field on non-Weekday)", func() {
		spec := map[string]interface{}{
			"vault": "personal",
			"title": "Foo",
			"schedule": map[string]interface{}{
				"recurrence": "Weekly",
				"weekdays":   []interface{}{"Mon", "Wed"},
			},
		}
		err := validateSpec(spec)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("exactly one"))
	})

	It("rejects weekday: Mon (short form — not in weekdayLongEnum)", func() {
		Expect(pkg.WeekdayLongEnumForTest()).NotTo(ContainElement("Mon"))
		spec := map[string]interface{}{
			"vault": "personal",
			"title": "Foo",
			"schedule": map[string]interface{}{
				"recurrence": "Weekday",
				"weekday":    "Mon",
			},
		}
		err := validateSpec(spec)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("weekday"))
	})

	It("rejects weekday: Satuday (typo — not in weekdayLongEnum)", func() {
		spec := map[string]interface{}{
			"vault": "personal",
			"title": "Foo",
			"schedule": map[string]interface{}{
				"recurrence": "Weekday",
				"weekday":    "Satuday",
			},
		}
		err := validateSpec(spec)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("weekday"))
	})

	It("rejects a vault slug that does not match the regex", func() {
		spec := map[string]interface{}{
			"vault": "Bad Vault",
			"title": "Foo",
			"schedule": map[string]interface{}{
				"recurrence": "Daily",
			},
		}
		err := validateSpec(spec)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("vault"))
	})

	It("rejects an unknown recurrence value (lowercase)", func() {
		spec := map[string]interface{}{
			"vault": "personal",
			"title": "Foo",
			"schedule": map[string]interface{}{
				"recurrence": "weekly",
			},
		}
		err := validateSpec(spec)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("recurrence"))
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

var _ = Describe("weekday long-form enum — all 7 accepted values on single weekday field", func() {
	DescribeTable(
		"accepts each of the 7 long-form day strings as weekday on Weekday recurrence",
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
			Expect(validateSpec(spec)).To(Succeed())
		},
		Entry("Monday", "Monday"),
		Entry("Tuesday", "Tuesday"),
		Entry("Wednesday", "Wednesday"),
		Entry("Thursday", "Thursday"),
		Entry("Friday", "Friday"),
		Entry("Saturday", "Saturday"),
		Entry("Sunday", "Sunday"),
	)
})

var _ = Describe("weekday enum lengths", func() {
	It("WeekdayLongEnumForTest has 7 elements", func() {
		Expect(pkg.WeekdayLongEnumForTest()).To(HaveLen(7))
	})

	It("WeekdayAllEnumForTest has 14 elements", func() {
		Expect(pkg.WeekdayAllEnumForTest()).To(HaveLen(14))
	})

	It("Mon is not in WeekdayLongEnumForTest (short forms rejected on single field)", func() {
		Expect(pkg.WeekdayLongEnumForTest()).NotTo(ContainElement("Mon"))
	})

	It("FunDay is not in WeekdayAllEnumForTest", func() {
		Expect(pkg.WeekdayAllEnumForTest()).NotTo(ContainElement("FunDay"))
	})

	It("rejects a weekdays item with unknown day value", func() {
		funDay := "FunDay"
		found := false
		for _, w := range pkg.WeekdayAllEnumForTest() {
			if w == funDay {
				found = true
				break
			}
		}
		Expect(found).To(BeFalse(), "FunDay must not be in the weekdays item enum")
	})
})

var _ = Describe("weekdays list — no-duplicate CEL rule", func() {
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
		func(weekdays interface{}, expectPass bool) {
			self := map[string]interface{}{}
			if weekdays != nil {
				self["weekdays"] = weekdays
			}
			result := evalListRule(
				pkg.WeekdayNoDuplicateRuleForTest(),
				pkg.WeekdayNoDuplicateMessageForTest(),
				self,
			)
			if expectPass {
				Expect(
					result,
				).To(BeEmpty(), "expected no-duplicate rule to pass for weekdays=%v", weekdays)
			} else {
				Expect(result).To(ContainSubstring("duplicate"), "expected no-duplicate rule to fail for weekdays=%v", weekdays)
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
		Entry("absent weekdays → accept (rule short-circuits)", nil, true),
	)
})

var _ = Describe("weekdays MaxItems bound", func() {
	It("weekdays property carries MaxItems == 7 and keeps MinItems == 1", func() {
		schema := pkg.ScheduleCRSchemaPtrForTest()
		weekdays := schema.Properties["spec"].Properties["schedule"].Properties["weekdays"]
		Expect(weekdays.MaxItems).NotTo(BeNil(), "weekdays must declare MaxItems")
		Expect(*weekdays.MaxItems).To(Equal(int64(7)))
		Expect(weekdays.MinItems).NotTo(BeNil(), "weekdays must keep MinItems")
		Expect(*weekdays.MinItems).To(Equal(int64(1)))
	})
})

var _ = Describe("Schedule CRD CEL cost-budget regression-lock", func() {
	It(
		"no x-kubernetes-validations rule on the assembled CRD exceeds the API-server per-rule cost budget",
		func() {
			// Convert the assembled v1 CRD to the internal apiextensions type, then
			// run it through the exact public function the API server uses at CRD
			// admission. ValidateCustomResourceDefinition compiles every CEL rule,
			// estimates worst-case cost using each array's maxItems, and emits a
			// field.Forbidden error whose detail contains "exceeds budget" when a
			// rule's estimated cost exceeds StaticEstimatedCostLimit. This is a
			// byte-equivalent verdict to production admission. Regression lock for
			// the spec-014 CrashLoopBackOff: this It FAILS on the pre-fix schema
			// (unbounded weekdays + nested map().all().exists_one()) and PASSES
			// after MaxItems:7 + the rewritten dup rule.
			v1CRD := pkg.DesiredScheduleCRDForTest()
			Expect(v1CRD).NotTo(BeNil())

			var internalCRD apiextensions.CustomResourceDefinition
			err := apiextensionsv1.Convert_v1_CustomResourceDefinition_To_apiextensions_CustomResourceDefinition(
				v1CRD,
				&internalCRD,
				nil,
			)
			Expect(err).NotTo(HaveOccurred(), "v1->internal CRD conversion must succeed")

			errs := apiextensionsvalidation.ValidateCustomResourceDefinition(
				context.Background(),
				&internalCRD,
			)

			var costErrs []string
			for _, e := range errs {
				if strings.Contains(e.Detail, "exceeds budget") {
					costErrs = append(costErrs, e.Error())
				}
			}
			Expect(
				costErrs,
			).To(BeEmpty(), "no CEL rule may exceed the per-rule cost budget; offending rules: %v", costErrs)
		},
	)
})

var _ = Describe("Schedule CRD structural schema round-trip", func() {
	It("converts the full CR schema to structural schema without error", func() {
		// Mirrors the path the apiserver uses when admitting a CRD. Any
		// structural-schema violation in the weekday shape would surface
		// here before it surfaces in production.
		v1Schema := pkg.ScheduleCRSchemaPtrForTest()
		Expect(v1Schema).NotTo(BeNil())

		var internalSchema apiextensions.JSONSchemaProps
		err := apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(
			v1Schema, &internalSchema, nil,
		)
		Expect(err).NotTo(HaveOccurred(), "v1→internal conversion must succeed")

		ss, err := structuralschema.NewStructural(&internalSchema)
		Expect(err).NotTo(HaveOccurred(), "NewStructural must accept the converted schema")

		// ValidateStructural checks structural-schema compliance — this is what
		// the API server's CRD admission webhook actually runs. It rejects schemas
		// with OneOf branches carrying their own type (non-structural). A passing
		// result here is the regression lock against the Spec-012 CrashLoopBackOff bug.
		errs := structuralschema.ValidateStructural(nil, ss)
		Expect(errs).To(BeEmpty(), "ValidateStructural must accept the converted schema: %v", errs)
	})
})

var _ = Describe("autoAbortPrior schema", func() {
	It("declares autoAbortPrior as a boolean property, no enum, not required", func() {
		schema := pkg.ScheduleCRSchemaPtrForTest()
		trigger := schema.Properties["spec"].Properties["schedule"]
		prop, ok := trigger.Properties["autoAbortPrior"]
		Expect(ok).To(BeTrue(), "schedule schema must declare autoAbortPrior")
		Expect(prop.Type).To(Equal("boolean"))
		Expect(prop.Enum).To(BeEmpty(), "autoAbortPrior must not be an enum")
		Expect(trigger.Required).NotTo(ContainElement("autoAbortPrior"),
			"autoAbortPrior must remain optional")
	})

	It("the schema with autoAbortPrior round-trips through structural-schema validation", func() {
		v1Schema := pkg.ScheduleCRSchemaPtrForTest()
		var internalSchema apiextensions.JSONSchemaProps
		err := apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(
			v1Schema, &internalSchema, nil,
		)
		Expect(err).NotTo(HaveOccurred())
		ss, err := structuralschema.NewStructural(&internalSchema)
		Expect(err).NotTo(HaveOccurred())
		Expect(structuralschema.ValidateStructural(nil, ss)).To(BeEmpty())
	})
})
