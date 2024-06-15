// Package objstorage abstracts object storage backends (MinIO/S3, Azure Blob
// Storage) behind a single Client interface so consumers never depend on a
// concrete SDK. The backend is selected at startup from env via
// connection.ConnectObjectStorage (STORAGE_DRIVER).
package objstorage

import (
	"context"
	"io"
	"strings"

	"github.com/0xelden/common-libs-go/serror"
)

// Driver names accepted by STORAGE_DRIVER.
const (
	DriverMinio = "minio"
	DriverAzure = "azure"
)

// ObjectInfo describes a stored object, normalized across backends.
type ObjectInfo struct {
	ContentType  string
	Size         int64
	UserMetadata map[string]string
}

// Meta returns a user-metadata value by key, case-insensitively. Backends
// disagree on key casing (MinIO title-cases keys, Azure lowercases them), so
// lookups must never assume exact case.
func (o ObjectInfo) Meta(key string) (string, bool) {
	for k, v := range o.UserMetadata {
		if strings.EqualFold(k, key) {
			return v, true
		}
	}
	return "", false
}

// Client is the minimal object-storage surface the common library needs.
// "Bucket" maps to an S3/MinIO bucket or an Azure Blob container.
type Client interface {
	// PutObject uploads reader as bucket/key. meta keys must be valid
	// identifiers ([A-Za-z_][A-Za-z0-9_]*) to stay portable to Azure.
	PutObject(ctx context.Context, bucket, key string, reader io.Reader, size int64, contentType string, meta map[string]string) serror.SError
	// GetObject streams the object body; the caller must Close it.
	GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, serror.SError)
	// StatObject returns object metadata, or an error if it does not exist.
	StatObject(ctx context.Context, bucket, key string) (ObjectInfo, serror.SError)
	// RemoveObject deletes the object; backends without force/governance
	// semantics perform a plain delete.
	RemoveObject(ctx context.Context, bucket, key string) serror.SError
	// EnsureBucket creates the bucket/container when it does not exist yet.
	EnsureBucket(ctx context.Context, bucket string) serror.SError
	// Ping verifies the backend is reachable with the configured
	// credentials, without creating or mutating anything.
	Ping(ctx context.Context) serror.SError
	// Driver reports which backend this client talks to (DriverMinio /
	// DriverAzure) — for health/status endpoints, never for branching logic.
	Driver() string
}
