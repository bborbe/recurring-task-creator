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
	"Sunday":    time.Sunday,
	"Monday":    time.Monday,
	"Tuesday":   time.Tuesday,
	"Wednesday": time.Wednesday,
	"Thursday":  time.Thursday,
	"Friday":    time.Friday,
	"Saturday":  time.Saturday,
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

	var weekday time.Weekday
	if cr.Spec.Schedule.Weekday != "" {
		wd, ok := weekdayByName[cr.Spec.Schedule.Weekday]
		if !ok {
			return schedule.TaskDefinition{}, errors.Errorf(
				ctx,
				"unknown weekday %q",
				cr.Spec.Schedule.Weekday,
			)
		}
		weekday = wd
	}

	return schedule.TaskDefinition{
		Slug:          cr.Name,
		TitleTemplate: cr.Spec.Title,
		BodyTemplate:  cr.Spec.Template.Body,
		Recurrence:    kind,
		Weekday:       weekday,
	}, nil
}
