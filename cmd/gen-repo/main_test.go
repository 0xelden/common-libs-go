package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-errors/errors"
)

func TestParseCreateTable(t *testing.T) {
	ddl := `
create table public.sample_table (
    id uuid primary key not null,
    name varchar(50),
    status integer not null,
    created_at timestamp(6),
    description text
);
`
	table, err := ParseCreateTable(ddl)
	if err != nil {
		t.Fatalf("ParseCreateTable returned error: %v", err)
	}
	if table.Schema != "public" {
		t.Fatalf("expected schema 'public', got %q", table.Schema)
	}
	if table.Name != "sample_table" {
		t.Fatalf("expected table name sample_table, got %q", table.Name)
	}
	if len(table.Columns) != 5 {
		t.Fatalf("expected 5 columns, got %d", len(table.Columns))
	}
	var idCol Column
	var statusCol Column
	for _, col := range table.Columns {
		switch col.Name {
		case "id":
			idCol = col
		case "status":
			statusCol = col
		}
	}
	if !idCol.NotNull || !idCol.IsPrimary {
		t.Fatalf("id column should be primary key and not null: %+v", idCol)
	}
	if !statusCol.NotNull {
		t.Fatalf("status column should be marked not null: %+v", statusCol)
	}
}

func TestParseCreateTableGeneratedColumn(t *testing.T) {
	ddl := `
create table public.sample_table (
    id uuid primary key not null,
    date varchar(10),
    year integer generated always as ((split_part(date, '-', 1))::integer) stored
);
`
	table, err := ParseCreateTable(ddl)
	if err != nil {
		t.Fatalf("ParseCreateTable returned error: %v", err)
	}

	var yearCol Column
	for _, col := range table.Columns {
		if col.Name == "year" {
			yearCol = col
			break
		}
	}
	if !yearCol.IsGenerated {
		t.Fatalf("expected generated column metadata: %+v", yearCol)
	}
	if !strings.Contains(yearCol.GenerationExpression, "split_part") {
		t.Fatalf("expected generated expression to be captured: %+v", yearCol)
	}
}

func TestLoadTableDefinitionFromDatabase(t *testing.T) {
	db := newStubDB(
		[][]driver.Value{
			{"id"},
		},
		[][]driver.Value{
			{"id", "uuid", "uuid", true, true, false, ""},
			{"name", "character varying", "varchar", false, false, false, ""},
			{"year", "integer", "int4", false, false, true, "(split_part((date)::text, '-'::text, 1))::integer"},
			{"created_at", "timestamp without time zone", "timestamp", false, false, false, ""},
		},
	)
	defer func() {
		currentStubData = nil
		_ = db.Close()
	}()

	table, err := loadTableDefinition(db, "public.demo_table")
	if err != nil {
		t.Fatalf("loadTableDefinition returned error: %v", err)
	}
	if table.Schema != "public" {
		t.Fatalf("expected schema public, got %s", table.Schema)
	}
	if table.Name != "demo_table" {
		t.Fatalf("expected table demo_table, got %s", table.Name)
	}
	if len(table.Columns) != 4 {
		t.Fatalf("expected 4 columns, got %d", len(table.Columns))
	}
	id := table.Columns[0]
	if id.Name != "id" || !id.IsPrimary || !id.NotNull {
		t.Fatalf("id column not marked primary/not null: %+v", id)
	}
	if id.RawType != "uuid" {
		t.Fatalf("id raw type mismatch, got %s", id.RawType)
	}
	name := table.Columns[1]
	if name.RawType != "character varying" {
		t.Fatalf("name raw type mismatch, got %s", name.RawType)
	}
	year := table.Columns[2]
	if !year.IsGenerated {
		t.Fatalf("expected year to be marked generated: %+v", year)
	}
	if !strings.Contains(year.GenerationExpression, "split_part") {
		t.Fatalf("expected generation expression for year: %+v", year)
	}
	created := table.Columns[3]
	if created.RawType != "timestamp without time zone" {
		t.Fatalf("created_at raw type mismatch, got %s", created.RawType)
	}
}

var currentStubData *stubData

var registerStubDriver sync.Once

type stubData struct {
	pkRows     [][]driver.Value
	columnRows [][]driver.Value
}

type stubDriver struct{}

func (d stubDriver) Open(string) (driver.Conn, error) {
	if currentStubData == nil {
		return nil, errors.New("stub data not configured")
	}
	return &stubConn{data: currentStubData}, nil
}

type stubConn struct {
	data *stubData
}

func (c *stubConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare not implemented")
}

func (c *stubConn) Close() error {
	return nil
}

func (c *stubConn) Begin() (driver.Tx, error) {
	return nil, errors.New("begin not implemented")
}

func (c *stubConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	q := strings.ToLower(query)
	switch {
	case strings.Contains(q, "information_schema.table_constraints"):
		return newStubRows([]string{"column_name"}, c.data.pkRows), nil
	case strings.Contains(q, "information_schema.columns"):
		return newStubRows([]string{"column_name", "data_type", "udt_name", "not_null", "has_default", "is_generated", "generation_expression"}, c.data.columnRows), nil
	default:
		return nil, fmt.Errorf("unexpected query: %s", query)
	}
}

type stubRows struct {
	columns []string
	rows    [][]driver.Value
	idx     int
}

func newStubRows(columns []string, rows [][]driver.Value) *stubRows {
	columnsCopy := make([]string, len(columns))
	copy(columnsCopy, columns)
	return &stubRows{columns: columnsCopy, rows: rows}
}

func (r *stubRows) Columns() []string {
	return r.columns
}

func (r *stubRows) Close() error {
	return nil
}

func (r *stubRows) Next(dest []driver.Value) error {
	if r.idx >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.idx])
	r.idx++
	return nil
}

func newStubDB(pkRows, columnRows [][]driver.Value) *sql.DB {
	currentStubData = &stubData{
		pkRows:     pkRows,
		columnRows: columnRows,
	}
	return openStubDB()
}

