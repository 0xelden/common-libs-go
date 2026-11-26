package controller

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/0xelden/common-libs-go/objstorage"
	"github.com/0xelden/common-libs-go/serror"
	"github.com/0xelden/common-libs-go/shared"
)

type fakeStorage struct {
	contentType string
	size        int64
	puts        int
}

func (f *fakeStorage) PutObject(_ context.Context, _, _ string, reader io.Reader, size int64, contentType string, _ map[string]string) serror.SError {
	f.puts++
	f.size = size
	f.contentType = contentType
	_, _ = io.Copy(io.Discard, reader)
	return nil
}

func (f *fakeStorage) GetObject(context.Context, string, string) (io.ReadCloser, serror.SError) {
	return nil, serror.New("not found")
}

func (f *fakeStorage) StatObject(context.Context, string, string) (objstorage.ObjectInfo, serror.SError) {
	return objstorage.ObjectInfo{}, serror.New("not found")
}

func (f *fakeStorage) RemoveObject(context.Context, string, string) serror.SError { return nil }
func (f *fakeStorage) EnsureBucket(context.Context, string) serror.SError         { return nil }
func (f *fakeStorage) Ping(context.Context) serror.SError                         { return nil }
func (f *fakeStorage) Driver() string                                             { return "fake" }

var pngHeader = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")

func TestUploadFromReadSeekerContentType(t *testing.T) {
	tests := []struct {
		name     string
		body     []byte
		claimed  string
		wantType string
	}{
		{name: "png", body: pngHeader, claimed: "image/png", wantType: "image/png"},
		{name: "png claimed as svg", body: pngHeader, claimed: "image/svg+xml", wantType: "image/png"},
		{name: "pdf", body: []byte("%PDF-1.7\n"), claimed: "application/octet-stream", wantType: "application/pdf"},
		{name: "svg named png", body: []byte(`<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"></svg>`), claimed: "image/png"},
		{name: "svg with xml prolog", body: []byte(`<?xml version="1.0"?><svg onload="alert(1)"/>`), claimed: "image/svg+xml"},
		{name: "html", body: []byte(`<html><script>alert(1)</script></html>`), claimed: "image/jpeg"},
		{name: "javascript", body: []byte(`alert(document.cookie)`), claimed: "image/png"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeStorage{}
			_, serr := objStorage{client: fake}.UploadFromReadSeeker(context.Background(), bytes.NewReader(tt.body), 1, "bucket", "file", tt.claimed)
			if tt.wantType == "" {
				if serr == nil || fake.puts != 0 {
					t.Fatalf("upload accepted: err=%v puts=%d type=%q", serr, fake.puts, fake.contentType)
				}
				return
			}
			if serr != nil {
				t.Fatalf("upload rejected: %v", serr)
			}
			if fake.contentType != tt.wantType || fake.size != int64(len(tt.body)) {
				t.Fatalf("stored type=%q size=%d, want %q %d", fake.contentType, fake.size, tt.wantType, len(tt.body))
			}
		})
	}
}

func TestUploadFromReadSeekerLimits(t *testing.T) {
	t.Setenv(shared.StorageMaxUploadSize, "16")
	fake := &fakeStorage{}
	big := append(append([]byte{}, pngHeader...), bytes.Repeat([]byte{0}, 16)...)
	if _, serr := (objStorage{client: fake}).UploadFromReadSeeker(context.Background(), bytes.NewReader(big), 1, "bucket", "file", "image/png"); serr == nil || !strings.Contains(serr.Error(), "limit") {
		t.Fatalf("oversized upload: err=%v", serr)
	}

	t.Setenv(shared.StorageMaxUploadSize, "")
	t.Setenv(shared.StorageAllowedTypes, "text/plain, text/html")
	if _, serr := (objStorage{client: fake}).UploadFromReadSeeker(context.Background(), strings.NewReader("a,b\n1,2\n"), 1, "bucket", "file.csv", "text/csv"); serr != nil {
		t.Fatalf("text/plain from STORAGE_ALLOWED_TYPES rejected: %v", serr)
	}
	if _, serr := (objStorage{client: fake}).UploadFromReadSeeker(context.Background(), strings.NewReader("<html></html>"), 1, "bucket", "file", "text/html"); serr == nil {
		t.Fatal("text/html must stay blocked even when listed in STORAGE_ALLOWED_TYPES")
	}
	if _, serr := (objStorage{client: fake}).UploadFromReadSeeker(context.Background(), bytes.NewReader(pngHeader), 1, "bucket", "file", "image/png"); serr == nil {
		t.Fatal("STORAGE_ALLOWED_TYPES must replace the default list")
	}
}
