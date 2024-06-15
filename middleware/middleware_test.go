package middleware

import (
	"net/url"
	"strings"
	"testing"
)

func Test_parseApp(t *testing.T) {
	type arg struct {
		url      string
		expected string
	}
	refs := []arg{
		{"https://google.com", "google.com"},
		{"https://www.google.com", "google.com"},
		{"https://www.google.com:8080", "google.com"},
		{"https://www.google.com:8080/path", "google.com"},
	}
	for _, r := range refs {
		u, err := url.Parse(r.url)
		if err != nil {
			t.Fatal(err)
		}
		host := strings.TrimPrefix(u.Hostname(), "www.")
		if host != r.expected {
			t.Fatal("invalid", host, r.expected)
		}
	}
}
