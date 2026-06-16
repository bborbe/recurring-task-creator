// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tick_test

import (
	"context"

	libtime "github.com/bborbe/time"
	libtimetest "github.com/bborbe/time/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	pubmocks "github.com/bborbe/recurring-task-creator/mocks"
	"github.com/bborbe/recurring-task-creator/pkg/schedule"
	"github.com/bborbe/recurring-task-creator/pkg/tick"
)

var _ = Describe("NewPrometheusMetrics", func() {
	It("returns a non-nil Metrics", func() {
		Expect(tick.NewPrometheusMetrics()).NotTo(BeNil())
	})
})

var _ = Describe("RunOnce", func() {
	It("publishes every entry that fires on the civil date and returns nil", func() {
		pub := &pubmocks.PublisherPublisher{}
		pub.PublishReturns(nil)
		clock := libtime.NewCurrentDateTime()
		clock.SetNow(libtimetest.ParseDateTime("2025-01-04T10:00:00Z"))
		metrics := &pubmocks.TickMetrics{}
		tk, err := tick.NewTick(
			context.Background(),
			schedule.Inventory(),
			pub,
			clock,
			metrics,
		)
		Expect(err).NotTo(HaveOccurred())

		// 2025-01-04 (Saturday) yields 24 always-fire + 12 Saturday weekday = 36.
		err = tk.RunOnce(context.Background())
		Expect(err).To(Succeed())
		Expect(pub.PublishCallCount()).To(Equal(36))
	})
})
