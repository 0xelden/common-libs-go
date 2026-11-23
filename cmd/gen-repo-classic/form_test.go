package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateFilesCreateEditForms(t *testing.T) {
	table := &Table{
		Name: "article",
		Columns: []Column{
			{Name: "id", RawType: "uuid", IsPrimary: true},
			{Name: "title", RawType: "varchar(50)", NotNull: true},
			{Name: "seq", RawType: "bigint", IsIdentity: true},
			{Name: "status", RawType: "integer"},
			{Name: "created_at", RawType: "timestamp"},
		},
	}
	tags := map[string]map[string]string{"Title": {"validate": "required|max_len:50"}}
	files, err := generateFiles(table, "gitlab.com/example/project", tags, true)
	if err != nil {
		t.Fatalf("generateFiles returned error: %v", err)
	}
	paths := map[string]string{}
	for _, file := range files {
		paths[file.Path] = file.Content
	}
	modelFile := paths[filepath.Join("modules", "article", "dto", "article_dto.go")]

	createForm := structBody(t, modelFile, "type ArticleCreateForm struct")
	if !strings.Contains(createForm, `validate:"required|max_len:50"`) {
		t.Fatalf("create form should keep validate rules:\n%s", modelFile)
	}
	if strings.Contains(createForm, "CreatedAt") || strings.Contains(createForm, "Seq ") || strings.Contains(createForm, "db:") {
		t.Fatalf("create form should not expose audit, identity columns or db tags:\n%s", modelFile)
	}

	editForm := structBody(t, modelFile, "type ArticleEditForm struct")
	if !strings.Contains(editForm, `Title *string `+"`"+`form:"title" json:"title" validate:"max_len:50"`+"`") {
		t.Fatalf("edit form should drop required but keep other rules:\n%s", modelFile)
	}
}

func TestWithoutRequiredRule(t *testing.T) {
	cases := map[string]string{
		"":                    "",
		"required":            "",
		"required|max_len:50": "max_len:50",
		"email|required":      "email",
		"min_len:3":           "min_len:3",
	}
	for in, want := range cases {
		if got := withoutRequiredRule(in); got != want {
			t.Fatalf("withoutRequiredRule(%q) = %q, want %q", in, got, want)
		}
	}
}
