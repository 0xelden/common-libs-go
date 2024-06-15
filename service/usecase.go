package service

import (
	"context"
	"io"
	"mime/multipart"

	"github.com/gin-gonic/gin"
	"github.com/0xelden/common-libs-go/models/objstore"
	"github.com/0xelden/common-libs-go/serror"
)

type ObjectStorageUsecase interface {
	HashObject(ctx context.Context, reader io.ReadSeeker, bucket string) (result string, serr serror.SError)
	UploadFromHeader(ctx context.Context, bucket string, fileHeader *multipart.FileHeader) (id string, serr serror.SError)
	UploadFromReadSeeker(ctx context.Context, reader io.ReadSeeker, size int64, bucket, id, contentType string) (result string, serr serror.SError)
	StreamObject(ctx context.Context, bucket, id string, c *gin.Context) (serr serror.SError)
	DownloadObject(ctx context.Context, bucket, id string) (name string, result []byte, serr serror.SError)
	DeleteObject(ctx context.Context, bucket, id string) (serr serror.SError)
	CreateBucket(ctx context.Context, bucket string) serror.SError
	IsFileExist(ctx context.Context, bucket, hash string) (exist bool)
	ZipObjects(ctx context.Context, bucket string, form objstore.ZipObjects) (result []byte, serr serror.SError)
}
