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
