// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package publisher

import (
	lib "github.com/bborbe/agent/lib"

	"github.com/bborbe/recurring-task-creator/pkg/schedule"
)

//counterfeiter:generate -o ../../mocks/publisher-frontmatter-formatter.go --fake-name PublisherFrontmatterFormatter . FrontmatterFormatter

// FrontmatterFormatter builds the YAML frontmatter stamped onto every
// published task. The formatter is the single seam between operator-
// supplied frontmatter (from `Schedule.spec.template.frontmatter`) and
// the wire-level `task.CreateCommand.Frontmatter` payload: it seeds
// published-author defaults, renders placeholder tokens in string
// values via the publisher's closed placeholder set, merges operator
// keys (which may override the defaults), and force-sets the provenance
// key `created_by: recurring-task-creator` last so a Schedule CR cannot
// impersonate a different author.
type FrontmatterFormatter interface {
	// Format returns the frontmatter for one published task. String
	// values in `operator` are rendered through the same placeholder
	// substitution as title/body (`{{date}}`, `{{iso-week}}`, etc.);
	// non-string values (ints, slices, maps) pass through unchanged.
	// `slug` and `date` parameterize the placeholder render; `date` is
	// the Berlin civil date the task fires for (the publisher converts
	// wall-clock once at the tick boundary).
	Format(operator lib.TaskFrontmatter, slug string, date schedule.Date) lib.TaskFrontmatter
}

// NewFrontmatterFormatter returns the default FrontmatterFormatter that
// renders placeholders in string values via the publisher's closed
// placeholder set (see render.go). Stateless: safe to construct once
// and share across goroutines.
func NewFrontmatterFormatter() FrontmatterFormatter {
	return &frontmatterFormatter{}
}

type frontmatterFormatter struct{}

func (f *frontmatterFormatter) Format(
	operator lib.TaskFrontmatter,
	slug string,
	date schedule.Date,
) lib.TaskFrontmatter {
	out := lib.TaskFrontmatter{
		"status":    "in_progress",
		"page_type": "task",
	}
	for k, v := range operator {
		if s, ok := v.(string); ok {
			out[k] = renderTemplate(s, slug, date)
			continue
		}
		out[k] = v
	}
	out["created_by"] = "recurring-task-creator"
	return out
}
