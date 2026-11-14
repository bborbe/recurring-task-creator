// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package publisher

import (
	"context"

	lib "github.com/bborbe/agent"
	"github.com/bborbe/errors"
	"github.com/bborbe/validation"
)

// ModeRoleRule validates the supervisor-consumed mode/role pair in an
// operator-supplied frontmatter map. The supervisor's role-mode matrix
// forces `role: human` and `role: manager` to interactive and every other
// role — including an absent role, which defaults to `agent` — to
// headless. Therefore `mode: interactive` paired with any role outside
// {human, manager} is self-contradictory and must be refused rather than
// silently resolved to headless.
type ModeRoleRule struct {
	// Slug is the schedule slug named verbatim in a refusal message.
	Slug string
	// Frontmatter is the operator-supplied frontmatter map, read exactly
	// as declared: before defaults, merging, or placeholder rendering.
	Frontmatter lib.TaskFrontmatter
}

// Validate implements validation.HasValidation. It is a pure in-memory
// check with no I/O, clock, retry, or goroutine use.
func (r ModeRoleRule) Validate(ctx context.Context) error {
	return validation.All{
		validation.Name("mode/role pair", validation.HasValidationFunc(r.validateModeRole)),
	}.Validate(ctx)
}

// validateModeRole refuses `mode: interactive` unless `role` is the literal
// string `human` or `manager`. Every other shape passes: an absent or
// non-interactive mode, a non-string mode, and a non-string role impose no
// constraint. Strings are compared literally, never rendered, so a
// placeholder-looking role cannot slip past the guard.
func (r ModeRoleRule) validateModeRole(ctx context.Context) error {
	mode, ok := r.Frontmatter["mode"].(string)
	if !ok || mode != "interactive" {
		return nil
	}
	role, present := r.Frontmatter["role"]
	if !present {
		return errors.Errorf(
			ctx,
			"schedule %q declares mode %q with role absent; interactive requires role human or manager",
			r.Slug,
			mode,
		)
	}
	roleString, ok := role.(string)
	if !ok {
		return nil
	}
	if roleString == "human" || roleString == "manager" {
		return nil
	}
	return errors.Errorf(
		ctx,
		"schedule %q declares mode %q with role %q; interactive requires role human or manager",
		r.Slug,
		mode,
		roleString,
	)
}
