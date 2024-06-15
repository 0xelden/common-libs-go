package main

import (
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-errors/errors"
)

type GeneratedFile struct {
	Path     string
	Content  string
	Targets  []DeclTarget
	SkipHard bool
}

type DeclTarget struct {
	Kind token.Token
	Name string
}

func generateFiles(table *Table, modulePath string, dtoTags map[string]map[string]string, withCtl bool) ([]GeneratedFile, error) {
	moduleName := moduleNameForTable(table.Name)
	structName := structNameForTable(table.Name)

	dtoFile, err := buildDTOFile(table, dtoTags)
	if err != nil {
		return nil, err
	}

	files := []GeneratedFile{
		{
			Path:    filepath.Join("modules", moduleName, "dto", fmt.Sprintf("%s_dto.go", moduleName)),
			Content: dtoFile,
			Targets: []DeclTarget{{Kind: token.TYPE, Name: structName + "Dto"}},
		},
		{
			Path:    filepath.Join("modules", moduleName, "model", fmt.Sprintf("%s_model.go", moduleName)),
			Content: buildModelFile(table, modulePath),
			Targets: []DeclTarget{{Kind: token.TYPE, Name: structName}},
		},
		{
			Path:    filepath.Join("modules", moduleName, fmt.Sprintf("%s.query.pg.go", moduleName)),
			Content: buildQueryFile(table),
			Targets: []DeclTarget{
				{Kind: token.CONST, Name: "Count" + structName + "Query"},
				{Kind: token.CONST, Name: "Index" + structName + "Query"},
			},
		},
		{
			Path:    filepath.Join("modules", moduleName, fmt.Sprintf("%s.repo.pg.go", moduleName)),
			Content: buildRepositoryFile(table, modulePath),
			Targets: []DeclTarget{
				{Kind: token.TYPE, Name: structName + "Repo"},
				{Kind: token.FUNC, Name: "New" + structName + "Repo"},
			},
		},
	}

	if withCtl {
		files = append(files,
			GeneratedFile{
				Path:    filepath.Join("modules", moduleName, fmt.Sprintf("%s.ctrl.go", moduleName)),
				Content: buildControllerFile(table, modulePath),
				Targets: []DeclTarget{
					{Kind: token.TYPE, Name: structName + "Ctrl"},
					{Kind: token.FUNC, Name: "New" + structName + "Usecase"},
				},
			},
			GeneratedFile{
				Path:    filepath.Join("modules", moduleName, fmt.Sprintf("%s.http.go", moduleName)),
				Content: buildHTTPFile(table, modulePath),
				Targets: []DeclTarget{
					{Kind: token.FUNC, Name: "Index" + structName},
					{Kind: token.FUNC, Name: "Save" + structName},
				},
			},
		)
	}

	return files, nil
}

func buildDTOFile(table *Table, existingTags map[string]map[string]string) (string, error) {
	structName := structNameForTable(table.Name)
	imports := map[string]struct{}{}
	fields := make([]string, 0, len(table.Columns))

	for _, col := range table.Columns {
		if col.IsGenerated {
			continue
		}

		goType := buildGoType(col)
		if goType.ImportPath != "" {
			imports[goType.ImportPath] = struct{}{}
		}

		fieldType := goType.TypeName
		if shouldUsePointer(col) {
			fieldType = "*" + fieldType
		}

		fieldName := pascalCase(col.Name)
		tag := mergeDTOStructTags(col.Name, existingTags[fieldName])
		fields = append(fields, fmt.Sprintf("\t%s %s %s", fieldName, fieldType, tag))
	}

	var builder strings.Builder
	builder.WriteString("package dto\n\n")
	writeImportBlock(&builder, imports)
	builder.WriteString(fmt.Sprintf("type %sDto struct {\n", structName))
	if len(fields) > 0 {
		builder.WriteString(strings.Join(fields, "\n"))
		builder.WriteString("\n")
	}
	builder.WriteString("}\n")

	return builder.String(), nil
}

