// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tick_test

import (
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("package surface", func() {
	It("imports no forbidden packages", func() {
		forbidden := []string{
			`"net/http"`,
			`"github.com/segmentio/kafka-go"`,
			`"github.com/IBM/sarama"`,
			`"github.com/bborbe/jira-task-creator"`,
			"time.Now()",
		}
		entries, err := os.ReadDir(".")
		Expect(err).NotTo(HaveOccurred())
		for _, e := range entries {
			name := e.Name()
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			data, err := os.ReadFile(name)
			Expect(err).NotTo(HaveOccurred(), name)
			text := string(data)
			for _, f := range forbidden {
				Expect(strings.Contains(text, f)).To(BeFalse(),
					"%s contains forbidden token %s", name, f)
			}
			Expect(strings.Contains(text, `"github.com/bborbe/jira-task-creator/`)).
				To(BeFalse(), "%s imports a github.com/bborbe/jira-task-creator/... subpackage", name)
		}
	})
})
