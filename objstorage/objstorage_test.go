package objstorage

import (
	"bytes"
	"context"
	"io"
	"testing"

	_ "github.com/joho/godotenv/autoload"
	"github.com/0xelden/common-libs-go/helper"
)

func TestObjectInfoMeta(t *testing.T) {
	info := ObjectInfo{UserMetadata: map[string]string{"Raw_name": "report.xlsx"}}

	for _, key := range []string{"raw_name", "Raw_name", "RAW_NAME"} {
		got, ok := info.Meta(key)
		if !ok || got != "report.xlsx" {
			t.Errorf("Meta(%q) = %q, %v; want report.xlsx, true", key, got, ok)
		}
	}

	if _, ok := info.Meta("missing"); ok {
		t.Error("Meta(missing) should not be found")
	}
}

func TestValidAzureContainer(t *testing.T) {
	valid := []string{"log-import-file", "abc", "a1b-2c3"}
	invalid := []string{"ab", "UPPER", "under_score", "-lead", "trail-", "double--dash", ""}

	for _, name := range valid {
		if serr := validAzureContainer(name); serr != nil {
			t.Errorf("validAzureContainer(%q) = %v, want nil", name, serr)
		}
	}
	for _, name := range invalid {
		if serr := validAzureContainer(name); serr == nil {
			t.Errorf("validAzureContainer(%q) = nil, want error", name)
		}
	}
}

// TestAzureRoundTrip exercises the Azure adapter against a live Azurite
// emulator (docker run -p 10000:10000 mcr.microsoft.com/azure-storage/azurite).
// Like the other connection-dependent tests, it skips itself when the env is
// not configured — there is no storage service in CI.
func TestAzureRoundTrip(t *testing.T) {
	connString := helper.Env("STORAGE_AZURE_CONNECTION_STRING", helper.Env("AZURE_STORAGE_CONNECTION_STRING"))
	if connString == "" {
		t.Log("TestAzureRoundTrip skipped")
		return
	}

	client, serr := NewAzureFromConnectionString(connString)
	if serr != nil {
		t.Fatalf("NewAzureFromConnectionString() error = %v", serr)
	}

	var (
		ctx     = context.Background()
		bucket  = "objstorage-adapter-test"
		key     = "round-trip"
		payload = []byte("hello from objstorage")
	)

	if serr := client.Ping(ctx); serr != nil {
		t.Fatalf("Ping() error = %v", serr)
	}
	if got := client.Driver(); got != DriverAzure {
		t.Errorf("Driver() = %q, want %q", got, DriverAzure)
	}

	if serr := client.EnsureBucket(ctx, bucket); serr != nil {
		t.Fatalf("EnsureBucket() error = %v", serr)
	}
	// second call must be a no-op, not a conflict error
	if serr := client.EnsureBucket(ctx, bucket); serr != nil {
		t.Fatalf("EnsureBucket() second call error = %v", serr)
	}

	serr = client.PutObject(ctx, bucket, key, bytes.NewReader(payload), int64(len(payload)), "text/plain", map[string]string{"raw_name": "hello.txt"})
	if serr != nil {
		t.Fatalf("PutObject() error = %v", serr)
	}

	info, serr := client.StatObject(ctx, bucket, key)
	if serr != nil {
		t.Fatalf("StatObject() error = %v", serr)
	}
	if info.ContentType != "text/plain" {
		t.Errorf("ContentType = %q, want text/plain", info.ContentType)
	}
	if info.Size != int64(len(payload)) {
		t.Errorf("Size = %d, want %d", info.Size, len(payload))
	}
	if name, ok := info.Meta("raw_name"); !ok || name != "hello.txt" {
		t.Errorf("Meta(raw_name) = %q, %v; want hello.txt, true", name, ok)
	}

	obj, serr := client.GetObject(ctx, bucket, key)
	if serr != nil {
		t.Fatalf("GetObject() error = %v", serr)
	}
	got, err := io.ReadAll(obj)
	_ = obj.Close()
	if err != nil {
		t.Fatalf("read object: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("object body = %q, want %q", got, payload)
	}

	if serr := client.RemoveObject(ctx, bucket, key); serr != nil {
		t.Fatalf("RemoveObject() error = %v", serr)
	}
	if _, serr := client.StatObject(ctx, bucket, key); serr == nil {
		t.Error("StatObject() after remove should fail")
	}
	// removing an already-removed object must be idempotent
	if serr := client.RemoveObject(ctx, bucket, key); serr != nil {
		t.Fatalf("RemoveObject() second call error = %v", serr)
	}
}
