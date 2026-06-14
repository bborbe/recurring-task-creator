// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package schedule

// ScheduleLookup is a function over the schedule domain that returns
// the task definitions firing on a given civil date. Satisfied by
// TasksForDate; declared here so consumers (pkg/tick, pkg/handler) share
// one canonical type instead of duplicating it structurally.
type ScheduleLookup func(date Date) []TaskDefinition
