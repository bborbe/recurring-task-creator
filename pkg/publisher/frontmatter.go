// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package publisher

import (
	lib "github.com/bborbe/agent/lib"
)

// buildFrontmatter returns the frontmatter stamped onto every published
// task. Three published-author defaults seed the output (`status:
// in_progress`, `page_type: task`); operator-supplied keys from
// `Schedule.spec.template.frontmatter` are then merged on top and may
// override those defaults. Finally `created_by: recurring-task-creator`
// is force-set so a Schedule CR cannot impersonate a different author
// of the published vault file — `created_by` is provenance, not
// configuration. Every other key is operator-configurable.
func buildFrontmatter(operatorFrontmatter lib.TaskFrontmatter) lib.TaskFrontmatter {
	out := lib.TaskFrontmatter{
		"status":    "in_progress",
		"page_type": "task",
	}
	for k, v := range operatorFrontmatter {
		out[k] = v
	}
	out["created_by"] = "recurring-task-creator"
	return out
}