func buildModelFile(table *Table, modulePath string) string {
	structName := structNameForTable(table.Name)
	moduleName := moduleNameForTable(table.Name)
	imports := map[string]struct{}{
		filepath.ToSlash(filepath.Join(modulePath, "modules", moduleName, "dto")): {},
	}
	fields := make([]string, 0, len(table.Columns))

	for _, col := range table.Columns {
		if !col.IsGenerated {
			continue
		}

		goType := buildGoType(col)
		if goType.ImportPath != "" {
			imports[goType.ImportPath] = struct{}{}
		}

		fieldType := goType.TypeName
		if shouldUsePointer(col) {
			fieldType = "*" + fieldType
		}

		fields = append(fields, fmt.Sprintf("\t%s %s %s", pascalCase(col.Name), fieldType, mergeDTOStructTags(col.Name, nil)))
	}

	var builder strings.Builder
	builder.WriteString("package model\n\n")
	writeImportBlock(&builder, imports)
	builder.WriteString(fmt.Sprintf("type %s struct {\n", structName))
	builder.WriteString(fmt.Sprintf("\tdto.%sDto\n", structName))
	if len(fields) > 0 {
		builder.WriteString("\n")
		builder.WriteString("\t// Database-generated columns are exposed on the read model only.\n")
		builder.WriteString(strings.Join(fields, "\n"))
		builder.WriteString("\n")
	}
	builder.WriteString("}\n")

	return builder.String()
}

func buildQueryFile(table *Table) string {
	structName := structNameForTable(table.Name)
	packageName := moduleNameForTable(table.Name)
	schemaQualified := table.Name
	if table.Schema != "" {
		schemaQualified = fmt.Sprintf("%s.%s", table.Schema, table.Name)
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("package %s\n\n", packageName))
	builder.WriteString("const (\n")
	fmt.Fprintf(&builder, "\tCount%sQuery = `\n", structName)
	builder.WriteString("\t\tSELECT COUNT(*) AS total \n")
	fmt.Fprintf(&builder, "\t\t  FROM %s result\n", schemaQualified)
	builder.WriteString("\t\t---%s -- filter handle\n")
	builder.WriteString("\t;`\n\n")

	fmt.Fprintf(&builder, "\tIndex%sQuery = `\n", structName)
	builder.WriteString("\t\tWITH rows as (\n")
	builder.WriteString("\t\t\tSELECT result.*\n")
	fmt.Fprintf(&builder, "\t\t    FROM %s result\n", schemaQualified)
	builder.WriteString("\t\t\t---%s -- filter handle\n")
	builder.WriteString("\t\t)\n")
	builder.WriteString("\t\tSELECT *\n")
	builder.WriteString("\t\tFROM \"rows\"\n")
	builder.WriteString("\t\t---%s -- sorter handle\n")
	builder.WriteString("\t\tLIMIT $1 OFFSET $2\n")
	builder.WriteString("\t;`\n")
	builder.WriteString(")\n")

	return builder.String()
}