func openStubDB() *sql.DB {
	registerStubDriver.Do(func() {
		sql.Register("stub", stubDriver{})
	})
	db, _ := sql.Open("stub", "")
	return db
}

func TestGenerateFiles(t *testing.T) {
	table := &Table{
		Schema: "mda",
		Name:   "demo_table",
		Columns: []Column{
			{Name: "id", RawType: "uuid", NotNull: true},
			{Name: "name", RawType: "varchar(10)"},
			{Name: "year", RawType: "integer", IsGenerated: true, GenerationExpression: "(split_part((date)::text, '-'::text, 1))::integer"},
			{Name: "created_at", RawType: "timestamp"},
		},
	}
	modulePath := "gitlab.com/example/project"
	files, err := generateFiles(table, modulePath, nil, false)
	if err != nil {
		t.Fatalf("generateFiles returned error: %v", err)
	}
	if len(files) != 3 {
		t.Fatalf("expected 3 generated files, got %d", len(files))
	}
	fileMap := map[string]GeneratedFile{}
	for _, file := range files {
		fileMap[file.Path] = file
	}
	expectedModelPath := filepath.Join("modules", "demo_table", "model", "demo_table_model.go")
	expectedQueryPath := filepath.Join("modules", "demo_table", "demo_table.query.pg.go")
	expectedRepoPath := filepath.Join("modules", "demo_table", "demo_table.repo.pg.go")
	for _, path := range []string{expectedModelPath, expectedQueryPath, expectedRepoPath} {
		if _, ok := fileMap[path]; !ok {
			t.Fatalf("expected generated file at %s", path)
		}
	}
	modelFile := fileMap[expectedModelPath].Content
	if !strings.Contains(modelFile, "type DemoTableDto struct") {
		t.Fatalf("model file missing dto struct definition:\n%s", modelFile)
	}
	if !strings.Contains(modelFile, "*time.Time") {
		t.Fatalf("model file should use time for timestamp columns:\n%s", modelFile)
	}
	if strings.Contains(modelFile, "uuid.UUID") {
		t.Fatalf("model file should use string for uuid columns:\n%s", modelFile)
	}
	if !strings.Contains(modelFile, "type DemoTable struct") {
		t.Fatalf("model file missing struct definition:\n%s", modelFile)
	}
	if !strings.Contains(modelFile, `Id string `+"`"+`form:"id" json:"id" db:"id" validate:"required|uuid"`+"`") {
		t.Fatalf("model file should mark id as required uuid:\n%s", modelFile)
	}
	if strings.Contains(modelFile, "query:\"") {
		t.Fatalf("model file should not emit query tags:\n%s", modelFile)
	}
	if !strings.Contains(modelFile, "DemoTableDto") {
		t.Fatalf("model file missing dto embed:\n%s", modelFile)
	}
	if !strings.Contains(modelFile, "Year *int") {
		t.Fatalf("model file should expose generated columns:\n%s", modelFile)
	}
	if !strings.Contains(modelFile, "read model only") {
		t.Fatalf("model file should mark generated columns:\n%s", modelFile)
	}
	if !strings.Contains(modelFile, "func (DemoTable) ConfigValidation") {
		t.Fatalf("model file should include ConfigValidation:\n%s", modelFile)
	}
	if !strings.Contains(modelFile, `shared.SceneCreate: []string{"Name"},`) {
		t.Fatalf("model file should include create scene fields:\n%s", modelFile)
	}
	if !strings.Contains(modelFile, `shared.SceneEdit:   []string{"Id"},`) {
		t.Fatalf("model file should include edit scene fields:\n%s", modelFile)
	}
	queryFile := fileMap[expectedQueryPath].Content
	if !strings.Contains(queryFile, "CountDemoTableQuery") || !strings.Contains(queryFile, "IndexDemoTableQuery") {
		t.Fatalf("query file missing constants:\n%s", queryFile)
	}
	if !strings.Contains(queryFile, "LIMIT $1 OFFSET $2") {
		t.Fatalf("query file should use postgres placeholders:\n%s", queryFile)
	}
	repoFile := fileMap[expectedRepoPath].Content
	if !strings.Contains(repoFile, "NewDemoTableRepo") {
		t.Fatalf("repository file missing constructor:\n%s", repoFile)
	}
	if !strings.Contains(repoFile, "table := \"mda.demo_table\"") {
		t.Fatalf("repository file missing schema-qualified table name:\n%s", repoFile)
	}
	if !strings.Contains(repoFile, fmt.Sprintf("\"%s/modules/demo_table/model\"", modulePath)) {
		t.Fatalf("repository file missing model import path:\n%s", repoFile)
	}
	if !strings.Contains(repoFile, "NewDemoTableRepo(driver db.Driver)") {
		t.Fatalf("repository file should accept db.Driver:\n%s", repoFile)
	}
	if !strings.Contains(repoFile, "csvc.Repository[model.DemoTableDto, model.DemoTable]") {
		t.Fatalf("repository file missing repository generic type:\n%s", repoFile)
	}
	if !strings.Contains(repoFile, "WithIndexQuery(CountDemoTableQuery, IndexDemoTableQuery)") {
		t.Fatalf("repository file should wire generated index queries into the repository builder:\n%s", repoFile)
	}
}

