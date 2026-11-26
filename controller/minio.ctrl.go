package controller

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xelden/common-libs-go/helper"
	"github.com/0xelden/common-libs-go/logger"
	"github.com/0xelden/common-libs-go/models/objstore"
	"github.com/0xelden/common-libs-go/objstorage"
	"github.com/0xelden/common-libs-go/serror"
	"github.com/0xelden/common-libs-go/service"
	"github.com/0xelden/common-libs-go/shared"
	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go/v7"
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

	c.Header("X-Content-Type-Options", "nosniff")
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

const DefaultMaxUploadSize int64 = 10 << 20

var (
	DefaultAllowedUploadTypes = []string{"image/jpeg", "image/png", "image/webp", "image/gif", "application/pdf", "video/mp4"}
	blockedUploadTypes        = []string{"text/html", "text/xml", "application/xml", "image/svg+xml", "text/javascript", "application/javascript"}
	validatedRefusedTypes     = []string{"text/html", "application/xhtml+xml", "text/javascript", "application/javascript"}
)

// allowedUploadTypes reads STORAGE_ALLOWED_TYPES (comma-separated MIME types),
// falling back to DefaultAllowedUploadTypes.
func allowedUploadTypes() []string {
	raw := helper.Env(shared.StorageAllowedTypes)
	if strings.TrimSpace(raw) == "" {
		return DefaultAllowedUploadTypes
	}
	var types []string
	for _, t := range strings.Split(raw, ",") {
		if t = strings.ToLower(strings.TrimSpace(t)); t != "" {
			types = append(types, t)
		}
	}
	return types
}

// checkUpload enforces STORAGE_MAX_UPLOAD_SIZE (bytes, default
// DefaultMaxUploadSize) and returns the content type sniffed from the first
// 512 bytes. HTML, XML, SVG and JavaScript are always rejected; anything else
// must be in allowedUploadTypes. The reader is rewound before returning.
func checkUpload(reader io.ReadSeeker) (size int64, contentType string, serr serror.SError) {
	size, err := reader.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, "", serror.NewFromError(err)
	}
	if maxSize := helper.StringToInt(helper.Env(shared.StorageMaxUploadSize), DefaultMaxUploadSize); size > maxSize {
		return 0, "", serror.Newf("file is %d bytes, larger than the %d byte limit", size, maxSize)
	}
	if _, err = reader.Seek(0, io.SeekStart); err != nil {
		return 0, "", serror.NewFromError(err)
	}

	header := make([]byte, 512)
	n, err := io.ReadFull(reader, header)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return 0, "", serror.NewFromError(err)
	}
	if _, err = reader.Seek(0, io.SeekStart); err != nil {
		return 0, "", serror.NewFromError(err)
	}

	contentType, _, err = mime.ParseMediaType(http.DetectContentType(header[:n]))
	if err != nil {
		return 0, "", serror.NewFromError(err)
	}
	for _, blocked := range blockedUploadTypes {
		if contentType == blocked {
			return 0, "", serror.Newf("file type %s is not allowed", contentType)
		}
	}
	for _, allowed := range allowedUploadTypes() {
		if contentType == allowed {
			return size, contentType, nil
		}
	}
	return 0, "", serror.Newf("file type %s is not allowed", contentType)
}

// UploadFromReadSeeker stores reader under its SHA-256 hash. The size and
// content type are taken from the content itself (see checkUpload), not from
// the size and contentType arguments, which come from the client.
func (o objStorage) UploadFromReadSeeker(ctx context.Context, reader io.ReadSeeker, size int64, bucket, id, contentType string) (result string, serr serror.SError) {
	size, contentType, serr = checkUpload(reader)
	if serr != nil {
		return result, serr
	}
	return o.store(ctx, reader, size, bucket, id, contentType)
}

// UploadValidated stores reader with a contentType the caller has already
// derived from the content (sniffed, allow-listed and, for SVG, sanitized),
// skipping checkUpload's allow-list and size limit. Never pass the client's
// Content-Type header here. HTML and JavaScript are still refused.
func (o objStorage) UploadValidated(ctx context.Context, reader io.ReadSeeker, bucket, id, contentType string) (result string, serr serror.SError) {
	contentType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return result, serror.Newf("invalid content type: %v", err)
	}
	for _, refused := range validatedRefusedTypes {
		if contentType == refused {
			return result, serror.Newf("file type %s is not allowed", contentType)
		}
	}
	size, err := reader.Seek(0, io.SeekEnd)
	if err != nil {
		return result, serror.NewFromError(err)
	}
	if _, err = reader.Seek(0, io.SeekStart); err != nil {
		return result, serror.NewFromError(err)
	}
	return o.store(ctx, reader, size, bucket, id, contentType)
}

func (o objStorage) store(ctx context.Context, reader io.ReadSeeker, size int64, bucket, id, contentType string) (result string, serr serror.SError) {
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

	err := os.MkdirAll(tmpDir, 0750)
	if err != nil {
		return nil, serror.NewFromError(err)
	}

	for _, objID := range form.ObjectId {
		name, data, serr := o.DownloadObject(ctx, bucket, objID)
		if serr != nil {
			return nil, serr
		}

		name = filepath.Base(name)
		if name == "." || name == ".." || name == string(filepath.Separator) {
			return nil, serror.Newf("invalid object name for %s", objID)
		}

		filePath := filepath.Join(tmpDir, name)
		file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600) // #nosec G304
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
		f, err := os.Open(filepath.Join(tmpDir, file.Name())) // #nosec G304
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
