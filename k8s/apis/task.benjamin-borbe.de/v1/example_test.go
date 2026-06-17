// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package v1_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"sigs.k8s.io/yaml"

	v1 "github.com/bborbe/recurring-task-creator/k8s/apis/task.benjamin-borbe.de/v1"
)

// file path resolution: example_test.go is at k8s/apis/.../v1/example_test.go,
// testdata is at k8s/apis/.../v1/testdata/example.yaml. `testdata/` is the
// conventional Go testdata directory; `os.ReadFile("testdata/example.yaml")`
// resolves relative to the package's source directory.
const examplePath = "testdata/example.yaml"

var _ = Describe("example.yaml", func() {
	var (
		raw []byte
		sch v1.Schedule
	)

	BeforeEach(func() {
		path, err := filepath.Abs(examplePath)
		Expect(err).NotTo(HaveOccurred())
		raw, err = os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred(), "read %s", path)
		Expect(raw).NotTo(BeEmpty())
	})

	It("parses as a canonical Schedule", func() {
		Expect(yaml.UnmarshalStrict(raw, &sch)).To(Succeed())
	})

	It("has the frozen apiVersion and kind", func() {
		Expect(yaml.UnmarshalStrict(raw, &sch)).To(Succeed())
		Expect(sch.APIVersion).To(Equal("task.benjamin-borbe.de/v1"))
		Expect(sch.Kind).To(Equal("Schedule"))
		Expect(sch.Name).To(Equal("weekly-review"))
		Expect(sch.Namespace).To(Equal("default"))
	})

	It("round-trips every Spec field", func() {
		Expect(yaml.UnmarshalStrict(raw, &sch)).To(Succeed())
		Expect(sch.Spec.Vault).To(Equal("default"))
		Expect(sch.Spec.Title).To(Equal("Weekly Review"))
		Expect(sch.Spec.Schedule.Recurrence).To(Equal("Weekday"))
		Expect(sch.Spec.Schedule.Weekday).To(Equal("Saturday"))
		Expect(sch.Spec.Template.Body).To(ContainSubstring("Reflect on the past week."))
		// sigs.k8s.io/yaml round-trips YAML through JSON, so YAML integers
		// become float64 in a map[string]interface{}; assert numerically.
		Expect(sch.Spec.Template.Frontmatter["priority"]).To(BeNumerically("==", 2))
		Expect(sch.Spec.Template.Frontmatter).To(HaveKeyWithValue("recurring", "Weekday"))
	})

	It("rejects an unknown field (strict-unmarshal guard)", func() {
		bad := []byte(`apiVersion: task.benjamin-borbe.de/v1
kind: Schedule
metadata:
  name: weekly-review
  namespace: default
spec:
  vault: default
  title: Weekly Review
  schedule:
    recurrence: Weekday
    weeday: Saturday
`)
		var s v1.Schedule
		err := yaml.UnmarshalStrict(bad, &s)
		Expect(err).To(HaveOccurred(), "strict-unmarshal must reject misspelled weekday field")
		Expect(err.Error()).To(ContainSubstring("weeday"))
	})
})

func TestSuite(t *testing.T) {
	time.Local = time.UTC
	format.TruncatedDiff = false
	RegisterFailHandler(Fail)
	suiteConfig, reporterConfig := GinkgoConfiguration()
	suiteConfig.Timeout = 60 * time.Second
	RunSpecs(t, "v1 Suite", suiteConfig, reporterConfig)
}