func TestGenerateFilesWithController(t *testing.T) {
	table := &Table{
		Schema: "public",
		Name:   "demo_table",
		Columns: []Column{
			{Name: "id", RawType: "uuid", NotNull: true},
			{Name: "name", RawType: "varchar(10)"},
			{Name: "status", RawType: "integer"},
			{Name: "created_at", RawType: "timestamp"},
			{Name: "updated_by", RawType: "uuid"},
		},
	}
	modulePath := "gitlab.com/example/project"
	files, err := generateFiles(table, modulePath, nil, true)
	if err != nil {
		t.Fatalf("generateFiles returned error: %v", err)
	}
	if len(files) != 5 {
		t.Fatalf("expected 5 generated files, got %d", len(files))
	}
	ctrlPath := filepath.Join("modules", "demo_table", "demo_table.ctrl.go")
	var ctrlFile GeneratedFile
	found := false
	for _, file := range files {
		if file.Path == ctrlPath {
			ctrlFile = file
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected generated file at %s", ctrlPath)
	}
	if !strings.Contains(ctrlFile.Content, "type DemoTableCtrl struct") {
		t.Fatalf("controller file missing DemoTableCtrl struct:\n%s", ctrlFile.Content)
	}
	if !strings.Contains(ctrlFile.Content, "func NewDemoTableUsecase") {
		t.Fatalf("controller file missing NewDemoTableUsecase:\n%s", ctrlFile.Content)
	}
	if !strings.Contains(ctrlFile.Content, "demoTable *DemoTableRepo") {
		t.Fatalf("controller file should hold a concrete repo:\n%s", ctrlFile.Content)
	}
	if strings.Contains(ctrlFile.Content, "shared.Validate.Struct") {
		t.Fatalf("controller file should not perform shared.Validate.Struct validation:\n%s", ctrlFile.Content)
	}
	if !strings.Contains(ctrlFile.Content, "editable := model.DemoTableDto{") {
		t.Fatalf("controller file missing editable dto patch payload:\n%s", ctrlFile.Content)
	}
	if !strings.Contains(ctrlFile.Content, "Id: form.Id,") {
		t.Fatalf("controller file should include id in editable payload:\n%s", ctrlFile.Content)
	}
	if !strings.Contains(ctrlFile.Content, "Name: form.Name,") {
		t.Fatalf("controller file should include non-audit fields in editable payload:\n%s", ctrlFile.Content)
	}
	if strings.Contains(ctrlFile.Content, "Status: form.Status,") || strings.Contains(ctrlFile.Content, "CreatedAt: form.CreatedAt,") || strings.Contains(ctrlFile.Content, "UpdatedBy: form.UpdatedBy,") {
		t.Fatalf("controller file should exclude status and audit fields from editable payload:\n%s", ctrlFile.Content)
	}
	if !strings.Contains(ctrlFile.Content, "Repository().DTO().PatchRow(ctx, editable, nil)") {
		t.Fatalf("controller file should patch editable dto:\n%s", ctrlFile.Content)
	}

	httpPath := filepath.Join("modules", "demo_table", "demo_table.http.go")
	found = false
	for _, file := range files {
		if file.Path == httpPath {
			if !strings.Contains(file.Content, "func IndexDemoTable") {
				t.Fatalf("http file missing IndexDemoTable:\n%s", file.Content)
			}
			if !strings.Contains(file.Content, "func SaveDemoTable") {
				t.Fatalf("http file missing SaveDemoTable:\n%s", file.Content)
			}
			if !strings.Contains(file.Content, "form := model.DemoTable{}") {
				t.Fatalf("http file should bind into the model struct:\n%s", file.Content)
			}
			if !strings.Contains(file.Content, "api.BindValidate(c, &form, shared.SceneCreate)") {
				t.Fatalf("http file should validate create with scenes:\n%s", file.Content)
			}
			if !strings.Contains(file.Content, "api.BindValidate(c, &form, shared.SceneEdit)") {
				t.Fatalf("http file should validate edit with scenes:\n%s", file.Content)
			}
			if !strings.Contains(file.Content, "form.DemoTableDto") {
				t.Fatalf("http file should pass dto payload from the model wrapper:\n%s", file.Content)
			}
			if !strings.Contains(file.Content, `serror.New("invalid id, expected to be a uuid")`) {
				t.Fatalf("http file should validate uuid route params:\n%s", file.Content)
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected generated file at %s", httpPath)
	}
}

func TestGenerateFilesWithoutController(t *testing.T) {
	table := &Table{
		Schema: "public",
		Name:   "demo_table",
		Columns: []Column{
			{Name: "id", RawType: "uuid", NotNull: true},
		},
	}
	modulePath := "gitlab.com/example/project"
	files, err := generateFiles(table, modulePath, nil, false)
	if err != nil {
		t.Fatalf("generateFiles returned error: %v", err)
	}
	if len(files) != 3 {
		t.Fatalf("expected 3 generated files, got %d", len(files))
	}
	ctrlPath := filepath.Join("modules", "demo_table", "demo_table.ctrl.go")
	for _, file := range files {
		if file.Path == ctrlPath {
			t.Fatalf("did not expect generated file at %s", ctrlPath)
		}
	}
	httpPath := filepath.Join("modules", "demo_table", "demo_table.http.go")
	for _, file := range files {
		if file.Path == httpPath {
			t.Fatalf("did not expect generated file at %s", httpPath)
		}
	}
}

func TestGenerateFilesUseCurrentModuleNaming(t *testing.T) {
	table := &Table{
		Schema: "usm",
		Name:   "usm_menus",
		Columns: []Column{
			{Name: "id", RawType: "uuid", NotNull: true},
		},
	}
	modulePath := "gitlab.com/example/project"
	files, err := generateFiles(table, modulePath, nil, true)
	if err != nil {
		t.Fatalf("generateFiles returned error: %v", err)
	}

	paths := map[string]GeneratedFile{}
	for _, file := range files {
		paths[file.Path] = file
	}

	modelPath := filepath.Join("modules", "menu", "model", "menu_model.go")
	if _, ok := paths[modelPath]; !ok {
		t.Fatalf("expected generated file at %s", modelPath)
	}
	if !strings.Contains(paths[modelPath].Content, "type MenuDto struct") {
		t.Fatalf("expected MenuDto struct:\n%s", paths[modelPath].Content)
	}

	repoPath := filepath.Join("modules", "menu", "menu.repo.pg.go")
	if _, ok := paths[repoPath]; !ok {
		t.Fatalf("expected generated file at %s", repoPath)
	}
	if !strings.Contains(paths[repoPath].Content, "type MenuRepo struct") {
		t.Fatalf("expected MenuRepo struct:\n%s", paths[repoPath].Content)
	}
}

func TestModulePathFromGoMod(t *testing.T) {
	tmpDir := t.TempDir()
	goModPath := filepath.Join(tmpDir, "go.mod")
	content := "module example.com/demo\n\ngo 1.21\n"
	if err := os.WriteFile(goModPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	subDir := filepath.Join(tmpDir, "nested")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("mkdir nested dir: %v", err)
	}

	mod, err := modulePathFromGoMod(subDir)
	if err != nil {
		t.Fatalf("modulePathFromGoMod returned error: %v", err)
	}
	if mod != "example.com/demo" {
		t.Fatalf("unexpected module path: %s", mod)
	}
}

func TestLoadDTOFieldTags(t *testing.T) {
	tmpDir := t.TempDir()
	modelDir := filepath.Join(tmpDir, "modules", "demo_table", "model")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatalf("mkdir model dir: %v", err)
	}
	content := "package model\n\ntype DemoTableDto struct {\n\tId string `form:\"id\" json:\"id\" db:\"id\" query:\"id\" validate:\"omitempty,uuid\"`\n\tName string `form:\"name\" json:\"name,omitempty\" db:\"name\"`\n}\n"
	modelPath := filepath.Join(modelDir, "demo_table_model.go")
	if err := os.WriteFile(modelPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write model file: %v", err)
	}

	table := &Table{
		Name: "demo_table",
		Columns: []Column{
			{Name: "id", RawType: "uuid", NotNull: true},
			{Name: "name", RawType: "varchar(10)"},
		},
	}
	tags, err := loadDTOFieldTags(tmpDir, table)
	if err != nil {
		t.Fatalf("loadDTOFieldTags returned error: %v", err)
	}
	if tags == nil || tags["Id"]["validate"] != "omitempty,uuid" {
		t.Fatalf("validate tag not loaded: %#v", tags)
	}
	if tags["Name"]["json"] != "name,omitempty" {
		t.Fatalf("json tag not loaded: %#v", tags["Name"])
	}

	modelFile := buildModelFile(table, tags)
	if !strings.Contains(modelFile, "validate:\"required|uuid\"") {
		t.Fatalf("model file should normalize id validate tag:\n%s", modelFile)
	}
	if !strings.Contains(modelFile, "json:\"name,omitempty\"") {
		t.Fatalf("model file missing preserved json tag:\n%s", modelFile)
	}
	if strings.Contains(modelFile, "query:\"") {
		t.Fatalf("model file should not contain query tags:\n%s", modelFile)
	}
}

func TestUpdateRegistryFiles(t *testing.T) {
	tmpDir := t.TempDir()
	table := &Table{
		Schema: "usm",
		Name:   "usm_menus",
		Columns: []Column{
			{Name: "id", RawType: "uuid", NotNull: true},
		},
	}
	modulePath := "gitlab.com/example/project"

	files, err := updateRegistryFiles(tmpDir, modulePath, table, true)
	if err != nil {
		t.Fatalf("updateRegistryFiles returned error: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("expected no registry files, got %d", len(files))
	}
}

func TestRenderGeneratedFileAppendAfterImport(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "sample.go")
	existing := "package demo\n\nimport \"fmt\"\n\ntype Existing struct{}\n"
	if err := os.WriteFile(targetPath, []byte(existing), 0o644); err != nil {
		t.Fatalf("write existing file: %v", err)
	}

	generated := "package demo\n\ntype Added struct{}\n"
	file := GeneratedFile{
		Path:    "sample.go",
		Content: generated,
		Targets: []DeclTarget{{Kind: token.TYPE, Name: "Added"}},
	}
	content, shouldWrite, err := renderGeneratedFile(config{}, targetPath, file, generated)
	if err != nil {
		t.Fatalf("renderGeneratedFile returned error: %v", err)
	}
	if !shouldWrite {
		t.Fatalf("expected renderGeneratedFile to request write")
	}
	if !strings.Contains(content, "type Added struct{}") {
		t.Fatalf("expected appended type in content:\n%s", content)
	}
	if strings.Index(content, "type Added") < strings.Index(content, "import") {
		t.Fatalf("expected added type after import:\n%s", content)
	}
}

func TestRenderGeneratedFileForceRewritesExistingGeneratedFile(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "sample.go")
	existing := "package demo\n\ntype Added struct{ Old string }\n"
	if err := os.WriteFile(targetPath, []byte(existing), 0o644); err != nil {
		t.Fatalf("write existing file: %v", err)
	}

	generated := "package demo\n\ntype Added struct{ New string }\n"
	file := GeneratedFile{
		Path:    "sample.go",
		Content: generated,
		Targets: []DeclTarget{{Kind: token.TYPE, Name: "Added"}},
	}
	content, shouldWrite, err := renderGeneratedFile(config{force: true}, targetPath, file, generated)
	if err != nil {
		t.Fatalf("renderGeneratedFile returned error: %v", err)
	}
	if !shouldWrite {
		t.Fatalf("expected renderGeneratedFile to rewrite generated file when force is enabled")
	}
	if !strings.Contains(content, "New string") || strings.Contains(content, "Old string") {
		t.Fatalf("expected forced rewrite content:\n%s", content)
	}
}

func TestRenderGeneratedFileForceRewritesWholeFile(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "sample.go")
	existing := "package demo\n\nimport \"fmt\"\n\ntype Existing struct{}\n"
	if err := os.WriteFile(targetPath, []byte(existing), 0o644); err != nil {
		t.Fatalf("write existing file: %v", err)
	}

	generated := "package demo\n\ntype Added struct{}\n"
	file := GeneratedFile{
		Path:    "sample.go",
		Content: generated,
		Targets: []DeclTarget{{Kind: token.TYPE, Name: "Added"}},
	}
	content, shouldWrite, err := renderGeneratedFile(config{force: true}, targetPath, file, generated)
	if err != nil {
		t.Fatalf("renderGeneratedFile returned error: %v", err)
	}
	if !shouldWrite {
		t.Fatalf("expected renderGeneratedFile to rewrite generated file when force is enabled")
	}
	if !strings.Contains(content, "type Added struct{}") || strings.Contains(content, "type Existing struct{}") {
		t.Fatalf("expected full-file rewrite content:\n%s", content)
	}
}