func buildRepositoryFile(table *Table, modulePath string) string {
	structName := structNameForTable(table.Name)
	moduleName := moduleNameForTable(table.Name)
	interfaceName := repoInterfaceName(table.Name)
	schemaQualified := table.Name
	if table.Schema != "" {
		schemaQualified = fmt.Sprintf("%s.%s", table.Schema, table.Name)
	}

	template := `package %[1]s

import (
	"github.com/0xelden/common-libs-go/db"
	csvc "github.com/0xelden/common-libs-go/service"
	"github.com/0xelden/common-libs-go/service/repository/general"
	"%[2]s/modules/%[1]s/dto"
	"%[2]s/modules/%[1]s/model"
	"%[2]s/service"
)

var _ service.%[3]s = &%[4]sRepo{}

//goland:noinspection GoNameStartsWithPackageName
type %[4]sRepo struct {
	table  string
	driver db.Driver
	gen    csvc.GeneralRepo
	dto    csvc.DTO[dto.%[4]sDto]
	repo   csvc.Repository[dto.%[4]sDto, model.%[4]s]
}

func New%[4]sRepo(driver db.Driver) (result *%[4]sRepo) {
	table := "%[5]s"
	gen := general.NewGeneralRepo(driver, table)
	result = &%[4]sRepo{
		driver: driver,
		gen:    gen,
		table:  table,
		dto:    general.NewDTO[dto.%[4]sDto](table, gen),
	}
	repo := general.
		NewRepositoryBuilder[dto.%[4]sDto, model.%[4]s](driver, table).
		WithIndexQuery(Count%[4]sQuery, Index%[4]sQuery).
		WithOverride(result).
		WithDtoInterface(result.dto).
		WithGeneralRepoInterface(gen).
		New()
	result.repo = repo
	return result
}

func (r *%[4]sRepo) Repository() csvc.Repository[dto.%[4]sDto, model.%[4]s] {
	return r.repo
}
`

	return fmt.Sprintf(template, moduleName, modulePath, interfaceName, structName, schemaQualified)
}

func buildControllerFile(table *Table, modulePath string) string {
	moduleName := moduleNameForTable(table.Name)
	structName := structNameForTable(table.Name)
	repoField := lowerCamelCase(structName)
	usecaseName := usecaseInterfaceName(table.Name)
	repoName := repoInterfaceName(table.Name)
	editableFields := buildEditableDTOFields(table)

	template := `package %[1]s

import (
	"context"

	"github.com/0xelden/common-libs-go/api"
	"github.com/0xelden/common-libs-go/db"
	"github.com/0xelden/common-libs-go/helper"
	"github.com/0xelden/common-libs-go/shared"
	"%[2]s/modules/%[1]s/dto"
	"%[2]s/modules/%[1]s/model"
	"%[2]s/service"
	"github.com/0xelden/common-libs-go/serror"
)

var _ service.%[3]s = &%[4]sCtrl{}

//goland:noinspection GoNameStartsWithPackageName
type %[4]sCtrl struct {
	%[5]s service.%[6]s
}

func New%[4]sUsecase(driver db.Driver) *%[4]sCtrl {
	return &%[4]sCtrl{
		%[5]s: New%[4]sRepo(driver),
	}
}

func (c *%[4]sCtrl) Index%[4]s(ctx context.Context, param api.IndexParam) (result api.RowIndex[model.%[4]s], serr serror.SError) {
	err := shared.Validate.Struct(&param)
	if err != nil {
		return result, serror.NewFromError(err)
	}
	return c.%[5]s.Repository().List(ctx, param)
}

func (c *%[4]sCtrl) Create%[4]s(ctx context.Context, form dto.%[4]sDto) (id string, serr serror.SError) {
	err := shared.Validate.Struct(&form)
	if err != nil {
		return id, serror.NewFromError(err)
	}

	res, serr := c.%[5]s.Repository().Insert(ctx, form)
	if serr != nil {
		return id, serr
	}

	return res.Id, nil
}

func (c *%[4]sCtrl) View%[4]s(ctx context.Context, param api.ViewParam) (result model.%[4]s, serr serror.SError) {
	err := shared.Validate.Struct(&param)
	if err != nil {
		return result, serror.NewFromError(err)
	}
	return c.%[5]s.Repository().Get(ctx, param)
}

func (c *%[4]sCtrl) Edit%[4]s(ctx context.Context, form dto.%[4]sDto) (affected int64, serr serror.SError) {
	err := shared.Validate.Struct(&form)
	if err != nil {
		return affected, serror.NewFromError(err)
	}
	editable := dto.%[4]sDto{
%[7]s
	}
	_, serr = c.%[5]s.Repository().DTO().PatchRow(ctx, editable, nil)
	if serr != nil {
		return affected, serr
	}
	return 1, nil
}

func (c *%[4]sCtrl) Delete%[4]s(ctx context.Context, id string) (affected int64, serr serror.SError) {
	if !helper.IsValidUUID(id) {
		return affected, serror.New("invalid id, expected to be a uuid")
	}
	return c.%[5]s.Repository().Remove(ctx, id)
}
`

	return fmt.Sprintf(template, moduleName, modulePath, usecaseName, structName, repoField, repoName, editableFields)
}

