package objstorage

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/0xelden/common-libs-go/serror"
)

// Azurite (Azure Storage Emulator) ships with one well-known account; this is
// the same publicly documented default the .NET SDK expands
// "UseDevelopmentStorage=true" into.
const azuriteConnString = "DefaultEndpointsProtocol=http;" +
	"AccountName=devstoreaccount1;" +
	"AccountKey=Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==;" +
	"BlobEndpoint=http://127.0.0.1:10000/devstoreaccount1;"

// Azure container names: 3-63 chars, lowercase letters/digits/hyphens, must
// start+end alphanumeric, no consecutive hyphens. Stricter than MinIO buckets,
// so validate up front to fail with a readable error instead of a REST 400.
var azContainerRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$`)

type azureClient struct {
	client *azblob.Client
}

// NewAzureFromConnectionString builds an objstorage.Client for Azure Blob
// Storage. The value "UseDevelopmentStorage=true" is expanded to the
// well-known local Azurite emulator account, mirroring the .NET SDK.
func NewAzureFromConnectionString(connString string) (Client, serror.SError) {
	if strings.EqualFold(strings.TrimSpace(connString), "UseDevelopmentStorage=true") {
		connString = azuriteConnString
	}
	cli, err := azblob.NewClientFromConnectionString(connString, nil)
	if err != nil {
		return nil, serror.NewFromError(err)
	}
	return azureClient{client: cli}, nil
}

// NewAzureSharedKey builds an objstorage.Client from an account name + key.
// serviceURL may be empty; it defaults to https://{account}.blob.core.windows.net.
func NewAzureSharedKey(account, key, serviceURL string) (Client, serror.SError) {
	if strings.TrimSpace(serviceURL) == "" {
		serviceURL = fmt.Sprintf("https://%s.blob.core.windows.net", account)
	}
	cred, err := azblob.NewSharedKeyCredential(account, key)
	if err != nil {
		return nil, serror.NewFromError(err)
	}
	cli, err := azblob.NewClientWithSharedKeyCredential(serviceURL, cred, nil)
	if err != nil {
		return nil, serror.NewFromError(err)
	}
	return azureClient{client: cli}, nil
}

func validAzureContainer(bucket string) serror.SError {
	if !azContainerRe.MatchString(bucket) || strings.Contains(bucket, "--") {
		return serror.Newf("invalid Azure container name %q: must be 3-63 lowercase letters, digits or single hyphens, starting and ending alphanumeric", bucket)
	}
	return nil
}

func toAzureMeta(meta map[string]string) map[string]*string {
	if len(meta) == 0 {
		return nil
	}
	out := make(map[string]*string, len(meta))
	for k, v := range meta {
		v := v
		out[k] = &v
	}
	return out
}

func fromAzureMeta(meta map[string]*string) map[string]string {
	out := make(map[string]string, len(meta))
	for k, v := range meta {
		if v != nil {
			out[k] = *v
		}
	}
	return out
}

func (a azureClient) blobClient(bucket, key string) *blob.Client {
	return a.client.ServiceClient().NewContainerClient(bucket).NewBlobClient(key)
}

func (a azureClient) PutObject(ctx context.Context, bucket, key string, reader io.Reader, size int64, contentType string, meta map[string]string) serror.SError {
	opts := &azblob.UploadStreamOptions{
		Metadata: toAzureMeta(meta),
	}
	if contentType != "" {
		opts.HTTPHeaders = &blob.HTTPHeaders{BlobContentType: &contentType}
	}
	if _, err := a.client.UploadStream(ctx, bucket, key, reader, opts); err != nil {
		return serror.NewFromError(err)
	}
	return nil
}

func (a azureClient) GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, serror.SError) {
	resp, err := a.client.DownloadStream(ctx, bucket, key, nil)
	if err != nil {
		return nil, serror.NewFromError(err)
	}
	return resp.Body, nil
}

func (a azureClient) StatObject(ctx context.Context, bucket, key string) (ObjectInfo, serror.SError) {
	props, err := a.blobClient(bucket, key).GetProperties(ctx, nil)
	if err != nil {
		return ObjectInfo{}, serror.NewFromError(err)
	}

	info := ObjectInfo{UserMetadata: fromAzureMeta(props.Metadata)}
	if props.ContentType != nil {
		info.ContentType = *props.ContentType
	}
	if props.ContentLength != nil {
		info.Size = *props.ContentLength
	}
	return info, nil
}

func (a azureClient) RemoveObject(ctx context.Context, bucket, key string) serror.SError {
	if _, err := a.client.DeleteBlob(ctx, bucket, key, nil); err != nil {
		if bloberror.HasCode(err, bloberror.BlobNotFound) {
			return nil
		}
		return serror.NewFromError(err)
	}
	return nil
}

func (a azureClient) Ping(ctx context.Context) serror.SError {
	// Account-level read; works with shared key and on Azurite, and never
	// creates anything.
	if _, err := a.client.ServiceClient().GetProperties(ctx, nil); err != nil {
		return serror.NewFromError(err)
	}
	return nil
}

func (a azureClient) Driver() string {
	return DriverAzure
}

func (a azureClient) EnsureBucket(ctx context.Context, bucket string) serror.SError {
	if serr := validAzureContainer(bucket); serr != nil {
		return serr
	}
	if _, err := a.client.CreateContainer(ctx, bucket, nil); err != nil {
		if bloberror.HasCode(err, bloberror.ContainerAlreadyExists) {
			return nil
		}
		return serror.NewFromError(err)
	}
	return nil
}
