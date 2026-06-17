// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package store

import (
	"context"

	"github.com/bborbe/errors"
	"github.com/golang/glog"
	labels "k8s.io/apimachinery/pkg/labels"

	listersv1 "github.com/bborbe/recurring-task-creator/k8s/client/listers/task.benjamin-borbe.de/v1"
	"github.com/bborbe/recurring-task-creator/pkg/schedule"
)

//counterfeiter:generate -o ../../mocks/store-store.go --fake-name FakeScheduleStore . ScheduleStore

// ScheduleStore returns the current recurring-task inventory, read from
// the informer cache over the Schedule CRD.
type ScheduleStore interface {
	// List returns every Schedule CR in the watched namespace, adapted
	// to []schedule.TaskDefinition. CRs whose Recurrence value is not a
	// known RecurrenceKind are logged and dropped (never abort the read).
	// A lister error is wrapped and returned.
	List(ctx context.Context) ([]schedule.TaskDefinition, error)
}

// NewScheduleStore builds a ScheduleStore backed by an informer lister.
func NewScheduleStore(lister listersv1.ScheduleLister, namespace string) ScheduleStore {
	return &scheduleStore{lister: lister, namespace: namespace}
}

type scheduleStore struct {
	lister    listersv1.ScheduleLister
	namespace string
}

func (s *scheduleStore) List(ctx context.Context) ([]schedule.TaskDefinition, error) {
	crs, err := s.lister.Schedules(s.namespace).List(labels.Everything())
	if err != nil {
		return nil, errors.Wrap(ctx, err, "list schedules from informer cache")
	}
	out := make([]schedule.TaskDefinition, 0, len(crs))
	for _, cr := range crs {
		def, err := adaptSchedule(ctx, cr)
		if err != nil {
			glog.Warningf("store: dropping schedule %q: %v", cr.Name, err)
			continue
		}
		out = append(out, def)
	}
	return out, nil
}
