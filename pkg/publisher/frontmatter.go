// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package publisher

import (
	lib "github.com/bborbe/agent/lib"

	"github.com/bborbe/recurring-task-creator/pkg/schedule"
)

// buildFrontmatter returns the exact frontmatter shape for every recurring
// task. The shape is FROZEN: changing any of these keys or values is a
// breaking change to the migration's vault-file layout.
func buildFrontmatter(recurrence schedule.RecurrenceKind) lib.TaskFrontmatter {
	return lib.TaskFrontmatter{
		"assignee":  "bborbe",
		"status":    "in_progress",
		"page_type": "task",
		"goals":     []interface{}{goalsLink},
		"priority":  2,
		"recurring": string(recurrence),
	}
}
