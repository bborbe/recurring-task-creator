// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package publisher

import (
	"context"

	lib "github.com/bborbe/agent"
	"github.com/bborbe/errors"
	"github.com/google/uuid"

	"github.com/bborbe/recurring-task-creator/pkg/schedule"
)

//counterfeiter:generate -o ../../mocks/publisher-task-identifier-creator.go --fake-name PublisherTaskIdentifierCreator . TaskIdentifierCreator

// TaskIdentifierCreator derives the deterministic UUID5 identifier AND
// the period-anchored title-suffix token for a recurring task in one
// call. The publisher needs both — the identifier is the dedup key on
// the downstream controller, the period token is the title suffix —
// and they share the same period-token computation, so returning them
// together avoids re-deriving the token at the call site.
//
// The identifier is UUID5(uuidNamespace, "recurring-<slug>-<token>"),
// where <token> is the result of PeriodTokenBuilder.Build for the
// same (def, date). Same input on a second call produces the same
// identifier across processes, redeploys, and replays — this is the
// contract the controller's de-dup relies on.
type TaskIdentifierCreator interface {
	// Create returns the (identifier, periodToken) pair for (def, date).
	// def.PeriodOffset shifts the period token for the period-anchored
	// recurrence kinds (Monthly / Quarterly / Yearly); the shift feeds
	// into the UUID5 input, so different offsets produce different
	// identifiers for the same fire date.
	Create(
		ctx context.Context,
		def schedule.TaskDefinition,
		date schedule.Date,
	) (lib.TaskIdentifier, PeriodToken, error)
}

// NewTaskIdentifierCreator returns the default TaskIdentifierCreator
// backed by the given PeriodTokenBuilder. The builder is the only
// dependency — the UUID5 namespace is a package constant.
func NewTaskIdentifierCreator(builder PeriodTokenBuilder) TaskIdentifierCreator {
	return &taskIdentifierCreator{builder: builder}
}

type taskIdentifierCreator struct {
	builder PeriodTokenBuilder
}

func (c *taskIdentifierCreator) Create(
	ctx context.Context,
	def schedule.TaskDefinition,
	date schedule.Date,
) (lib.TaskIdentifier, PeriodToken, error) {
	token, err := c.builder.Build(ctx, def, date)
	if err != nil {
		return "", "", errors.Wrapf(
			ctx,
			err,
			"TaskIdentifierCreator.Create: slug %q",
			def.Slug,
		)
	}
	name := "recurring-" + def.Slug + "-" + string(token)
	return lib.TaskIdentifier(uuid.NewSHA1(uuidNamespace, []byte(name)).String()), token, nil
}