func buildEditableDTOFields(table *Table) string {
	fields := make([]string, 0, len(table.Columns))

	for _, col := range table.Columns {
		if !isIDColumn(col) || col.IsGenerated {
			continue
		}
		fieldName := pascalCase(col.Name)
		fields = append(fields, fmt.Sprintf("\t\t%s: form.%s,", fieldName, fieldName))
		break
	}

	for _, col := range table.Columns {
		if col.IsGenerated || isIDColumn(col) || isAuditColumn(col.Name) || strings.EqualFold(col.Name, "status") {
			continue
		}
		fieldName := pascalCase(col.Name)
		fields = append(fields, fmt.Sprintf("\t\t%s: form.%s,", fieldName, fieldName))
	}

	return strings.Join(fields, "\n")
}

func buildHTTPFile(table *Table, modulePath string) string {
	moduleName := moduleNameForTable(table.Name)
	structName := structNameForTable(table.Name)
	usecaseName := usecaseInterfaceName(table.Name)

	template := `package %[1]s

import (
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/0xelden/common-libs-go/api"
	"github.com/0xelden/common-libs-go/db"
	"github.com/0xelden/common-libs-go/shared"
	"%[2]s/modules/%[1]s/dto"
	"%[2]s/service"
)

func Index%[3]s(ctrl service.%[4]s) gin.HandlerFunc {
	return func(c *gin.Context) {
		param, err := api.NewIndexParam(c, binding.Query)
		if err != nil {
			api.Error(c, err)
			return
		}
		ctx := api.SetContext(c, shared.OmitTotal, c.Query(shared.OmitTotal))
		result, serr := ctrl.Index%[3]s(ctx, *param)
		if serr != nil {
			api.Error(c, serr)
			return
		}
		api.Success(c, result)
	}
}

func Save%[3]s(txb db.TrxBuilder) gin.HandlerFunc {
	return func(c *gin.Context) {
		form := dto.%[3]sDto{}
		if err := api.FormUnmarshal(c, &form); err != nil {
			api.Error(c, err)
			return
		}
		ctx, trx := api.SetCtxAndTrx(c, txb)
		result, serr := New%[3]sUsecase(trx).Create%[3]s(ctx, form)
		if serr != nil {
			api.Error(c, serr)
			return
		}
		api.Success(c, result)
	}
}

func View%[3]s(ctrl service.%[4]s) gin.HandlerFunc {
	return func(c *gin.Context) {
		param, err := api.NewViewParam(c, binding.Query, c.Param("id"))
		if err != nil {
			api.Error(c, err)
			return
		}
		ctx := api.SetContext(c)
		result, serr := ctrl.View%[3]s(ctx, *param)
		if serr != nil {
			api.Error(c, serr)
			return
		}
		api.Success(c, result)
	}
}

func Edit%[3]s(txb db.TrxBuilder) gin.HandlerFunc {
	return func(c *gin.Context) {
		form := dto.%[3]sDto{}
		if err := api.FormUnmarshal(c, &form); err != nil {
			api.Error(c, err)
			return
		}
		ctx, trx := api.SetCtxAndTrx(c, txb)
		result, serr := New%[3]sUsecase(trx).Edit%[3]s(ctx, form)
		if serr != nil {
			api.Error(c, serr)
			return
		}
		api.Success(c, result)
	}
}

func Delete%[3]s(txb db.TrxBuilder) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, trx := api.SetCtxAndTrx(c, txb)
		result, serr := New%[3]sUsecase(trx).Delete%[3]s(ctx, c.Param("id"))
		if serr != nil {
			api.Error(c, serr)
			return
		}
		api.Success(c, result)
	}
}
`

	return fmt.Sprintf(template, moduleName, modulePath, structName, usecaseName)
}

