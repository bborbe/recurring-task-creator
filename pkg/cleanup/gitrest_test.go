// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cleanup

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGitRestClient_GetFile(t *testing.T) {
	t.Run("successful GET", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Gateway-Secret") != "my-secret" {
				t.Errorf("expected X-Gateway-Secret header, got %q", r.Header.Get("X-Gateway-Secret"))
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("file content"))
		}))
		defer srv.Close()

		client := NewGitRestClient(nil, srv.URL, "my-secret")
		content, err := client.GetFile(context.Background(), "path/to/file.md")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(content) != "file content" {
			t.Errorf("expected %q, got %q", "file content", string(content))
		}
	})

	t.Run("404 not found", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		client := NewGitRestClient(nil, srv.URL, "")
		_, err := client.GetFile(context.Background(), "nonexistent.md")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if errors.Is(err, ErrVaultConflict) {
			t.Errorf("expected not ErrVaultConflict for 404, got: %v", err)
		}
	})

	t.Run("5xx server error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("server error"))
		}))
		defer srv.Close()

		client := NewGitRestClient(nil, srv.URL, "")
		_, err := client.GetFile(context.Background(), "path/file.md")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("gateway secret omitted when empty", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Gateway-Secret") != "" {
				t.Errorf("expected no X-Gateway-Secret header, got %q", r.Header.Get("X-Gateway-Secret"))
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("content"))
		}))
		defer srv.Close()

		client := NewGitRestClient(nil, srv.URL, "")
		_, err := client.GetFile(context.Background(), "path/file.md")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestGitRestClient_ListFiles(t *testing.T) {
	t.Run("successful list", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := map[string]interface{}{
				"files": []string{"file1.md", "file2.md"},
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		client := NewGitRestClient(nil, srv.URL, "")
		files, err := client.ListFiles(context.Background(), "prefix")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 2 || files[0] != "file1.md" || files[1] != "file2.md" {
			t.Errorf("unexpected files: %v", files)
		}
	})

	t.Run("404 not found", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		client := NewGitRestClient(nil, srv.URL, "")
		_, err := client.ListFiles(context.Background(), "nonexistent")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("5xx server error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("server error"))
		}))
		defer srv.Close()

		client := NewGitRestClient(nil, srv.URL, "")
		_, err := client.ListFiles(context.Background(), "prefix")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestGitRestClient_UpdateFile_Success(t *testing.T) {
	getCalled := false
	putCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			getCalled = true
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("original content"))
			return
		}
		if r.Method == http.MethodPut {
			putCalled = true
			body := make([]byte, 1024)
			n, _ := r.Body.Read(body)
			if string(body[:n]) != "MUTATED" {
				t.Errorf("expected mutated content, got %q", string(body[:n]))
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer srv.Close()

	client := NewGitRestClient(nil, srv.URL, "")
	err := client.UpdateFile(context.Background(), "path/file.md", func([]byte) ([]byte, error) {
		return []byte("MUTATED"), nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !getCalled {
		t.Error("GET was not called")
	}
	if !putCalled {
		t.Error("PUT was not called")
	}
}

func TestGitRestClient_UpdateFile_Conflict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("original"))
			return
		}
		if r.Method == http.MethodPut {
			w.WriteHeader(http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer srv.Close()

	client := NewGitRestClient(nil, srv.URL, "")
	err := client.UpdateFile(context.Background(), "path/file.md", func([]byte) ([]byte, error) {
		return []byte("mutated"), nil
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrVaultConflict) {
		t.Errorf("expected ErrVaultConflict, got: %v", err)
	}
}

func TestGitRestClient_UpdateFile_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("content"))
			return
		}
		if r.Method == http.MethodPut {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer srv.Close()

	client := NewGitRestClient(nil, srv.URL, "")
	err := client.UpdateFile(context.Background(), "path/file.md", func([]byte) ([]byte, error) {
		return []byte("mutated"), nil
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGitRestClient_UpdateFile_GetNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewGitRestClient(nil, srv.URL, "")
	err := client.UpdateFile(context.Background(), "path/file.md", func([]byte) ([]byte, error) {
		return []byte("mutated"), nil
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGitRestClient_UpdateFile_GetUnexpectedStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	client := NewGitRestClient(nil, srv.URL, "")
	err := client.UpdateFile(context.Background(), "path/file.md", func([]byte) ([]byte, error) {
		return []byte("mutated"), nil
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGitRestClient_UpdateFile_MutatorError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("original"))
	}))
	defer srv.Close()

	client := NewGitRestClient(nil, srv.URL, "")
	err := client.UpdateFile(context.Background(), "path/file.md", func([]byte) ([]byte, error) {
		return nil, errors.New("mutator failed")
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGitRestClient_UpdateFile_PutUnexpectedStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("original"))
			return
		}
		if r.Method == http.MethodPut {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer srv.Close()

	client := NewGitRestClient(nil, srv.URL, "")
	err := client.UpdateFile(context.Background(), "path/file.md", func([]byte) ([]byte, error) {
		return []byte("mutated"), nil
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
