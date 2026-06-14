// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package publisher

import (
	"fmt"

	"github.com/bborbe/agent/lib"
	"github.com/google/uuid"

	"github.com/bborbe/recurring-task-creator/pkg/schedule"
)

// uuidNamespace is the UUID5 namespace used to derive TaskIdentifier values
// for recurring tasks. It is FROZEN — never read from env or flag, never
// regenerated, never changed. Changing it is a breaking change to the entire
// downstream Kafka stream (every identifier collides; the controller will
// create a duplicate vault file for every recurring task on the next tick).
//
// If a future spec needs a new namespace, define a new constant alongside
// this one with a distinct name and do not edit this one.
var uuidNamespace uuid.UUID = uuid.MustParse("f4e1c5b7-3a82-4d59-9e7c-1c8b9d2e4f6a")

// buildTaskIdentifier returns the deterministic TaskIdentifier for the
// (slug, date) pair. The identifier is UUID5(uuidNamespace, "recurring-<slug>-<YYYY-MM-DD>").
// Same input on a second call produces the same identifier across processes,
// redeploys, and replays — this is the contract the controller's de-dup
// relies on.
func buildTaskIdentifier(slug string, date schedule.Date) lib.TaskIdentifier {
	name := "recurring-" + slug + "-" + isoDate(date)
	return lib.TaskIdentifier(uuid.NewSHA1(uuidNamespace, []byte(name)).String())
}

func isoDate(date schedule.Date) string {
	return fmt.Sprintf("%04d-%02d-%02d", date.Year, int(date.Month), date.Day)
}
