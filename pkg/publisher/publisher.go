// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package publisher

import (
	"context"
	"strings"

	"github.com/bborbe/agent/lib/command/task"
	"github.com/bborbe/errors"
	"github.com/golang/glog"

	"github.com/bborbe/recurring-task-creator/pkg/schedule"
)

//counterfeiter:generate -o ../../mocks/publisher-publisher.go --fake-name PublisherPublisher . Publisher

// Publisher turns one (TaskDefinition, Date) pair into a validated
// task.CreateCommand and sends it via the injected task.CreateCommandSender.
type Publisher interface {
	// Publish builds a CreateCommand for (def, date) and sends it. The
	// returned error is wrapped with the slug and ISO date in its message.
	// Same (def, date) on a second call produces a byte-identical command.
	Publish(ctx context.Context, def schedule.TaskDefinition, date schedule.Date) error
}

// NewPublisher returns a Publisher that sends through sender, formatting
// frontmatter via formatter. The sender is invoked exactly once per
// Publish call (when inputs are valid). It validates the constructed
// command internally — see task.CreateCommandSender.SendCommand in
// github.com/bborbe/agent/lib/command/task. When dryRun is true, the
// publisher logs the would-be CreateCommand and skips the sender call
// (intended for local smoke-testing via cmd/run-once).
func NewPublisher(
	sender task.CreateCommandSender,
	formatter FrontmatterFormatter,
	dryRun bool,
) Publisher {
	return &publisher{sender: sender, formatter: formatter, dryRun: dryRun}
}

type publisher struct {
	sender    task.CreateCommandSender
	formatter FrontmatterFormatter
	dryRun    bool
}

func (p *publisher) Publish(
	ctx context.Context,
	def schedule.TaskDefinition,
	date schedule.Date,
) error {
	if def.Slug == "" {
		return errors.Errorf(ctx, "publish failed: empty slug")
	}
	if date.IsZero() {
		return errors.Errorf(ctx, "publish failed: zero date for slug %q", def.Slug)
	}
	token, err := buildTaskIdentifier(ctx, def.Slug, def.Recurrence, date, def.Weekday)
	if err != nil {
		return errors.Wrapf(
			ctx,
			err,
			"publish failed: build identifier for slug %q",
			def.Slug,
		)
	}
	periodToken, err := buildPeriodToken(ctx, def.Recurrence, date, def.Weekday)
	if err != nil {
		return errors.Wrapf(
			ctx,
			err,
			"publish failed: build period token for slug %q",
			def.Slug,
		)
	}
	cmd := task.CreateCommand{
		TaskIdentifier: token,
		Title: strings.TrimSpace(
			renderTemplate(def.TitleTemplate, def.Slug, date),
		) + " - " + periodToken,
		Frontmatter: p.formatter.Format(def.Frontmatter, def.Slug, date),
		Body:        renderTemplate(def.BodyTemplate, def.Slug, date),
	}
	if p.dryRun {
		glog.V(0).Infof("publisher: DRY_RUN — would send slug=%q date=%04d-%02d-%02d identifier=%s",
			def.Slug, date.Year, date.Month, date.Day, cmd.TaskIdentifier)
		return nil
	}
	if err := p.sender.SendCommand(ctx, cmd); err != nil {
		return errors.Wrapf(
			ctx,
			err,
			"publish failed: send CreateCommand for slug %q on %04d-%02d-%02d",
			def.Slug, date.Year, date.Month, date.Day,
		)
	}
	glog.V(2).
		Infof("publisher: sent CreateCommand slug=%q date=%04d-%02d-%02d", def.Slug, date.Year, date.Month, date.Day)
	return nil
}
