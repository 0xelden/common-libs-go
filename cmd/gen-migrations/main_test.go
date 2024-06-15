package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSlugify(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "basic", input: "Add Users", want: "add-users"},
		{name: "underscore", input: "add_users", want: "add-users"},
		{name: "punctuation", input: "add@users", want: "add-users"},
		{name: "trim", input: "  add users  ", want: "add-users"},
		{name: "digits", input: "v2 2025", want: "v2-2025"},
		{name: "empty", input: "  ", wantErr: true},
		{name: "no-letters", input: "---", wantErr: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := slugify(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestNextSequence(t *testing.T) {
	t.Parallel()

	missingDir := filepath.Join(t.TempDir(), "missing")
	seq, err := nextSequence(missingDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if seq != 1 {
		t.Fatalf("expected seq 1, got %d", seq)
	}

	dir := t.TempDir()
	files := []string{
		"001-25-01-01-111-one.sql",
		"003-25-01-01-111-three.sql",
		"note.txt",
		"abcd.sql",
	}
	for _, name := range files {
		path := filepath.Join(dir, name)
		if err := osWriteFile(path, ""); err != nil {
			t.Fatalf("write file %s: %v", name, err)
		}
	}

	seq, err = nextSequence(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if seq != 4 {
		t.Fatalf("expected seq 4, got %d", seq)
	}
}

func TestBuildMigrationFilename(t *testing.T) {
	t.Parallel()

	now := time.Date(2025, 12, 27, 1, 2, 3, 0, time.UTC)
	cases := []struct {
		name          string
		cfg           config
		seq           int
		desc          string
		migrationType flywayType
		want          string
	}{
		{
			name: "default",
			cfg:  config{},
			seq:  5,
			desc: "Add Users",
			want: "005-25-12-27-1766797323-add-users.sql",
		},
		{
			name:          "flyway versioned",
			cfg:           config{flyway: true},
			seq:           5,
			desc:          "Add Users",
			migrationType: flywayTypeVersioned,
			want:          "V5__add-users.sql",
		},
		{
			name:          "flyway undo",
			cfg:           config{flyway: true},
			seq:           5,
			desc:          "Add Users",
			migrationType: flywayTypeUndo,
			want:          "U5__add-users.sql",
		},
		{
			name:          "flyway repeatable",
			cfg:           config{flyway: true},
			desc:          "Add Users",
			migrationType: flywayTypeRepeat,
			want:          "R__add-users.sql",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := buildMigrationFilename(tc.cfg, tc.seq, now, tc.desc, tc.migrationType)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestNormalizeFlywayType(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   string
		want    flywayType
		wantErr bool
	}{
		{name: "versioned long", input: "versioned", want: flywayTypeVersioned},
		{name: "versioned lower alias", input: "v", want: flywayTypeVersioned},
		{name: "versioned upper alias", input: "V", want: flywayTypeVersioned},
		{name: "undo long", input: "undo", want: flywayTypeUndo},
		{name: "undo lower alias", input: "u", want: flywayTypeUndo},
		{name: "undo upper alias", input: "U", want: flywayTypeUndo},
		{name: "repeatable long", input: "repeatable", want: flywayTypeRepeat},
		{name: "repeatable lower alias", input: "r", want: flywayTypeRepeat},
		{name: "repeatable upper alias", input: "R", want: flywayTypeRepeat},
		{name: "invalid", input: "x", wantErr: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := normalizeFlywayType(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestNextSequenceWithFlywayFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	files := []string{
		"V12__backfill.sql",
		"U9__rollback.sql",
		"R__refresh_view.sql",
		"003-25-01-01-111-three.sql",
	}
	for _, name := range files {
		path := filepath.Join(dir, name)
		if err := osWriteFile(path, ""); err != nil {
			t.Fatalf("write file %s: %v", name, err)
		}
	}

	seq, err := nextSequence(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if seq != 13 {
		t.Fatalf("expected seq 13, got %d", seq)
	}
}

func osWriteFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
