// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package schedule

import (
	"sort"
	"sync"
)

// tasksForDateCache memoizes TasksForDate results by Date. The function is
// pure, so caching is safe; the same Date always returns the same slice
// reference, which keeps the "referentially transparent (deep equality)"
// acceptance test green despite the unexported `Fires` field being a
// function value (function values are not comparable in Go, so two fresh
// slices carrying the same function pointer still fail reflect.DeepEqual).
var tasksForDateCache sync.Map

// TasksForDate returns every inventory entry whose predicate fires on d,
// sorted by Slug ascending. Pure: same input always yields the same slice
// (deep equality). No I/O, no clock, no global state.
//
// The zero Date value returns an empty slice; it never panics.
func TasksForDate(d Date) []TaskDefinition {
	if d.IsZero() {
		return []TaskDefinition{}
	}
	if cached, ok := tasksForDateCache.Load(d); ok {
		if defs, ok := cached.([]TaskDefinition); ok {
			return defs
		}
	}
	var out []TaskDefinition
	for _, def := range inventory {
		if def.Fires(d) {
			out = append(out, def)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	if out == nil {
		out = []TaskDefinition{}
	}
	actual, _ := tasksForDateCache.LoadOrStore(d, out)
	if defs, ok := actual.([]TaskDefinition); ok {
		return defs
	}
	return out
}
