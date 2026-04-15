// Package exportstorage provides durable storage for GDPR data exports.
// In production, exports are uploaded to Google Cloud Storage with signed
// download URLs. In local development, exports are written to a local
// directory and served via the existing REST endpoint.
package exportstorage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"cloud.google.com/go/storage"
	"github.com/google/uuid"
)

// Store abstracts durable storage for export payloads.
type Store interface {
	// Upload serializes the export data and stores it durably.
	// Returns a download URL (signed URL for GCS, local path for filesystem).
	Upload(ctx context.Context, requestID uuid.UUID, data any) (downloadURL string, err error)
}

// signedURLExpiry is how long GCS signed URLs remain valid.
const signedURLExpiry = 7 * 24 * time.Hour

// GCSStore uploads exports to Google Cloud Storage.
type GCSStore struct {
	client *storage.Client
	bucket string
}

// NewGCSStore creates a GCS-backed export store.
func NewGCSStore(client *storage.Client, bucket string) *GCSStore {
	return &GCSStore{
		client: client,
		bucket: bucket,
	}
}

func (s *GCSStore) Upload(ctx context.Context, requestID uuid.UUID, data any) (string, error) {
	objectName := fmt.Sprintf("exports/%s.json", requestID.String())

	obj := s.client.Bucket(s.bucket).Object(objectName)
	w := obj.NewWriter(ctx)
	w.ContentType = "application/json"
	w.ContentDisposition = `attachment; filename="toqui-data-export.json"`

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		// Abort the write on encoding failure.
		_ = w.Close()
		return "", fmt.Errorf("encode export to GCS: %w", err)
	}

	if err := w.Close(); err != nil {
		return "", fmt.Errorf("close GCS writer: %w", err)
	}

	// Generate a signed URL using IAM-based signing (no service account key
	// file needed in Cloud Run environments with Workload Identity).
	signedURL, signErr := s.client.Bucket(s.bucket).SignedURL(objectName, &storage.SignedURLOptions{
		Method:  "GET",
		Expires: time.Now().Add(signedURLExpiry),
		Scheme:  storage.SigningSchemeV4,
	})
	if signErr != nil {
		// Fall back to a direct GCS URL if signing fails (e.g., in emulator
		// or when IAM permissions are not configured).
		slog.Warn("signed URL generation failed, using direct URL", "bucket", s.bucket, "object", objectName, "error", signErr)
		return fmt.Sprintf("https://storage.googleapis.com/%s/%s", s.bucket, objectName), nil
	}

	return signedURL, nil
}

// LocalStore writes exports to a local directory. Used for development.
type LocalStore struct {
	dir string
}

// NewLocalStore creates a filesystem-backed export store.
// The directory is created if it does not exist.
func NewLocalStore(dir string) (*LocalStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create export directory %s: %w", dir, err)
	}
	return &LocalStore{dir: dir}, nil
}

func (s *LocalStore) Upload(_ context.Context, requestID uuid.UUID, data any) (string, error) {
	filename := fmt.Sprintf("%s.json", requestID.String())
	path := filepath.Join(s.dir, filename)

	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("create export file: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		return "", fmt.Errorf("encode export to file: %w", err)
	}

	// Return the REST endpoint path — the download handler will serve the file.
	return fmt.Sprintf("/api/export/%s", requestID.String()), nil
}

// Dir returns the local storage directory path.
func (s *LocalStore) Dir() string {
	return s.dir
}

// OpenExport opens a locally stored export file for reading.
func (s *LocalStore) OpenExport(requestID uuid.UUID) (io.ReadCloser, error) {
	filename := fmt.Sprintf("%s.json", requestID.String())
	path := filepath.Join(s.dir, filename)

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("export not found: %s", requestID)
		}
		return nil, fmt.Errorf("open export file: %w", err)
	}
	return f, nil
}
