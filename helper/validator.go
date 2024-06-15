package helper

import (
	"bytes"
	"io"
	"net/http"
	"net/mail"

	"github.com/google/uuid"
	"github.com/0xelden/common-libs-go/serror"
)

// IsValidUUID check string for a valid uuid
func IsValidUUID(u string) bool {
	_, err := uuid.Parse(u)
	return err == nil
}

func IsPNG(r *bytes.Reader) (bool, serror.SError) {
	// Read the first 512 bytes (max needed for MIME sniffing)
	header := make([]byte, 512)
	n, err := r.Read(header)
	if err != nil && err != io.EOF {
		return false, serror.NewFromError(err)
	}

	// Reset reader so it's reusable
	_, err = r.Seek(0, io.SeekStart)
	if err != nil {
		return false, serror.NewFromError(err)
	}

	contentType := http.DetectContentType(header[:n])
	return contentType == "image/png", nil
}

// IsValidEmail Strict RFC 5322 parser using stdlib
func IsValidEmail(s string) bool {
	addr, err := mail.ParseAddress(s)
	if err != nil {
		return false
	}
	return addr.Address == s
}
