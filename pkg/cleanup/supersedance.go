// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cleanup

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bborbe/errors"
	libtime "github.com/bborbe/time"
	"github.com/golang/glog"
	"gopkg.in/yaml.v3"

	"github.com/bborbe/recurring-task-creator/pkg/publisher"
	"github.com/bborbe/recurring-task-creator/pkg/schedule"
	"github.com/bborbe/recurring-task-creator/pkg/store"
)

// Supersedance auto-aborts prior-period recurring-task instances left
// status: in_progress once the next period's instance has materialized.
type Supersedance struct {
	Store        store.ScheduleStore
	TokenBuilder publisher.PeriodTokenBuilder
	Reader       VaultReader
	Writer       VaultWriter
	Metrics      Metrics
	Clock        libtime.CurrentDateTimeGetter
}

// Run iterates every Schedule for the given Berlin civil date, skips those
// with SkipAutoCleanup == true, computes each Schedule's prior-period
// token, and supersedes the matching prior in-progress file when the
// next-period instance already exists. A per-Schedule failure is logged
// and counted (result="error"); it never aborts the whole tick.
func (s *Supersedance) Run(ctx context.Context, date schedule.Date) error {
	defs, err := s.Store.List(ctx)
	if err != nil {
		return errors.Wrap(ctx, err, "list schedules")
	}

	for _, def := range defs {
		select {
		case <-ctx.Done():
			return errors.Wrap(ctx, ctx.Err(), "cleanup: context cancelled")
		default:
		}

		s.processDef(ctx, def, date)
	}
	return nil
}

func (s *Supersedance) processDef(
	ctx context.Context,
	def schedule.TaskDefinition,
	date schedule.Date,
) {
	if def.SkipAutoCleanup {
		glog.V(3).Infof("cleanup: skipping %s: skipAutoCleanup=true", def.Slug)
		return
	}

	priorToken, err := PriorPeriodToken(ctx, s.TokenBuilder, def, date)
	if err != nil {
		glog.Errorf("cleanup: priorToken for %q failed: %v", def.Slug, err)
		s.Metrics.IncSuperseded("error", string(def.Recurrence))
		return
	}

	currentToken, err := s.TokenBuilder.Build(ctx, def, date)
	if err != nil {
		glog.Errorf("cleanup: currentToken for %q failed: %v", def.Slug, err)
		s.Metrics.IncSuperseded("error", string(def.Recurrence))
		return
	}

	slugStem := def.Slug // title stem
	priorPath := fmt.Sprintf("%s - %s.md", slugStem, priorToken)
	currentPath := fmt.Sprintf("%s - %s.md", slugStem, currentToken)

	// List files with the slug stem prefix to detect prior and current files.
	files, err := s.Reader.ListFiles(ctx, slugStem)
	if err != nil {
		glog.Errorf("cleanup: ListFiles for %q failed: %v", def.Slug, err)
		s.Metrics.IncSuperseded("error", string(def.Recurrence))
		return
	}

	priorExists := pathExists(files, priorPath)
	currentExists := pathExists(files, currentPath)

	if !priorExists {
		glog.V(3).Infof("cleanup: prior file %s does not exist, skipping", priorPath)
		return
	}

	if !shouldSupersedePrior(ctx, s.Reader, priorPath, currentExists, date, def.Recurrence) {
		return
	}

	s.applySupersede(ctx, priorPath, def.Recurrence)
}

// shouldSupersedePrior returns true when the prior file should be superseded.
// When the next-period file does not yet exist, fetches the prior file and
// checks whether its planned_date falls within the current firing window —
// in that case, skip (don't abort the current period).
func shouldSupersedePrior(
	ctx context.Context,
	reader VaultReader,
	priorPath string,
	currentExists bool,
	date schedule.Date,
	kind schedule.RecurrenceKind,
) bool {
	if currentExists {
		return true
	}
	content, err := reader.GetFile(ctx, priorPath)
	if err != nil {
		// Caller logs + counts error; treat as "do not supersede" rather than
		// escalate: failing to read shouldn't blow up the tick.
		return false
	}
	fm, _ := parseFrontmatter(ctx, content)
	if withinCadenceFM(fm, date, kind) {
		glog.V(3).Infof("cleanup: prior %s within firing window, not superseding", priorPath)
		return false
	}
	return true
}

// applySupersede fetches the prior file, confirms it is still in_progress,
// then writes the supersede transition via VaultWriter. Counts outcomes
// via Metrics; never returns an error (failures are logged + counted).
func (s *Supersedance) applySupersede(
	ctx context.Context,
	priorPath string,
	recurrence schedule.RecurrenceKind,
) {
	content, err := s.Reader.GetFile(ctx, priorPath)
	if err != nil {
		glog.Errorf("cleanup: GetFile for prior %s failed: %v", priorPath, err)
		s.Metrics.IncSuperseded("error", string(recurrence))
		return
	}

	fm, err := parseFrontmatter(ctx, content)
	if err != nil {
		glog.Errorf("cleanup: parse frontmatter for %s failed: %v", priorPath, err)
		s.Metrics.IncSuperseded("error", string(recurrence))
		return
	}

	status, _ := fm["status"].(string)
	if status != "in_progress" {
		glog.V(3).Infof("cleanup: prior %s status=%q, skipping idempotent", priorPath, status)
		return
	}

	ts := s.Clock.Now().Time()
	mutator := func(current []byte) ([]byte, error) {
		return mutateFrontmatterSupersede(ctx, current, ts)
	}

	if err := s.Writer.UpdateFile(ctx, priorPath, mutator); err != nil {
		if IsVaultConflict(err) {
			glog.Warningf("cleanup: git-rest conflict for %s, will retry next tick", priorPath)
			s.Metrics.IncSuperseded("conflict", string(recurrence))
			return
		}
		glog.Errorf("cleanup: UpdateFile for %s failed: %v", priorPath, err)
		s.Metrics.IncSuperseded("error", string(recurrence))
		return
	}

	glog.V(2).Infof("cleanup: superseded %s", priorPath)
	s.Metrics.IncSuperseded("success", string(recurrence))
}

