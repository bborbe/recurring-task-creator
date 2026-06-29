// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cleanup

import (
	"context"
	stderrors "errors"

	"github.com/bborbe/errors"
)

// ErrVaultConflict is returned (wrapped) by a VaultWriter when git-rest
// responds 409 because the file changed between read and write. The
// orchestrator classifies it as result="conflict" and defers to the next
// tick. Use bborbe/errors with a stderrors alias per the error-wrapping guide.
var ErrVaultConflict = stderrors.New("vault write conflict (git-rest 409)")

//counterfeiter:generate -o ../../mocks/cleanup-vault-reader.go --fake-name CleanupVaultReader . VaultReader

// VaultReader reads vault files via git-rest (HTTP). Read-only.
type VaultReader interface {
	// GetFile returns the raw bytes of the file at path. Returns a
	// wrapped error on transport failure, a not-found sentinel/error on
	// 404. path is "<vault-relative-dir>/<title> - <token>.md".
	GetFile(ctx context.Context, path string) ([]byte, error)

	// ListFiles returns the relative paths of every vault file whose name
	// begins with prefix. prefix is the slug-derived directory or title
	// stem used to detect whether the next-period instance exists.
	ListFiles(ctx context.Context, prefix string) ([]string, error)
}

//counterfeiter:generate -o ../../mocks/cleanup-vault-writer.go --fake-name CleanupVaultWriter . VaultWriter

// VaultWriter performs merge-aware writes via git-rest. The mutator is
// invoked against the CURRENT file bytes (re-read inside the writer so a
// vault-cli mid-edit is not clobbered); the writer POSTs the mutated
// result back. A 409 from git-rest (file changed between read and write)
// is surfaced as an error the caller classifies as a conflict.
type VaultWriter interface {
	// UpdateFile reads the file at path, applies mutator to its current
	// bytes, and writes the result back. Returns a 409-classified error
	// on a write conflict, a generic wrapped error otherwise, nil on success.
	UpdateFile(ctx context.Context, path string, mutator func([]byte) ([]byte, error)) error
}

// IsVaultConflict reports whether err wraps ErrVaultConflict.
func IsVaultConflict(err error) bool {
	return errors.Is(err, ErrVaultConflict)
}

// VaultClient combines VaultReader and VaultWriter into a single interface
// so the factory can accept a single implementation.
type VaultClient interface {
	VaultReader
	VaultWriter
}
