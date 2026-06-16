// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package store

import (
	"context"

	v1 "github.com/bborbe/recurring-task-creator/k8s/apis/task.benjamin-borbe.de/v1"
	"github.com/bborbe/recurring-task-creator/pkg/schedule"
)

// AdaptScheduleForTest exposes the unexported adapter to external tests.
func AdaptScheduleForTest(ctx context.Context, cr *v1.Schedule) (schedule.TaskDefinition, error) {
	return adaptSchedule(ctx, cr)
}
