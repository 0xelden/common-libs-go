package main

import (
	"go/format"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateFilesColumnAllowList(t *testing.T) {
	table := &Table{
		Name: "demo_table",
		Columns: []Column{
			{Name: "id", RawType: "uuid", IsPrimary: true},
			{Name: "Name", RawType: "varchar(50)"},
			{Name: "created_at", RawType: "timestamp"},
		},
	}
	files, err := generateFiles(table, "gitlab.com/example/project", nil, false)
	if err != nil {
		t.Fatalf("generateFiles returned error: %v", err)
	}
	paths := map[string]GeneratedFile{}
	for _, file := range files {
		paths[file.Path] = file
	}

	query := paths[filepath.Join("modules", "demo_table", "demo_table.query.pg.go")]
	formatted, err := format.Source([]byte(query.Content))
	if err != nil {
		t.Fatalf("query file is not valid go: %v\n%s", err, query.Content)
	}
	if string(formatted) != query.Content {
		t.Fatalf("query file is not gofmt formatted:\n%s", query.Content)
	}
	for _, want := range []string{
		`FilterableDemoTableColumns = []string{"id", "name", "created_at"}`,
		`SortableDemoTableColumns   = []string{"id", "name", "created_at"}`,
	} {
		if !strings.Contains(query.Content, want) {
			t.Fatalf("query file missing %s:\n%s", want, query.Content)
		}
	}
	targets := map[string]bool{}
	for _, target := range query.Targets {
		targets[target.Name] = true
	}
	if !targets["FilterableDemoTableColumns"] || !targets["SortableDemoTableColumns"] {
		t.Fatalf("query file targets missing allow-list vars: %+v", query.Targets)
	}

	repo := paths[filepath.Join("modules", "demo_table", "demo_table.repo.pg.go")]
	if !strings.Contains(repo.Content, "WithColumnAllowList(FilterableDemoTableColumns, SortableDemoTableColumns)") {
		t.Fatalf("repository file should apply the column allow-list:\n%s", repo.Content)
	}
}