func writeImportBlock(builder *strings.Builder, imports map[string]struct{}) {
	if len(imports) == 0 {
		return
	}

	keys := sortedKeys(imports)
	builder.WriteString("import (\n")
	for _, imp := range keys {
		builder.WriteString(fmt.Sprintf("\t%q\n", imp))
	}
	builder.WriteString(")\n\n")
}

func lowerCamelCase(input string) string {
	if input == "" {
		return input
	}
	runes := []rune(input)
	runes[0] = []rune(strings.ToLower(string(runes[0])))[0]
	return string(runes)
}

type goType struct {
	TypeName   string
	ImportPath string
}

func buildGoType(col Column) goType {
	base := baseSQLType(col.RawType)
	switch {
	case strings.Contains(base, "uuid"):
		return goType{TypeName: "string"}
	case base == "bigint" || strings.Contains(base, "bigserial"):
		return goType{TypeName: "int64"}
	case strings.Contains(base, "smallint"):
		return goType{TypeName: "int"}
	case strings.Contains(base, "int"):
		return goType{TypeName: "int"}
	case strings.Contains(base, "serial"):
		return goType{TypeName: "int"}
	case strings.Contains(base, "float"), strings.Contains(base, "numeric"), strings.Contains(base, "decimal"), strings.Contains(base, "double"), strings.Contains(base, "real"):
		return goType{TypeName: "float64"}
	case strings.Contains(base, "bool"):
		return goType{TypeName: "bool"}
	case strings.Contains(base, "timestamp"), strings.Contains(base, "timestamptz"), strings.Contains(base, "datetime"), base == "date", base == "time", base == "timetz":
		return goType{TypeName: "time.Time", ImportPath: "time"}
	case strings.Contains(base, "json"):
		return goType{TypeName: "json.RawMessage", ImportPath: "encoding/json"}
	case strings.Contains(base, "bytea"):
		return goType{TypeName: "[]byte"}
	case strings.Contains(base, "text"), strings.Contains(base, "char"):
		return goType{TypeName: "string"}
	default:
		return goType{TypeName: "string"}
	}
}

func shouldUsePointer(col Column) bool {
	return !strings.EqualFold(col.Name, "id")
}

func baseSQLType(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	replacements := map[string]string{
		"character varying":           "varchar",
		"timestamp without time zone": "timestamp",
		"timestamp with time zone":    "timestamptz",
		"time without time zone":      "time",
		"time with time zone":         "timetz",
		"double precision":            "double",
	}
	if rep, ok := replacements[raw]; ok {
		return rep
	}
	if idx := strings.Index(raw, "("); idx != -1 {
		raw = raw[:idx]
	}
	raw = strings.TrimSpace(raw)
	if rep, ok := replacements[raw]; ok {
		return rep
	}
	return raw
}

func sortedKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func findGoModPath(startDir string) (string, error) {
	current := startDir
	for {
		path := filepath.Join(current, "go.mod")
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		} else if err != nil && !os.IsNotExist(err) {
			return "", errors.Errorf("stat go.mod in %s: %v", current, err)
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return "", errors.Errorf("go.mod not found from %s", startDir)
}

func modulePathFromGoMod(startDir string) (string, error) {
	goModPath, err := findGoModPath(startDir)
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(goModPath)
	if err != nil {
		return "", errors.Errorf("read go.mod: %v", err)
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}
		if strings.HasPrefix(trimmed, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "module ")), nil
		}
	}

	return "", errors.Errorf("module path not found in %s", goModPath)
}
