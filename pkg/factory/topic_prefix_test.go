// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package factory_test

import (
	"context"

	"github.com/bborbe/agent/command/task"
	"github.com/bborbe/cqrs/base"
	kafkamocks "github.com/bborbe/kafka/mocks"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/recurring-task-creator/pkg/factory"
)

// This golden test proves the exact Kafka topic name published by
// factory.CreateCommandSender (via cdb.NewCommandObjectSender, wired the
// same way as main.go / cmd/run-once/main.go), for a non-empty TopicPrefix
// (both dev and prod deploy values) and for an empty one (unprefixed
// topic, no leading dash).
var _ = Describe("Published command topic (TopicPrefix golden test)", func() {
	var (
		ctx          context.Context
		fakeProducer *kafkamocks.KafkaSyncProducer
	)

	BeforeEach(func() {
		ctx = context.Background()
		fakeProducer = &kafkamocks.KafkaSyncProducer{}
		fakeProducer.SendMessageReturns(0, 0, nil)
	})

	sendTestCommand := func(sender task.CreateCommandSender) {
		Expect(sender.SendCommand(ctx, task.CreateCommand{
			TaskIdentifier: "test-uuid-1234",
			Title:          "Test Task",
		})).To(Succeed())
	}

	Context("with the develop (dev) TopicPrefix", func() {
		It("publishes to the prefixed topic", func() {
			sender := factory.CreateCommandSender(fakeProducer, base.TopicPrefix("develop"), false)

			sendTestCommand(sender)

			Expect(fakeProducer.SendMessageCallCount()).To(Equal(1))
			_, msg := fakeProducer.SendMessageArgsForCall(0)
			Expect(msg.Topic).To(Equal("develop-agent-task-v1-request"))
		})
	})

	Context("with the master (prod) TopicPrefix", func() {
		It("publishes to the prefixed topic", func() {
			sender := factory.CreateCommandSender(fakeProducer, base.TopicPrefix("master"), false)

			sendTestCommand(sender)

			Expect(fakeProducer.SendMessageCallCount()).To(Equal(1))
			_, msg := fakeProducer.SendMessageArgsForCall(0)
			Expect(msg.Topic).To(Equal("master-agent-task-v1-request"))
		})
	})

	Context("with an empty TopicPrefix", func() {
		It("publishes to the unprefixed topic (no leading dash)", func() {
			sender := factory.CreateCommandSender(fakeProducer, base.TopicPrefix(""), false)

			sendTestCommand(sender)

			Expect(fakeProducer.SendMessageCallCount()).To(Equal(1))
			_, msg := fakeProducer.SendMessageArgsForCall(0)
			Expect(msg.Topic).To(Equal("agent-task-v1-request"))
		})
	})
})