func TestPatchServiceHTTPRoutesAddsModuleImportAndBlock(t *testing.T) {
	tmpDir := t.TempDir()
	routeDir := filepath.Join(tmpDir, "service", "http")
	if err := os.MkdirAll(routeDir, 0o755); err != nil {
		t.Fatalf("mkdir route dir: %v", err)
	}

	existing := `package http

import (
	"gitlab.com/example/project/modules/runtime"
)

func (ox gatewayHandler) initRoute() {
	{
		controller := "/runtime"
		router := ox.service.Group(controller)
		router.GET("/ping", runtime.GetPing(ox.runtimeInfoUsecase))
	}
}
`
	if err := os.WriteFile(filepath.Join(routeDir, "route.go"), []byte(existing), 0o644); err != nil {
		t.Fatalf("write route.go: %v", err)
	}

	generatedHTTP := `package demo_table

func IndexDemoTable(ctrl *DemoTableCtrl) gin.HandlerFunc { return nil }
func SaveDemoTable(txb db.TrxBuilder) gin.HandlerFunc { return nil }
func ViewDemoTable(ctrl *DemoTableCtrl) gin.HandlerFunc { return nil }
func EditDemoTable(txb db.TrxBuilder) gin.HandlerFunc { return nil }
func DeleteDemoTable(txb db.TrxBuilder) gin.HandlerFunc { return nil }
`

	content, shouldWrite, err := patchServiceHTTPRoutes(filepath.Join(tmpDir, "service", "http", "route.go"), "gitlab.com/example/project", generatedHTTP)
	if err != nil {
		t.Fatalf("patchServiceHTTPRoutes returned error: %v", err)
	}
	if !shouldWrite {
		t.Fatalf("expected route patch to request write")
	}

	if !strings.Contains(content, `"gitlab.com/example/project/modules/demo_table"`) {
		t.Fatalf("expected module import in route.go:\n%s", content)
	}
	if !strings.Contains(content, `demoTableCtrl = demo_table.NewDemoTableUsecase(ox.driver)`) {
		t.Fatalf("expected controller var initialization:\n%s", content)
	}
	if !strings.Contains(content, `controller := "/demo-table"`) {
		t.Fatalf("expected module controller block:\n%s", content)
	}
	if !strings.Contains(content, `router.GET("/index", demo_table.IndexDemoTable(demoTableCtrl))`) {
		t.Fatalf("expected index route registration:\n%s", content)
	}
	if !strings.Contains(content, `router.POST("/add", demo_table.SaveDemoTable(ox.trxBuilder))`) {
		t.Fatalf("expected save route registration:\n%s", content)
	}
	if !strings.Contains(content, `router.DELETE("/delete/:id", demo_table.DeleteDemoTable(ox.trxBuilder))`) {
		t.Fatalf("expected delete route registration:\n%s", content)
	}
}

