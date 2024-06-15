package objstorage

import (
	"context"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/0xelden/common-libs-go/serror"
)

type minioClient struct {
	client *minio.Client
}

// NewMinio wraps an already-connected MinIO client as an objstorage.Client.
func NewMinio(client *minio.Client) Client {
	return minioClient{client: client}
}

func (m minioClient) PutObject(ctx context.Context, bucket, key string, reader io.Reader, size int64, contentType string, meta map[string]string) serror.SError {
	_, err := m.client.PutObject(ctx, bucket, key, reader, size, minio.PutObjectOptions{
		ContentType:  contentType,
		UserMetadata: meta,
	})
	if err != nil {
		return serror.NewFromError(err)
	}
	return nil
}

func (m minioClient) GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, serror.SError) {
	obj, err := m.client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, serror.NewFromError(err)
	}
	return obj, nil
}

func (m minioClient) StatObject(ctx context.Context, bucket, key string) (ObjectInfo, serror.SError) {
	info, err := m.client.StatObject(ctx, bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return ObjectInfo{}, serror.NewFromError(err)
	}
	return ObjectInfo{
		ContentType:  info.ContentType,
		Size:         info.Size,
		UserMetadata: info.UserMetadata,
	}, nil
}

func (m minioClient) RemoveObject(ctx context.Context, bucket, key string) serror.SError {
	err := m.client.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{
		ForceDelete:      true,
		GovernanceBypass: true,
	})
	if err != nil {
		return serror.NewFromError(err)
	}
	return nil
}

func (m minioClient) Ping(ctx context.Context) serror.SError {
	// BucketExists is a cheap HEAD that only errors on connectivity/auth
	// problems — a missing probe bucket returns (false, nil).
	if _, err := m.client.BucketExists(ctx, "healthz-probe"); err != nil {
		return serror.NewFromError(err)
	}
	return nil
}

func (m minioClient) Driver() string {
	return DriverMinio
}

func (m minioClient) EnsureBucket(ctx context.Context, bucket string) serror.SError {
	found, err := m.client.BucketExists(ctx, bucket)
	if err != nil {
		return serror.NewFromError(err)
	}
	if found {
		return nil
	}
	if err := m.client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
		return serror.NewFromError(err)
	}
	return nil
}
