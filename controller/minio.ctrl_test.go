package controller

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/0xelden/common-libs-go/objstorage"
	"github.com/0xelden/common-libs-go/serror"
	"github.com/gin-gonic/gin"
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

func TestUploadFromReadSeekerSizeLimit(t *testing.T) {
	fake := &fakeStorage{}
	big := append(append([]byte{}, pngHeader...), bytes.Repeat([]byte{0}, int(DefaultMaxUploadSize))...)
	if _, serr := (objStorage{client: fake}).UploadFromReadSeeker(context.Background(), bytes.NewReader(big), 1, "bucket", "file", "image/png"); serr == nil || !strings.Contains(serr.Error(), "limit") {
		t.Fatalf("oversized upload: err=%v", serr)
	}
	if fake.puts != 0 {
		t.Fatalf("oversized upload was stored")
	}
}

func TestUploadValidated(t *testing.T) {
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><rect width="1" height="1"/></svg>`)
	webm := []byte{0x1A, 0x45, 0xDF, 0xA3}
	big := bytes.Repeat([]byte{0}, int(DefaultMaxUploadSize)+1)
	tests := []struct {
		name        string
		body        []byte
		contentType string
		extra       []string
		wantErr     bool
	}{
		{name: "default type", body: pngHeader, contentType: "image/png"},
		{name: "larger than DefaultMaxUploadSize", body: big, contentType: "video/mp4"},
		{name: "extra type from the caller", body: webm, contentType: "video/webm", extra: []string{"video/webm"}},
		{name: "svg allowed by the caller", body: svg, contentType: "image/svg+xml", extra: []string{"image/svg+xml"}},
		{name: "params are dropped", body: svg, contentType: "image/svg+xml; charset=utf-8", extra: []string{"image/svg+xml"}},
		{name: "svg by default", body: svg, contentType: "image/svg+xml", wantErr: true},
		{name: "type outside the allow-list", body: webm, contentType: "video/webm", wantErr: true},
		{name: "text/xml", body: []byte("<a/>"), contentType: "text/xml", wantErr: true},
		{name: "application/x-javascript", body: []byte("alert(1)"), contentType: "application/x-javascript", wantErr: true},
		{name: "text/ecmascript", body: []byte("alert(1)"), contentType: "text/ecmascript", wantErr: true},
		{name: "html even when listed", body: []byte("<html></html>"), contentType: "text/html", extra: []string{"text/html"}, wantErr: true},
		{name: "javascript even when listed", body: []byte("alert(1)"), contentType: "application/javascript", extra: []string{"application/javascript"}, wantErr: true},
		{name: "empty", body: svg, contentType: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeStorage{}
			_, serr := objStorage{client: fake}.UploadValidated(context.Background(), bytes.NewReader(tt.body), "bucket", "file", tt.contentType, tt.extra...)
			if tt.wantErr {
				if serr == nil || fake.puts != 0 {
					t.Fatalf("upload accepted: err=%v puts=%d", serr, fake.puts)
				}
				return
			}
			if serr != nil {
				t.Fatalf("upload rejected: %v", serr)
			}
			want, _, _ := strings.Cut(tt.contentType, ";")
			if fake.contentType != want || fake.size != int64(len(tt.body)) {
				t.Fatalf("stored type=%q size=%d, want %q %d", fake.contentType, fake.size, want, len(tt.body))
			}
		})
	}
}

type streamStorage struct {
	fakeStorage
	contentType string
}

func (s *streamStorage) StatObject(context.Context, string, string) (objstorage.ObjectInfo, serror.SError) {
	return objstorage.ObjectInfo{ContentType: s.contentType}, nil
}

func (s *streamStorage) GetObject(context.Context, string, string) (io.ReadCloser, serror.SError) {
	return io.NopCloser(strings.NewReader("body")), nil
}

func TestStreamObjectSVGPolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tt := range []struct {
		contentType string
		wantCSP     bool
	}{
		{contentType: "image/svg+xml", wantCSP: true},
		{contentType: "image/svg+xml; charset=utf-8", wantCSP: true},
		{contentType: "image/png"},
		{contentType: "application/pdf"},
	} {
		t.Run(tt.contentType, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/x", nil)

			if serr := (objStorage{client: &streamStorage{contentType: tt.contentType}}).StreamObject(context.Background(), "bucket", "id", c); serr != nil {
				t.Fatalf("StreamObject: %v", serr)
			}
			csp := w.Header().Get("Content-Security-Policy")
			if tt.wantCSP != strings.Contains(csp, "script-src 'none'") {
				t.Fatalf("Content-Security-Policy = %q, want script-src 'none': %v", csp, tt.wantCSP)
			}
			if w.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatal("missing X-Content-Type-Options: nosniff")
			}
		})
	}
}
