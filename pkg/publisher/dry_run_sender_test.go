// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package publisher_test

import (
	"context"

	"github.com/bborbe/agent/lib/command/task"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/recurring-task-creator/pkg/publisher"
)

var _ = Describe("NoopSender", func() {
	It("SendCommand returns nil without contacting any broker", func() {
		sender := publisher.NewNoopSender()
		err := sender.SendCommand(context.Background(), task.CreateCommand{})
		Expect(err).To(Succeed())
	})
})
