package lifecycle

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/gallowaysoftware/toqui/backend/internal/exportstorage"
)

func TestHasLocalExport_NilStore(t *testing.T) {
	s := &Service{}
	if s.HasLocalExport(uuid.New()) {
		t.Error("expected false when no export store configured")
	}
}

func TestOpenLocalExport_NilStore(t *testing.T) {
	s := &Service{}
	_, err := s.OpenLocalExport(uuid.New())
	if err == nil {
		t.Error("expected error when no export store configured")
	}
}

func TestHasLocalExport_WithLocalStore(t *testing.T) {
	dir := t.TempDir()
	store, err := exportstorage.NewLocalStore(dir)
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}

	s := &Service{}
	s.SetExportStore(store)

	requestID := uuid.New()

	// Before upload, should not exist.
	if s.HasLocalExport(requestID) {
		t.Error("expected false before upload")
	}

	// Upload a test export.
	_, err = store.Upload(context.Background(), requestID, map[string]string{"test": "data"})
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}

	// After upload, should exist.
	if !s.HasLocalExport(requestID) {
		t.Error("expected true after upload")
	}
}

func TestOpenLocalExport_WithLocalStore(t *testing.T) {
	dir := t.TempDir()
	store, err := exportstorage.NewLocalStore(dir)
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}

	s := &Service{}
	s.SetExportStore(store)

	requestID := uuid.New()

	// Upload a test export.
	_, err = store.Upload(context.Background(), requestID, map[string]string{"test": "data"})
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}

	rc, err := s.OpenLocalExport(requestID)
	if err != nil {
		t.Fatalf("OpenLocalExport: %v", err)
	}
	rc.Close()
}

func TestOpenLocalExport_NotFound(t *testing.T) {
	dir := t.TempDir()
	store, err := exportstorage.NewLocalStore(dir)
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}

	s := &Service{}
	s.SetExportStore(store)

	_, err = s.OpenLocalExport(uuid.New())
	if err == nil {
		t.Error("expected error for non-existent export")
	}
}

func TestHasLocalExport_GCSStoreReturnsFalse(t *testing.T) {
	// GCSStore is not a LocalStore, so HasLocalExport should return false.
	gcsStore := exportstorage.NewGCSStore(nil, "test-bucket")
	s := &Service{}
	s.SetExportStore(gcsStore)

	if s.HasLocalExport(uuid.New()) {
		t.Error("expected false for GCS store")
	}
}

func TestOpenLocalExport_GCSStoreReturnsError(t *testing.T) {
	gcsStore := exportstorage.NewGCSStore(nil, "test-bucket")
	s := &Service{}
	s.SetExportStore(gcsStore)

	_, err := s.OpenLocalExport(uuid.New())
	if err == nil {
		t.Error("expected error for GCS store")
	}
}
