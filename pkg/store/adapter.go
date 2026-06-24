// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"strings"
	"time"

	"github.com/bborbe/errors"

	v1 "github.com/bborbe/recurring-task-creator/k8s/apis/task.benjamin-borbe.de/v1"
	"github.com/bborbe/recurring-task-creator/pkg/schedule"
)

var weekdayByName = map[string]time.Weekday{
	"Sunday": time.Sunday, "Monday": time.Monday, "Tuesday": time.Tuesday,
	"Wednesday": time.Wednesday, "Thursday": time.Thursday, "Friday": time.Friday,
	"Saturday": time.Saturday,
	"Sun":      time.Sunday, "Mon": time.Monday, "Tue": time.Tuesday,
	"Wed": time.Wednesday, "Thu": time.Thursday, "Fri": time.Friday,
	"Sat": time.Saturday,
}

func adaptSchedule(ctx context.Context, cr *v1.Schedule) (schedule.TaskDefinition, error) {
	kind := schedule.RecurrenceKind(strings.ToLower(cr.Spec.Schedule.Recurrence))
	valid := false
	for _, k := range schedule.AllRecurrenceKinds {
		if k == kind {
			valid = true
			break
		}
	}
	if !valid {
		return schedule.TaskDefinition{}, errors.Errorf(
			ctx,
			"unknown recurrence %q",
			cr.Spec.Schedule.Recurrence,
		)
	}

	var weekdays []time.Weekday
	seen := map[time.Weekday]bool{}
	for _, name := range cr.Spec.Schedule.Weekday {
		wd, ok := weekdayByName[name]
		if !ok {
			return schedule.TaskDefinition{}, errors.Errorf(
				ctx,
				"unknown weekday %q in schedule %q",
				name, cr.Name,
			)
		}
		if seen[wd] {
			continue
		}
		seen[wd] = true
		weekdays = append(weekdays, wd)
	}

	return schedule.TaskDefinition{
		Slug:          cr.Name,
		TitleTemplate: cr.Spec.Title,
		BodyTemplate:  cr.Spec.Template.Body,
		Recurrence:    kind,
		Weekdays:      weekdays,
		Frontmatter:   cr.Spec.Template.Frontmatter,
		PeriodOffset:  cr.Spec.Schedule.PeriodOffset,
	}, nil
}