func TestPatchServiceHTTPRoutesAppendsMissingRoutesWithoutClobberingBlock(t *testing.T) {
	tmpDir := t.TempDir()
	routeDir := filepath.Join(tmpDir, "service", "http")
	if err := os.MkdirAll(routeDir, 0o755); err != nil {
		t.Fatalf("mkdir route dir: %v", err)
	}

	existing := `package http

import (
	"gitlab.com/example/project/modules/demo_table"
)

func (ox gatewayHandler) initRoute() {
	var (
		demoTableCtrl = demo_table.NewDemoTableUsecase(ox.driver)
	)
	{
		ctrl := "/demo_table"
		router := ox.service.Group(ctrl)
		router.GET("/index", demo_table.IndexDemoTable(demoTableCtrl))
		// keep this manual line
	}
}
`
	if err := os.WriteFile(filepath.Join(routeDir, "route.go"), []byte(existing), 0o644); err != nil {
		t.Fatalf("write route.go: %v", err)
	}

	generatedHTTP := `package demo_table

func IndexDemoTable(ctrl *DemoTableCtrl) gin.HandlerFunc { return nil }
func SaveDemoTable(txb db.TrxBuilder) gin.HandlerFunc { return nil }
func ViewDemoTable(ctrl *DemoTableCtrl) gin.HandlerFunc { return nil }
func EditDemoTable(txb db.TrxBuilder) gin.HandlerFunc { return nil }
func DeleteDemoTable(txb db.TrxBuilder) gin.HandlerFunc { return nil }
`

	content, shouldWrite, err := patchServiceHTTPRoutes(filepath.Join(tmpDir, "service", "http", "route.go"), "gitlab.com/example/project", generatedHTTP)
	if err != nil {
		t.Fatalf("patchServiceHTTPRoutes returned error: %v", err)
	}
	if !shouldWrite {
		t.Fatalf("expected route patch to request write")
	}

	if strings.Count(content, `router.GET("/index", demo_table.IndexDemoTable(demoTableCtrl))`) != 1 {
		t.Fatalf("expected index route to remain unique:\n%s", content)
	}
	if !strings.Contains(content, `ctrl := "/demo-table"`) {
		t.Fatalf("expected existing block controller path to be normalized:\n%s", content)
	}
	if !strings.Contains(content, `// keep this manual line`) {
		t.Fatalf("expected manual comment to be preserved:\n%s", content)
	}
	if !strings.Contains(content, `router.GET("/view/:id", demo_table.ViewDemoTable(demoTableCtrl))`) {
		t.Fatalf("expected view route registration:\n%s", content)
	}
	if !strings.Contains(content, `router.PUT("/edit", demo_table.EditDemoTable(ox.trxBuilder))`) {
		t.Fatalf("expected edit route registration:\n%s", content)
	}
	if !strings.Contains(content, `router.DELETE("/delete/:id", demo_table.DeleteDemoTable(ox.trxBuilder))`) {
		t.Fatalf("expected delete route registration:\n%s", content)
	}
}

