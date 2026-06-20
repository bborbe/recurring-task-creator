// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package publisher

import (
	"strings"

	"github.com/bborbe/recurring-task-creator/pkg/schedule"
)

//counterfeiter:generate -o ../../mocks/publisher-renderer.go --fake-name PublisherRenderer . Renderer

// Renderer substitutes the closed set of `{{...}}` placeholder tokens
// (see placeholders.go) inside an operator-authored template string.
// Same seam used by the publisher for title and body rendering and by
// FrontmatterFormatter for string-valued frontmatter entries — single
// definition of "what a placeholder is" across every render site.
type Renderer interface {
	// Render returns template with every placeholder substituted by
	// its rendered value for date. The slug parameter is reserved for
	// future placeholders that depend on the slug itself; the current
	// set does not. Pure: no I/O, no time, no state — same inputs on
	// a second call produce a byte-identical result.
	Render(template, slug string, date schedule.Date) string
}

// NewRenderer returns the default Renderer backed by the package-level
// placeholders table. Stateless: safe to construct once and share
// across goroutines.
func NewRenderer() Renderer {
	return &renderer{}
}

type renderer struct{}

func (r *renderer) Render(template, slug string, date schedule.Date) string {
	_ = slug
	out := template
	for _, p := range placeholders {
		out = strings.ReplaceAll(out, p.name, p.compute(date))
	}
	return out
}
