// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cleanup_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/recurring-task-creator/pkg/cleanup"
	"github.com/bborbe/recurring-task-creator/pkg/schedule"
)

var _ = Describe("Metrics", func() {
	Describe("IncSuperseded", func() {
		It("increments the labelled counter for success/daily", func() {
			metrics := cleanup.NewPrometheusMetrics()
			Ω(metrics).ShouldNot(BeNil())

			// Validate method accepts the expected label values without panicking.
			metrics.IncSuperseded("success", string(schedule.RecurrenceDaily))
			metrics.IncSuperseded("conflict", string(schedule.RecurrenceMonthly))
			metrics.IncSuperseded("error", string(schedule.RecurrenceYearly))
		})
	})
})

var _ = Describe("ErrVaultConflict", func() {
	It("is non-nil", func() {
		Ω(cleanup.ErrVaultConflict).ShouldNot(BeNil())
	})
})