func TestPatchServiceHTTPRoutesSupportsCustomRoutePath(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "internal", "http", "route.go")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		t.Fatalf("mkdir route dir: %v", err)
	}

	existing := `package http

func (ox gatewayHandler) initRoute() {
}
`
	if err := os.WriteFile(targetPath, []byte(existing), 0o644); err != nil {
		t.Fatalf("write route.go: %v", err)
	}

	generatedHTTP := `package demo_table

func IndexDemoTable(ctrl *DemoTableCtrl) gin.HandlerFunc { return nil }
`

	content, shouldWrite, err := patchServiceHTTPRoutes(targetPath, "gitlab.com/example/project", generatedHTTP)
	if err != nil {
		t.Fatalf("patchServiceHTTPRoutes returned error: %v", err)
	}
	if !shouldWrite {
		t.Fatalf("expected route patch to request write")
	}
	if !strings.Contains(content, `controller := "/demo-table"`) {
		t.Fatalf("expected module controller block:\n%s", content)
	}
	if !strings.Contains(content, `demoTableCtrl = demo_table.NewDemoTableUsecase(ox.driver)`) {
		t.Fatalf("expected controller var initialization:\n%s", content)
	}
	if !strings.Contains(content, `router.GET("/index", demo_table.IndexDemoTable(demoTableCtrl))`) {
		t.Fatalf("expected index route registration:\n%s", content)
	}
}

func TestPatchServiceHTTPHandlerNoop(t *testing.T) {
	content, shouldWrite, err := patchServiceHTTPHandler(t.TempDir(), `package demo_table`)
	if err != nil {
		t.Fatalf("patchServiceHTTPHandler returned error: %v", err)
	}
	if shouldWrite || content != "" {
		t.Fatalf("expected handler patch to be a no-op, got write=%v content=%q", shouldWrite, content)
	}
}

func TestPatchHTTPServicesConfigNoop(t *testing.T) {
	content, shouldWrite, err := patchHTTPServicesConfig(t.TempDir(), "gitlab.com/example/project", `package demo_table`)
	if err != nil {
		t.Fatalf("patchHTTPServicesConfig returned error: %v", err)
	}
	if shouldWrite || content != "" {
		t.Fatalf("expected services config patch to be a no-op, got write=%v content=%q", shouldWrite, content)
	}
}

func TestBuildPostmanCollectionCreatesStandaloneCollectionAndPreservesMetadata(t *testing.T) {
	useFixedPostmanMetadata(t)

	table := &Table{
		Schema: "public",
		Name:   "demo_table",
		Columns: []Column{
			{Name: "id", RawType: "uuid", NotNull: true, IsPrimary: true},
			{Name: "name", RawType: "varchar(50)"},
			{Name: "year", RawType: "integer", IsGenerated: true},
			{Name: "month", RawType: "integer", IsGenerated: true},
			{Name: "status", RawType: "integer"},
		},
	}

	spec, err := inferHTTPRouteSpec(buildHTTPFile(table, "gitlab.com/example/project"), "")
	if err != nil {
		t.Fatalf("inferHTTPRouteSpec returned error: %v", err)
	}

	content, changed, err := buildPostmanCollection("", table, spec, "")
	if err != nil {
		t.Fatalf("buildPostmanCollection returned error: %v", err)
	}
	if !changed {
		t.Fatalf("expected collection to change")
	}

	var collection postmanCollection
	if err = json.Unmarshal([]byte(content), &collection); err != nil {
		t.Fatalf("unmarshal updated collection: %v", err)
	}

	if len(collection.Item) != 1 {
		t.Fatalf("expected standalone collection with one top-level folder, got %d", len(collection.Item))
	}
	folder := collection.Item[0]
	if folder.Name != "DEMO TABLE" {
		t.Fatalf("unexpected folder name: %s", folder.Name)
	}
	if len(folder.Item) != 5 {
		t.Fatalf("expected 5 generated requests, got %d", len(folder.Item))
	}

	index := folder.Item[findPostmanItemIndex(folder.Item, "index")]
	if index.Request == nil || index.Request.URL.Raw != "{{gateway}}/demo-table/index?page=1&size=10" {
		t.Fatalf("unexpected index request url: %#v", index.Request)
	}

	add := folder.Item[findPostmanItemIndex(folder.Item, "add")]
	if add.Request == nil || add.Request.Body == nil || add.Request.Body.Mode != "formdata" {
		t.Fatalf("expected add request formdata body: %#v", add.Request)
	}
	if postmanFormdataValue(add.Request.Body.Formdata, "id") != "" {
		t.Fatalf("did not expect add body to include id: %#v", add.Request.Body.Formdata)
	}
	if postmanFormdataValue(add.Request.Body.Formdata, "name") != "name" {
		t.Fatalf("expected generated add body field for name: %#v", add.Request.Body.Formdata)
	}
	for _, key := range []string{"year", "month"} {
		if postmanFormdataValue(add.Request.Body.Formdata, key) != "" {
			t.Fatalf("did not expect generated field %s in add body: %#v", key, add.Request.Body.Formdata)
		}
	}
	for _, key := range []string{"created_by", "created_at", "updated_by", "updated_at"} {
		if postmanFormdataValue(add.Request.Body.Formdata, key) != "" {
			t.Fatalf("did not expect audit field %s in add body: %#v", key, add.Request.Body.Formdata)
		}
	}

	edit := folder.Item[findPostmanItemIndex(folder.Item, "edit")]
	if postmanFormdataValue(edit.Request.Body.Formdata, "id") != sampleUUIDValue() {
		t.Fatalf("expected edit body to include id: %#v", edit.Request.Body.Formdata)
	}
	if postmanFormdataCount(edit.Request.Body.Formdata, "id") != 1 {
		t.Fatalf("expected edit body to include id once: %#v", edit.Request.Body.Formdata)
	}
	for _, key := range []string{"year", "month"} {
		if postmanFormdataValue(edit.Request.Body.Formdata, key) != "" {
			t.Fatalf("did not expect generated field %s in edit body: %#v", key, edit.Request.Body.Formdata)
		}
	}
	for _, key := range []string{"created_by", "created_at", "updated_by", "updated_at"} {
		if postmanFormdataValue(edit.Request.Body.Formdata, key) != "" {
			t.Fatalf("did not expect audit field %s in edit body: %#v", key, edit.Request.Body.Formdata)
		}
	}

	if findPostmanItemIndex(folder.Item, "update-status") != -1 {
		t.Fatalf("did not expect update-status request in generated folder")
	}

	view := folder.Item[findPostmanItemIndex(folder.Item, "view")]
	if len(view.Request.URL.Query) != 0 {
		t.Fatalf("expected view request to avoid template query placeholders: %#v", view.Request.URL.Query)
	}
	if index.Request.Description != nil || add.Request.Description != nil || edit.Request.Description != nil {
		t.Fatalf("expected template-specific request descriptions to be cleared")
	}
	if strings.Contains(content, "AAA") || strings.Contains(content, "company type") || strings.Contains(content, "company_id") {
		t.Fatalf("expected standalone collection to avoid office-specific placeholders:\n%s", content)
	}

	assertGeneratedPostmanInfo(t, collection.Info, "demo_table")
	if len(collection.Event) != 2 || collection.Event[0]["listen"] != "prerequest" || collection.Event[1]["listen"] != "test" {
		t.Fatalf("expected collection events to be preserved: %#v", collection.Event)
	}
	if len(collection.Variable) != 0 {
		t.Fatalf("did not expect root variables in extracted template: %#v", collection.Variable)
	}
	if len(collection.Auth) != 0 {
		t.Fatalf("did not expect root auth in extracted template: %#v", collection.Auth)
	}
}

