// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package publisher

import "github.com/google/uuid"

// UuidNamespaceForTest exposes the frozen UUID5 namespace to external
// tests so they can compute the expected identifier offline.
func UuidNamespaceForTest() uuid.UUID { return uuidNamespace }