// pathExists reports whether path is present in the list.
func pathExists(paths []string, path string) bool {
	for _, p := range paths {
		if p == path {
			return true
		}
	}
	return false
}

// cadenceDays returns the approximate number of days in one cadence for the recurrence kind.
func cadenceDays(kind schedule.RecurrenceKind) int {
	switch kind {
	case schedule.RecurrenceDaily:
		return 1
	case schedule.RecurrenceWeekly, schedule.RecurrenceWeekday:
		return 7
	case schedule.RecurrenceMonthly:
		return 31
	case schedule.RecurrenceQuarterly:
		return 93
	case schedule.RecurrenceYearly:
		return 366
	default:
		return 0
	}
}

// withinCadence reports whether the planned_date (as a time.Time) is within
// one cadence of the current date.
func withinCadence(
	planned time.Time,
	currentDate schedule.Date,
	kind schedule.RecurrenceKind,
) bool {
	t := currentDate.Time()
	cadence := cadenceDays(kind)
	if cadence == 0 {
		return false
	}
	diff := t.Sub(planned).Hours() / 24
	return diff >= 0 && diff <= float64(cadence)
}

// withinCadenceFM reports whether the frontmatter's planned_date field
// represents a date within one cadence of currentDate.
func withinCadenceFM(
	fm map[string]interface{},
	currentDate schedule.Date,
	kind schedule.RecurrenceKind,
) bool {
	var planned time.Time
	switch v := fm["planned_date"].(type) {
	case string:
		var err error
		planned, err = time.Parse("2006-01-02", v)
		if err != nil {
			return false
		}
	case time.Time:
		planned = v
	default:
		return false
	}
	return withinCadence(planned, currentDate, kind)
}

// parseFrontmatter extracts the YAML frontmatter from markdown content.
// Returns an empty map if parsing fails.
func parseFrontmatter(ctx context.Context, content []byte) (map[string]interface{}, error) {
	fmStr, _ := splitFrontmatter(string(content))
	if fmStr == "" {
		return map[string]interface{}{}, nil
	}
	var fm map[string]interface{}
	if err := yaml.Unmarshal([]byte(fmStr), &fm); err != nil {
		return nil, errors.Wrap(ctx, err, "parse frontmatter yaml")
	}
	if fm == nil {
		return map[string]interface{}{}, nil
	}
	return fm, nil
}

// splitFrontmatter separates "---\n<yaml>\n---\n<body>" into (yaml, body).
// Returns ("", content) when no frontmatter is present.
func splitFrontmatter(content string) (string, string) {
	if !strings.HasPrefix(content, "---\n") {
		return "", content
	}
	rest := content[4:]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return "", content
	}
	return rest[:end], rest[end+5:]
}

// mutateFrontmatterSupersede updates the frontmatter of the markdown content:
// - Sets status: aborted
// - Sets phase: done
// - Sets completed_date to now (ISO date)
// - Appends superseded_by: auto-cleanup-<unix-ts>
// - Re-stamps created_by: recurring-task-creator (preserves the publisher invariant)
func mutateFrontmatterSupersede(ctx context.Context, content []byte, ts time.Time) ([]byte, error) {
	fmStr, body := splitFrontmatter(string(content))
	if fmStr == "" {
		// No frontmatter; inject one at the top.
		// This shouldn't happen in practice but handle gracefully.
		return nil, errors.Errorf(ctx, "mutateFrontmatterSupersede: no frontmatter found")
	}

	var fm map[string]interface{}
	if err := yaml.Unmarshal([]byte(fmStr), &fm); err != nil {
		return nil, errors.Wrap(ctx, err, "unmarshal frontmatter for supersede")
	}
	if fm == nil {
		fm = map[string]interface{}{}
	}

	fm["status"] = "aborted"
	fm["phase"] = "done"
	fm["completed_date"] = ts.UTC().Format("2006-01-02")
	fm["superseded_by"] = fmt.Sprintf("auto-cleanup-%d", ts.Unix())
	fm["created_by"] = "recurring-task-creator"

	newFmBytes, err := yaml.Marshal(fm)
	if err != nil {
		return nil, errors.Wrap(ctx, err, "marshal supersede frontmatter")
	}

	newContent := fmt.Sprintf("---\n%s---\n%s", string(newFmBytes), body)
	return []byte(newContent), nil
}