func TestBuildPostmanCollectionPreservesExistingRequestEventAndExtras(t *testing.T) {
	useFixedPostmanMetadata(t)

	table := &Table{
		Schema: "public",
		Name:   "demo_table",
		Columns: []Column{
			{Name: "id", RawType: "uuid", NotNull: true, IsPrimary: true},
			{Name: "name", RawType: "varchar(50)"},
		},
	}

	spec, err := inferHTTPRouteSpec(buildHTTPFile(table, "gitlab.com/example/project"), "")
	if err != nil {
		t.Fatalf("inferHTTPRouteSpec returned error: %v", err)
	}

	collection := postmanCollection{
		Info: map[string]any{
			"name": "Generated Existing",
		},
		Item: []postmanItem{
			{
				Name: "DEMO TABLE",
				Item: []postmanItem{
					{
						Name: "add",
						Event: []map[string]any{
							{"listen": "prerequest"},
						},
						Request: &postmanRequest{
							Method: "POST",
							URL: postmanURL{
								Raw:  "{{gateway}}/demo-table/add",
								Host: []string{"{{gateway}}"},
								Path: []string{"demo-table", "add"},
							},
						},
					},
					{
						Name: "custom",
						Request: &postmanRequest{
							Method: "POST",
							URL: postmanURL{
								Raw:  "{{gateway}}/demo-table/custom",
								Host: []string{"{{gateway}}"},
								Path: []string{"demo-table", "custom"},
							},
						},
					},
				},
			},
		},
	}

	data, err := json.Marshal(collection)
	if err != nil {
		t.Fatalf("marshal collection: %v", err)
	}

	content, changed, err := buildPostmanCollection(string(data), table, spec, "")
	if err != nil {
		t.Fatalf("buildPostmanCollection returned error: %v", err)
	}
	if !changed {
		t.Fatalf("expected collection to change")
	}

	var updated postmanCollection
	if err = json.Unmarshal([]byte(content), &updated); err != nil {
		t.Fatalf("unmarshal updated collection: %v", err)
	}

	folder := updated.Item[0]

	add := folder.Item[findPostmanItemIndex(folder.Item, "add")]
	if len(add.Event) != 1 || add.Event[0]["listen"] != "prerequest" {
		t.Fatalf("expected add request event to be preserved: %#v", add.Event)
	}

	if findPostmanItemIndex(folder.Item, "custom") == -1 {
		t.Fatalf("expected custom request to be preserved")
	}
	assertGeneratedPostmanInfo(t, updated.Info, "demo_table")
}

func TestBuildPostmanCollectionPrependsURLPathWhenConfigured(t *testing.T) {
	useFixedPostmanMetadata(t)

	table := &Table{
		Schema: "public",
		Name:   "demo_table",
		Columns: []Column{
			{Name: "id", RawType: "uuid", NotNull: true, IsPrimary: true},
			{Name: "name", RawType: "varchar(50)"},
		},
	}

	spec, err := inferHTTPRouteSpec(buildHTTPFile(table, "gitlab.com/example/project"), "")
	if err != nil {
		t.Fatalf("inferHTTPRouteSpec returned error: %v", err)
	}

	content, changed, err := buildPostmanCollection("", table, spec, "/erp/hkhd/")
	if err != nil {
		t.Fatalf("buildPostmanCollection returned error: %v", err)
	}
	if !changed {
		t.Fatalf("expected collection to change")
	}

	var collection postmanCollection
	if err = json.Unmarshal([]byte(content), &collection); err != nil {
		t.Fatalf("unmarshal updated collection: %v", err)
	}

	index := collection.Item[0].Item[findPostmanItemIndex(collection.Item[0].Item, "index")]
	if index.Request == nil {
		t.Fatalf("expected index request to exist")
	}
	if index.Request.URL.Raw != "{{gateway}}/erp/hkhd/demo-table/index?page=1&size=10" {
		t.Fatalf("unexpected index request url: %#v", index.Request.URL)
	}
	if strings.Join(index.Request.URL.Path, "/") != "erp/hkhd/demo-table/index" {
		t.Fatalf("unexpected index request path: %#v", index.Request.URL.Path)
	}
}

