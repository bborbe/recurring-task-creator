// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package publisher

import (
	"github.com/google/uuid"
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
