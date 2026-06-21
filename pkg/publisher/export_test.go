// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package publisher

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/bborbe/recurring-task-creator/pkg/schedule"
)

// UuidNamespaceForTest exposes the frozen UUID5 namespace to external
// tests so they can compute the expected identifier offline.
func UuidNamespaceForTest() uuid.UUID { return uuidNamespace }

// BuildPeriodTokenForTest exposes buildPeriodToken to external tests so
// they can compute the expected title suffix for a (def, date) pair
// without re-implementing the period-token derivation. The test asserts
// the publisher's rendered title ends with " - " + the result of this
// function for the same input — guaranteeing the render and the
// identifier pipeline use the same period token.
func BuildPeriodTokenForTest(
	ctx context.Context,
	recurrence schedule.RecurrenceKind,
	date schedule.Date,
	weekday time.Weekday,
	periodOffset int,
) (string, error) {
	return buildPeriodToken(ctx, recurrence, date, weekday, periodOffset)
}