func TestPatchPostmanCollectionUsesBuiltInTemplateAndModuleNamedOutputFile(t *testing.T) {
	useFixedPostmanMetadata(t)

	tmpDir := t.TempDir()

	table := &Table{
		Schema: "public",
		Name:   "demo_table",
		Columns: []Column{
			{Name: "id", RawType: "uuid", NotNull: true, IsPrimary: true},
			{Name: "name", RawType: "varchar(50)"},
		},
	}

	fileName, content, changed, err := patchPostmanCollection(tmpDir, "gitlab.com/example/project", table, "")
	if err != nil {
		t.Fatalf("patchPostmanCollection returned error: %v", err)
	}
	if !changed {
		t.Fatalf("expected generated collection to request write")
	}
	if fileName != "demo_table.collections.json" {
		t.Fatalf("unexpected output file name: %s", fileName)
	}
	if strings.Contains(content, `"name": "MASTER"`) {
		t.Fatalf("did not expect template section wrapper in standalone output:\n%s", content)
	}

	var collection postmanCollection
	if err := json.Unmarshal([]byte(content), &collection); err != nil {
		t.Fatalf("unmarshal generated collection: %v", err)
	}
	assertGeneratedPostmanInfo(t, collection.Info, "demo_table")
}

func TestPatchPostmanCollectionReadsExistingCollectionFromPostmanDir(t *testing.T) {
	useFixedPostmanMetadata(t)

	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "postman", "demo_table.collections.json")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		t.Fatalf("mkdir postman dir: %v", err)
	}

	existing := `{
  "info": {
    "name": "Generated Existing",
    "_postman_id": "99999999-8888-4777-8666-555555555555",
    "_exporter_id": "123456",
    "_collection_link": "https://go.postman.co/collection/37991928-99999999-8888-4777-8666-555555555555?source=collection_link"
  },
  "item": [
    {
      "name": "DEMO TABLE",
      "item": [
        {
          "name": "custom",
          "request": {
            "method": "POST",
            "url": {
              "raw": "{{gateway}}/demo-table/custom",
              "host": ["{{gateway}}"],
              "path": ["demo-table", "custom"]
            }
          }
        }
      ]
    }
  ]
}`
	if err := os.WriteFile(targetPath, []byte(existing), 0o644); err != nil {
		t.Fatalf("write existing collection: %v", err)
	}

	table := &Table{
		Schema: "public",
		Name:   "demo_table",
		Columns: []Column{
			{Name: "id", RawType: "uuid", NotNull: true, IsPrimary: true},
			{Name: "name", RawType: "varchar(50)"},
		},
	}

	_, content, changed, err := patchPostmanCollection(tmpDir, "gitlab.com/example/project", table, "")
	if err != nil {
		t.Fatalf("patchPostmanCollection returned error: %v", err)
	}
	if !changed {
		t.Fatalf("expected collection to change")
	}

	var collection postmanCollection
	if err := json.Unmarshal([]byte(content), &collection); err != nil {
		t.Fatalf("unmarshal generated collection: %v", err)
	}
	if findPostmanItemIndex(collection.Item[0].Item, "custom") == -1 {
		t.Fatalf("expected custom request from postman dir collection to be preserved")
	}
}

func TestModuleNameForTablePreservesSimRecplanNotesPlural(t *testing.T) {
	if got := moduleNameForTable("sim_recplan_notes"); got != "sim_recplan_notes" {
		t.Fatalf("expected sim_recplan_notes module name to stay plural, got %q", got)
	}
}

func useFixedPostmanMetadata(t *testing.T) {
	t.Helper()

	oldNow := postmanNow
	oldNewUUID := postmanNewUUID
	oldNewExporterID := postmanNewExporterID

	postmanNow = func() time.Time {
		return time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)
	}
	postmanNewUUID = func() string {
		return "11111111-2222-4333-8444-555555555555"
	}
	postmanNewExporterID = func() (string, error) {
		return "12345678", nil
	}

	t.Cleanup(func() {
		postmanNow = oldNow
		postmanNewUUID = oldNewUUID
		postmanNewExporterID = oldNewExporterID
	})
}

func assertGeneratedPostmanInfo(t *testing.T, info map[string]any, name string) {
	t.Helper()

	if info["name"] != name+" 2026-04-18" {
		t.Fatalf("expected generated name suffix: %#v", info)
	}
	if info["_postman_id"] != "11111111-2222-4333-8444-555555555555" {
		t.Fatalf("expected generated _postman_id: %#v", info)
	}
	if info["_exporter_id"] != "12345678" {
		t.Fatalf("expected generated _exporter_id: %#v", info)
	}
	if info["_collection_link"] != "https://go.postman.co/collection/37991928-11111111-2222-4333-8444-555555555555?source=collection_link" {
		t.Fatalf("expected generated _collection_link: %#v", info)
	}
	if info["schema"] != "https://schema.getpostman.com/json/collection/v2.1.0/collection.json" {
		t.Fatalf("expected schema metadata to be preserved: %#v", info)
	}
}

func postmanFormdataValue(formdata []map[string]any, key string) string {
	for _, field := range formdata {
		if fieldKey, _ := field["key"].(string); fieldKey == key {
			value, _ := field["value"].(string)
			return value
		}
	}
	return ""
}

func postmanFormdataCount(formdata []map[string]any, key string) int {
	count := 0
	for _, field := range formdata {
		if fieldKey, _ := field["key"].(string); fieldKey == key {
			count++
		}
	}
	return count
}
