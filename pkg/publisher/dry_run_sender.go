// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package publisher

import (
	"context"

	"github.com/bborbe/agent/lib/command/task"
)

// NewNoopSender returns a task.CreateCommandSender whose SendCommand
// method is a no-op (returns nil). Wired by main.go and cmd/run-once
// when DRY_RUN=true to avoid Kafka client init. The publisher's
// dryRun guard skips the sender call anyway; the noop exists so the
// application can construct itself without a real Kafka broker.
func NewNoopSender() task.CreateCommandSender {
	return noopSender{}
}

type noopSender struct{}

func (noopSender) SendCommand(_ context.Context, _ task.CreateCommand) error {
	return nil
}
