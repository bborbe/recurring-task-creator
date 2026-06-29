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

		if err := s.processDef(ctx, def, date); err != nil {
			glog.Errorf("cleanup: process def %q failed: %v", def.Slug, err)
		}
	}
	return nil
}

func (s *Supersedance) processDef(ctx context.Context, def schedule.TaskDefinition, date schedule.Date) error {
	if def.SkipAutoCleanup {
		glog.V(3).Infof("cleanup: skipping %s: skipAutoCleanup=true", def.Slug)
		return nil
	}

	priorToken, err := PriorPeriodToken(ctx, s.TokenBuilder, def, date)
	if err != nil {
		glog.Errorf("cleanup: priorToken for %q failed: %v", def.Slug, err)
		s.Metrics.IncSuperseded("error", string(def.Recurrence))
		return nil
	}

	currentToken, err := s.TokenBuilder.Build(ctx, def, date)
	if err != nil {
		glog.Errorf("cleanup: currentToken for %q failed: %v", def.Slug, err)
		s.Metrics.IncSuperseded("error", string(def.Recurrence))
		return nil
	}

	slugStem := def.Slug // title stem
	priorPath := fmt.Sprintf("%s - %s.md", slugStem, priorToken)
	currentPath := fmt.Sprintf("%s - %s.md", slugStem, currentToken)

	// List files with the slug stem prefix to detect prior and current files.
	files, err := s.Reader.ListFiles(ctx, slugStem)
	if err != nil {
		glog.Errorf("cleanup: ListFiles for %q failed: %v", def.Slug, err)
		s.Metrics.IncSuperseded("error", string(def.Recurrence))
		return nil
	}

	priorExists := pathExists(files, priorPath)
	currentExists := pathExists(files, currentPath)

	if !priorExists {
		glog.V(3).Infof("cleanup: prior file %s does not exist, skipping", priorPath)
		return nil
	}

	// Safety gate: if the next-period file does NOT exist AND the prior file's
	// planned_date is within one cadence of date, skip.
	if !currentExists {
		content, err := s.Reader.GetFile(ctx, priorPath)
		if err != nil {
			glog.Errorf("cleanup: GetFile for prior %s failed: %v", priorPath, err)
			s.Metrics.IncSuperseded("error", string(def.Recurrence))
			return nil
		}
		fm, _ := parseFrontmatter(content)
		if withinCadenceFM(fm, date, def.Recurrence) {
			glog.V(3).Infof("cleanup: prior %s within firing window, not superseding", priorPath)
			return nil
		}
	}

	content, err := s.Reader.GetFile(ctx, priorPath)
	if err != nil {
		glog.Errorf("cleanup: GetFile for prior %s failed: %v", priorPath, err)
		s.Metrics.IncSuperseded("error", string(def.Recurrence))
		return nil
	}

	fm, err := parseFrontmatter(content)
	if err != nil {
		glog.Errorf("cleanup: parse frontmatter for %s failed: %v", priorPath, err)
		s.Metrics.IncSuperseded("error", string(def.Recurrence))
		return nil
	}

	status, _ := fm["status"].(string)
	if status != "in_progress" {
		glog.V(3).Infof("cleanup: prior %s status=%q, skipping idempotent", priorPath, status)
		return nil
	}

	ts := s.Clock.Now().Time().Unix()
	mutator := func(current []byte) ([]byte, error) {
		return mutateFrontmatterSupersede(current, ts)
	}

	if err := s.Writer.UpdateFile(ctx, priorPath, mutator); err != nil {
		if IsVaultConflict(err) {
			glog.Warningf("cleanup: git-rest conflict for %s, will retry next tick", priorPath)
			s.Metrics.IncSuperseded("conflict", string(def.Recurrence))
			return nil
		}
		glog.Errorf("cleanup: UpdateFile for %s failed: %v", priorPath, err)
		s.Metrics.IncSuperseded("error", string(def.Recurrence))
		return nil
	}

	glog.V(2).Infof("cleanup: superseded %s", priorPath)
	s.Metrics.IncSuperseded("success", string(def.Recurrence))
	return nil
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
func withinCadence(planned time.Time, currentDate schedule.Date, kind schedule.RecurrenceKind) bool {
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
func withinCadenceFM(fm map[string]interface{}, currentDate schedule.Date, kind schedule.RecurrenceKind) bool {
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
func parseFrontmatter(content []byte) (map[string]interface{}, error) {
	fmStr, _ := splitFrontmatter(string(content))
	if fmStr == "" {
		return map[string]interface{}{}, nil
	}
	var fm map[string]interface{}
	if err := yaml.Unmarshal([]byte(fmStr), &fm); err != nil {
		return nil, errors.Wrap(context.Background(), err, "parse frontmatter yaml")
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
// - Sets completed_date marker
// - Appends superseded_by: auto-cleanup-<unix-ts>
func mutateFrontmatterSupersede(content []byte, unixTs int64) ([]byte, error) {
	fmStr, body := splitFrontmatter(string(content))
	if fmStr == "" {
		// No frontmatter; inject one at the top.
		// This shouldn't happen in practice but handle gracefully.
		return nil, errors.Errorf(context.Background(), "mutateFrontmatterSupersede: no frontmatter found")
	}

	var fm map[string]interface{}
	if err := yaml.Unmarshal([]byte(fmStr), &fm); err != nil {
		return nil, errors.Wrap(context.Background(), err, "unmarshal frontmatter for supersede")
	}
	if fm == nil {
		fm = map[string]interface{}{}
	}

	fm["status"] = "aborted"
	fm["phase"] = "done"
	fm["completed_date"] = fmt.Sprintf("auto-cleanup-%d", unixTs)
	fm["superseded_by"] = fmt.Sprintf("auto-cleanup-%d", unixTs)

	newFmBytes, err := yaml.Marshal(fm)
	if err != nil {
		return nil, errors.Wrap(context.Background(), err, "marshal supersede frontmatter")
	}

	newContent := fmt.Sprintf("---\n%s---\n%s", string(newFmBytes), body)
	return []byte(newContent), nil
}
