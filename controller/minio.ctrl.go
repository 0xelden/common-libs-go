package controller

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go/v7"
	"github.com/0xelden/common-libs-go/helper"
	"github.com/0xelden/common-libs-go/logger"
	"github.com/0xelden/common-libs-go/models/objstore"
	"github.com/0xelden/common-libs-go/objstorage"
	"github.com/0xelden/common-libs-go/serror"
	"github.com/0xelden/common-libs-go/service"
	"github.com/0xelden/common-libs-go/shared"
)

type objStorage struct {
	client objstorage.Client
}

// NewObjectStorage keeps the historical MinIO-typed constructor for
// backward compatibility with downstream services.
//
// Deprecated: use NewObjectStorageWithClient with connection.ConnectObjectStorage.
//
//goland:noinspection GoUnusedExportedFunction
func NewObjectStorage(client *minio.Client) service.ObjectStorageUsecase {
	return objStorage{client: objstorage.NewMinio(client)}
}

// NewObjectStorageWithClient builds the usecase on any objstorage backend
// (MinIO or Azure Blob), as selected by connection.ConnectObjectStorage.
func NewObjectStorageWithClient(client objstorage.Client) service.ObjectStorageUsecase {
	return objStorage{client: client}
}

func (o objStorage) DeleteObject(ctx context.Context, bucket, id string) (serr serror.SError) {
	return o.client.RemoveObject(ctx, bucket, id)
}

func (o objStorage) StreamObject(ctx context.Context, bucket, id string, c *gin.Context) (serr serror.SError) {
	info, serr := o.client.StatObject(ctx, bucket, id)
	if serr != nil {
		return serr
	}

	obj, serr := o.client.GetObject(ctx, bucket, id)
	if serr != nil {
		return serr
	}
	//goland:noinspection GoUnhandledErrorResult
	defer obj.Close()

	c.Header("Content-Type", info.ContentType)
	_, err := io.Copy(c.Writer, obj)
	if err != nil {
		logger.Infof("stream %s error %v", id, err)
	}

	return nil
}

func (o objStorage) DownloadObject(ctx context.Context, bucket, id string) (name string, object []byte, serr serror.SError) {
	obj, serr := o.client.GetObject(ctx, bucket, id)
	if serr != nil {
		return name, object, serr
	}
	//goland:noinspection GoUnhandledErrorResult
	defer obj.Close()

	out := new(bytes.Buffer)
	if _, err := io.Copy(out, obj); err != nil {
		return name, object, serror.NewFromError(err)
	}

	info, serr := o.client.StatObject(ctx, bucket, id)
	if serr != nil {
		return name, object, serr
	}

	if rawName, exists := info.Meta("raw_name"); exists {
		name = rawName
	}

	return name, out.Bytes(), nil
}

func (o objStorage) HashObject(ctx context.Context, reader io.ReadSeeker, bucket string) (result string, serr serror.SError) {
	h := sha256.New()
	if _, err := io.Copy(h, reader); err != nil {
		return result, serror.NewFromError(err)
	}

	result = hex.EncodeToString(h.Sum(nil))
	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		return result, serror.NewFromError(err)
	}

	return result, nil
}

func (o objStorage) UploadFromReadSeeker(ctx context.Context, reader io.ReadSeeker, size int64, bucket, id, contentType string) (result string, serr serror.SError) {
	if serr := o.CreateBucket(ctx, bucket); serr != nil {
		return result, serr
	}

	result, serr = o.HashObject(ctx, reader, bucket)
	if serr != nil {
		return result, serr
	}

	if o.IsFileExist(ctx, bucket, result) {
		return result, nil
	}

	serr = o.client.PutObject(ctx, bucket, result, reader, size, contentType, map[string]string{
		"Raw_name": id,
	})
	if serr != nil {
		return result, serr
	}

	return result, nil
}

func (o objStorage) UploadFromHeader(ctx context.Context, bucket string, fileHeader *multipart.FileHeader) (id string, serr serror.SError) {
	file, err := fileHeader.Open()
	if err != nil {
		return id, serror.NewFromError(err)
	}
	//goland:noinspection GoUnhandledErrorResult
	defer file.Close()

	id, serr = o.UploadFromReadSeeker(ctx, file, fileHeader.Size, bucket, fileHeader.Filename, fileHeader.Header.Get("Content-Type"))
	if serr != nil {
		return id, serr
	}

	return id, nil
}

func (o objStorage) CreateBucket(ctx context.Context, bucket string) serror.SError {
	return o.client.EnsureBucket(ctx, bucket)
}

func (o objStorage) IsFileExist(ctx context.Context, bucket, hash string) (exist bool) {
	_, serr := o.client.StatObject(ctx, bucket, hash)
	return serr == nil
}

func (o objStorage) ZipObjects(ctx context.Context, bucket string, form objstore.ZipObjects) (result []byte, serr serror.SError) {
	if err := shared.Validate.Struct(&form); err != nil {
		return result, serror.NewFromError(err)
	}

	tmpDir := "./tmp/" + helper.RandStringUnsafe(13)
	//goland:noinspection GoUnhandledErrorResult
	defer os.RemoveAll(tmpDir)

	err := os.MkdirAll(tmpDir, os.ModePerm)
	if err != nil {
		return nil, serror.NewFromError(err)
	}

	for _, objID := range form.ObjectId {
		name, data, serr := o.DownloadObject(ctx, bucket, objID)
		if serr != nil {
			return nil, serr
		}

		filePath := filepath.Join(tmpDir, name)
		file, err := os.Create(filePath)
		if err != nil {
			return nil, serror.NewFromError(err)
		}

		_, err = file.Write(data)
		_ = file.Close()
		if err != nil {
			return nil, serror.NewFromError(err)
		}
	}

	zipBuffer := new(bytes.Buffer)
	zipWriter := zip.NewWriter(zipBuffer)

	files, err := os.ReadDir(tmpDir)
	if err != nil {
		return nil, serror.NewFromError(err)
	}

	for _, file := range files {
		f, err := os.Open(filepath.Join(tmpDir, file.Name()))
		if err != nil {
			return nil, serror.NewFromError(err)
		}

		w, err := zipWriter.Create(file.Name())
		if err != nil {
			_ = f.Close()
			return nil, serror.NewFromError(err)
		}

		_, err = io.Copy(w, f)
		_ = f.Close()
		if err != nil {
			return nil, serror.NewFromError(err)
		}
	}

	if err := zipWriter.Close(); err != nil {
		return nil, serror.NewFromError(err)
	}

	return zipBuffer.Bytes(), nil
}
