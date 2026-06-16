// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package publisher

import (
	lib "github.com/bborbe/agent/lib"
)

// buildFrontmatter returns the exact frontmatter shape for every task
// published by this service. The shape is FROZEN: changing any of these
// keys or values is a breaking change to the migration's vault-file
// layout. The shape was reduced from seven keys to six by spec 008:
// the `recurring` key is gone — downstream vault tooling treats every
// published task as a normal one-shot task, regardless of cadence.
func buildFrontmatter() lib.TaskFrontmatter {
	return lib.TaskFrontmatter{
		"assignee":   "bborbe",
		"status":     "in_progress",
		"page_type":  "task",
		"goals":      []interface{}{goalsLink},
		"priority":   2,
		"created_by": "recurring-task-creator",
	}
}
