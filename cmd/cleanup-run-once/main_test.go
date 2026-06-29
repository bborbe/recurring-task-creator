// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"testing"
)

func TestCompile(t *testing.T) {
	// Compile test: verify that the main package compiles correctly.
	// This test will fail at compile time if there are type errors.
	_ = struct {
		app application
	}{}
}
