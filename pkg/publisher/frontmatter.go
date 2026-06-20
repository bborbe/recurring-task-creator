// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package publisher

import (
	lib "github.com/bborbe/agent/lib"

	"github.com/bborbe/recurring-task-creator/pkg/schedule"
)

// buildFrontmatter returns the frontmatter stamped onto every published
// task. Three published-author defaults seed the output (`status:
// in_progress`, `page_type: task`); operator-supplied keys from
// `Schedule.spec.template.frontmatter` are then merged on top and may
// override those defaults. Finally `created_by: recurring-task-creator`
// is force-set so a Schedule CR cannot impersonate a different author
// of the published vault file — `created_by` is provenance, not
// configuration. Every other key is operator-configurable.
//
// String values in operatorFrontmatter are passed through renderTemplate
// using the same closed placeholder set as title/body, so a Schedule CR
// can stamp dynamic fields such as `planned_date: "{{date}}"`. Non-string
// values (ints, slices, maps) pass through unchanged — placeholder
// substitution is a string-level transform.
func buildFrontmatter(
	operatorFrontmatter lib.TaskFrontmatter,
	slug string,
	date schedule.Date,
) lib.TaskFrontmatter {
	out := lib.TaskFrontmatter{
		"status":    "in_progress",
		"page_type": "task",
	}
	for k, v := range operatorFrontmatter {
		if s, ok := v.(string); ok {
			out[k] = renderTemplate(s, slug, date)
			continue
		}
		out[k] = v
	}
	out["created_by"] = "recurring-task-creator"
	return out
}
