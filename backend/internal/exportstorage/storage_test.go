package exportstorage

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestLocalStoreUpload(t *testing.T) {
	dir := t.TempDir()
	store, err := NewLocalStore(dir)
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}

	requestID := uuid.New()
	data := map[string]any{
		"exported_at": "2026-04-14T00:00:00Z",
		"user":        map[string]any{"id": "test-user", "email": "test@example.com"},
		"trips":       []any{},
	}

	downloadURL, err := store.Upload(context.Background(), requestID, data)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}

	// Verify the download URL points to the REST endpoint.
	expectedPath := "/api/export/" + requestID.String()
	if downloadURL != expectedPath {
		t.Errorf("download URL = %q, want %q", downloadURL, expectedPath)
	}

	// Verify the file was written.
	filePath := filepath.Join(dir, requestID.String()+".json")
	stat, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if stat.Size() == 0 {
		t.Error("file is empty")
	}

	// Verify the content is valid JSON with expected fields.
	f, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("open file: %v", err)
	}
	defer f.Close()

	var decoded map[string]any
	if err := json.NewDecoder(f).Decode(&decoded); err != nil {
		t.Fatalf("decode file: %v", err)
	}

	if decoded["exported_at"] != "2026-04-14T00:00:00Z" {
		t.Errorf("exported_at = %v, want 2026-04-14T00:00:00Z", decoded["exported_at"])
	}
}

func TestLocalStoreOpenExport(t *testing.T) {
	dir := t.TempDir()
	store, err := NewLocalStore(dir)
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}

	requestID := uuid.New()
	data := map[string]string{"test": "data"}

	_, err = store.Upload(context.Background(), requestID, data)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}

	// Open the export.
	rc, err := store.OpenExport(requestID)
	if err != nil {
		t.Fatalf("OpenExport: %v", err)
	}
	defer rc.Close()

	content, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}

	if !strings.Contains(string(content), `"test"`) {
		t.Errorf("content missing expected data: %s", content)
	}
}

func TestLocalStoreOpenExportNotFound(t *testing.T) {
	dir := t.TempDir()
	store, err := NewLocalStore(dir)
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}

	_, err = store.OpenExport(uuid.New())
	if err == nil {
		t.Error("expected error for non-existent export")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want to contain 'not found'", err.Error())
	}
}

func TestLocalStoreDir(t *testing.T) {
	dir := t.TempDir()
	store, err := NewLocalStore(dir)
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}

	if store.Dir() != dir {
		t.Errorf("Dir() = %q, want %q", store.Dir(), dir)
	}
}

func TestNewLocalStoreCreatesDir(t *testing.T) {
	base := t.TempDir()
	nestedDir := filepath.Join(base, "exports", "nested")

	store, err := NewLocalStore(nestedDir)
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}

	info, err := os.Stat(store.Dir())
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("path is not a directory")
	}
}

func TestStoreInterface(t *testing.T) {
	// Verify that LocalStore implements the Store interface.
	var _ Store = (*LocalStore)(nil)
	var _ Store = (*GCSStore)(nil)
}
