// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cleanup

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bborbe/errors"
)

// gitRestClient is a VaultReader + VaultWriter backed by a git-rest HTTP service.
type gitRestClient struct {
	httpClient    *http.Client
	baseURL       string
	gatewaySecret string
}

// NewGitRestClient builds a VaultReader+VaultWriter that talks to a git-rest HTTP service.
// Each HTTP call uses a 10-second timeout. When gatewaySecret is non-empty, the
// X-Gateway-Secret header is forwarded to git-rest.
func NewGitRestClient(httpClient *http.Client, baseURL, gatewaySecret string) *gitRestClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &gitRestClient{
		httpClient:    httpClient,
		baseURL:       strings.TrimSuffix(baseURL, "/"),
		gatewaySecret: gatewaySecret,
	}
}

// Compile-time check that gitRestClient satisfies both interfaces.
var _ VaultReader = (*gitRestClient)(nil)
var _ VaultWriter = (*gitRestClient)(nil)

// GetFile implements VaultReader.
func (g *gitRestClient) GetFile(ctx context.Context, path string) ([]byte, error) {
	url := g.baseURL + "/files/" + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.Wrapf(ctx, err, "create GET request for %s", path)
	}
	g.addAuthHeader(req)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrapf(ctx, err, "GET %s: request failed", path)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, errors.Wrapf(ctx, ErrVaultNotFound, "GET %s: not found", path)
	}
	if resp.StatusCode >= 500 {
		body, _ := io.ReadAll(resp.Body)
		return nil, errors.Wrapf(ctx, ErrVaultServerError, "GET %s: server error %d: %s", path, resp.StatusCode, string(body))
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, errors.Wrapf(ctx, ErrVaultUnexpectedStatus, "GET %s: unexpected status %d: %s", path, resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

// ListFiles implements VaultReader.
func (g *gitRestClient) ListFiles(ctx context.Context, prefix string) ([]string, error) {
	url := g.baseURL + "/files?prefix=" + prefix
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.Wrapf(ctx, err, "create LIST request for prefix %s", prefix)
	}
	g.addAuthHeader(req)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrapf(ctx, err, "LIST %s: request failed", prefix)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		body, _ := io.ReadAll(resp.Body)
		return nil, errors.Wrapf(ctx, ErrVaultServerError, "LIST %s: server error %d: %s", prefix, resp.StatusCode, string(body))
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, errors.Wrapf(ctx, ErrVaultUnexpectedStatus, "LIST %s: unexpected status %d: %s", prefix, resp.StatusCode, string(body))
	}

	var result listFilesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, errors.Wrapf(ctx, err, "decode LIST response for prefix %s", prefix)
	}
	return result.Files, nil
}

// UpdateFile implements VaultWriter. It re-reads the current file bytes, applies
// mutator, and writes the result back. On a git-rest 409 response, returns
// ErrVaultConflict.
func (g *gitRestClient) UpdateFile(ctx context.Context, path string, mutator func([]byte) ([]byte, error)) error {
	// Read current content
	current, err := func() ([]byte, error) {
		url := g.baseURL + "/files/" + path
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, errors.Wrapf(ctx, err, "UpdateFile: create GET request")
		}
		g.addAuthHeader(req)

		resp, err := g.httpClient.Do(req)
		if err != nil {
			return nil, errors.Wrapf(ctx, err, "UpdateFile: GET %s: request failed", path)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			return nil, errors.Wrapf(ctx, ErrVaultNotFound, "UpdateFile: file not found %s", path)
		}
		if resp.StatusCode >= 500 {
			body, _ := io.ReadAll(resp.Body)
			return nil, errors.Wrapf(ctx, ErrVaultServerError, "UpdateFile: GET %s: server error %d: %s", path, resp.StatusCode, string(body))
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, errors.Wrapf(ctx, ErrVaultUnexpectedStatus, "UpdateFile: GET %s: unexpected status %d: %s", path, resp.StatusCode, string(body))
		}
		return io.ReadAll(resp.Body)
	}()
	if err != nil {
		return err
	}

	mutated, err := mutator(current)
	if err != nil {
		return errors.Wrapf(ctx, err, "UpdateFile: mutator failed for %s", path)
	}

	// Write back
	url := g.baseURL + "/files/" + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(mutated))
	if err != nil {
		return errors.Wrapf(ctx, err, "UpdateFile: create PUT request for %s", path)
	}
	req.Header.Set("Content-Type", "text/plain")
	g.addAuthHeader(req)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return errors.Wrapf(ctx, err, "UpdateFile: PUT %s: request failed", path)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return errors.Wrapf(ctx, ErrVaultConflict, "UpdateFile: conflict for %s", path)
	}
	if resp.StatusCode >= 500 {
		body, _ := io.ReadAll(resp.Body)
		return errors.Wrapf(ctx, ErrVaultServerError, "UpdateFile: PUT %s: server error %d: %s", path, resp.StatusCode, string(body))
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return errors.Wrapf(ctx, ErrVaultUnexpectedStatus, "UpdateFile: PUT %s: unexpected status %d: %s", path, resp.StatusCode, string(body))
	}
	return nil
}

func (g *gitRestClient) addAuthHeader(req *http.Request) {
	if g.gatewaySecret != "" {
		req.Header.Set("X-Gateway-Secret", g.gatewaySecret)
	}
}

type listFilesResponse struct {
	Files []string `json:"files"`
}

// Sentinel errors for the git-rest client.
var (
	// ErrVaultNotFound is returned on a 404 from git-rest.
	ErrVaultNotFound = stderrors.New("vault file not found")

	// ErrVaultServerError is returned on a 5xx from git-rest.
	ErrVaultServerError = stderrors.New("vault server error")

	// ErrVaultUnexpectedStatus is returned on any unexpected non-2xx status.
	ErrVaultUnexpectedStatus = stderrors.New("vault unexpected status")
)
