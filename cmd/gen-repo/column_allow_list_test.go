package main

import (
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateFilesSensitiveColumns(t *testing.T) {
	table := &Table{
		Name: "demo_user",
		Columns: []Column{
			{Name: "id", RawType: "uuid", IsPrimary: true},
			{Name: "username", RawType: "varchar(50)"},
			{Name: "password_hash", RawType: "text"},
			{Name: "Refresh_Token", RawType: "text"},
			{Name: "api_secret", RawType: "text"},
		},
	}
	files, err := generateFiles(table, "gitlab.com/example/project", map[string]map[string]string{
		"PasswordHash": {"json": "password_hash"},
	}, false)
	if err != nil {
		t.Fatalf("generateFiles returned error: %v", err)
	}
	paths := map[string]GeneratedFile{}
	for _, file := range files {
		paths[file.Path] = file
	}

	query := paths[filepath.Join("modules", "demo_user", "demo_user.query.pg.go")].Content
	for _, want := range []string{
		`FilterableDemoUserColumns = []string{"id", "username"}`,
		`SortableDemoUserColumns   = []string{"id", "username"}`,
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("query file missing %s:\n%s", want, query)
		}
	}

	model := paths[filepath.Join("modules", "demo_user", "model", "demo_user_model.go")].Content
	for _, want := range []string{
		"PasswordHash *string `form:\"password_hash\" json:\"-\" db:\"password_hash\"`",
		"RefreshToken *string `form:\"Refresh_Token\" json:\"-\" db:\"Refresh_Token\"`",
		"ApiSecret *string `form:\"api_secret\" json:\"-\" db:\"api_secret\"`",
		"Username *string `form:\"username\" json:\"username\" db:\"username\"`",
	} {
		if !strings.Contains(model, want) {
			t.Fatalf("model file missing %s:\n%s", want, model)
		}
	}

	forms := model[strings.Index(model, "type DemoUserCreateForm struct"):]
	for _, unwanted := range []string{"PasswordHash", "RefreshToken", "ApiSecret"} {
		if strings.Contains(forms, unwanted) {
			t.Fatalf("create/edit forms must not accept %s:\n%s", unwanted, forms)
		}
	}
	if !strings.Contains(forms, "Username: f.Username") {
		t.Fatalf("create/edit forms should keep regular columns:\n%s", forms)
	}
}

func TestIsSensitiveColumn(t *testing.T) {
	for name, want := range map[string]bool{
		"password":         true,
		"password_hash":    true,
		"PASSWORD_HASH":    true,
		"passwd":           true,
		"pwd":              true,
		"user_pwd":         true,
		"otp":              true,
		"salt":             true,
		"password_salt":    true,
		"refresh_token":    true,
		"client_secret":    true,
		"api_key":          true,
		"private_key":      true,
		"apikey":           true,
		"token_hash":       true,
		"hash":             true,
		"file_hash":        false,
		"hashtags":         false,
		"token_expired_at": false,
		"password_changed": false,
		"keyword":          false,
		"name":             false,
	} {
		if got := isSensitiveColumn(name); got != want {
			t.Errorf("isSensitiveColumn(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestGenerateFilesSensitiveColumnOverride(t *testing.T) {
	table := &Table{
		Name: "demo_file",
		Columns: []Column{
			{Name: "id", RawType: "uuid", IsPrimary: true},
			{Name: "content_hash", RawType: "text"},
			{Name: "otp_code", RawType: "text"},
		},
	}
	files, err := generateFiles(table, "gitlab.com/example/project", map[string]map[string]string{
		"ContentHash": {"json": "content_hash", SensitiveTagKey: "public"},
		"OtpCode":     {SensitiveTagKey: "sensitive"},
	}, false)
	if err != nil {
		t.Fatalf("generateFiles returned error: %v", err)
	}
	paths := map[string]GeneratedFile{}
	for _, file := range files {
		paths[file.Path] = file
	}

	query := paths[filepath.Join("modules", "demo_file", "demo_file.query.pg.go")].Content
	if want := `FilterableDemoFileColumns = []string{"id", "content_hash"}`; !strings.Contains(query, want) {
		t.Fatalf("query file missing %s:\n%s", want, query)
	}

	model := paths[filepath.Join("modules", "demo_file", "model", "demo_file_model.go")].Content
	for _, want := range []string{
		"ContentHash *string `form:\"content_hash\" json:\"content_hash\" db:\"content_hash\" genrepo:\"public\"`",
		"OtpCode *string `form:\"otp_code\" json:\"-\" db:\"otp_code\" genrepo:\"sensitive\"`",
		"ContentHash: f.ContentHash",
	} {
		if !strings.Contains(model, want) {
			t.Fatalf("model file missing %s:\n%s", want, model)
		}
	}
	if strings.Contains(model, "OtpCode: f.OtpCode") {
		t.Fatalf("forms must not accept an opted-in sensitive column:\n%s", model)
	}
}

func TestGenerateFilesWarnsOnPublicCredentialColumn(t *testing.T) {
	var warnings strings.Builder
	warnOut = &warnings
	t.Cleanup(func() { warnOut = os.Stderr })

	table := &Table{
		Name: "demo_user",
		Columns: []Column{
			{Name: "id", RawType: "uuid", IsPrimary: true},
			{Name: "password_hash", RawType: "text"},
			{Name: "content_hash", RawType: "text"},
		},
	}
	if _, err := generateFiles(table, "gitlab.com/example/project", map[string]map[string]string{
		"PasswordHash": {SensitiveTagKey: "public"},
		"ContentHash":  {SensitiveTagKey: "public"},
	}, false); err != nil {
		t.Fatalf("generateFiles returned error: %v", err)
	}

	got := warnings.String()
	if !strings.Contains(got, "demo_user.password_hash") {
		t.Fatalf("expected a warning for password_hash marked public, got %q", got)
	}
	if strings.Contains(got, "content_hash") {
		t.Fatalf("content_hash is not a credential column, got warning %q", got)
	}
}

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
