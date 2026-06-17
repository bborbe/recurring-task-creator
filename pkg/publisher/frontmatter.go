// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package publisher

import (
	lib "github.com/bborbe/agent/lib"
)

// buildFrontmatter returns the frontmatter stamped onto every published
// task. The three built-in keys (status, page_type, created_by) are
// always emitted with the values below — they encode the publisher's
// invariants and are not operator-configurable. Operator-defined keys
// (assignee, goals, priority, category, etc.) come from the Schedule
// CR's `spec.template.frontmatter` field and are merged in via
// operatorFrontmatter. When a built-in key collides with an operator
// key the built-in wins — the publisher's contract about its own
// emissions is not overrideable by configuration.
func buildFrontmatter(operatorFrontmatter lib.TaskFrontmatter) lib.TaskFrontmatter {
	out := lib.TaskFrontmatter{}
	for k, v := range operatorFrontmatter {
		out[k] = v
	}
	out["status"] = "in_progress"
	out["page_type"] = "task"
	out["created_by"] = "recurring-task-creator"
	return out
}
