// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package schedule

// AllDefinitionsForTest exposes the inventory slice to external tests. The
// `_test.go` suffix keeps it out of production binaries.
func AllDefinitionsForTest() []TaskDefinition {
	out := make([]TaskDefinition, len(inventory))
	copy(out, inventory)
	return out
}

// FilterInventoryByDateForTest exposes the package-internal filter worker
// to external tests so the synthetic-fixture tests in
// tasks_for_date_test.go can drive the filter with a controlled inventory
// instead of the package-level 45-entry canonical inventory. Production
// callers go through TasksForDate (which reads the canonical inventory).
func FilterInventoryByDateForTest(defs []TaskDefinition, date Date) []TaskDefinition {
	return filterInventoryByDate(defs, date)
}
