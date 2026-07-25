package main

import (
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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

	files := []GeneratedFile{
		{
			Path:    filepath.Join("modules", moduleName, "model", fmt.Sprintf("%s_model.go", moduleName)),
			Content: buildModelFile(table, dtoTags),
			Targets: []DeclTarget{
				{Kind: token.TYPE, Name: structName + "Dto"},
				{Kind: token.TYPE, Name: structName},
				{Kind: token.FUNC, Name: "ConfigValidation"},
			},
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

func buildModelFile(table *Table, existingTags map[string]map[string]string) string {
	structName := structNameForTable(table.Name)
	imports := map[string]struct{}{}
	dtoFields := make([]string, 0, len(table.Columns))
	readFields := make([]string, 0, len(table.Columns))

	for _, col := range table.Columns {
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
		if col.IsGenerated {
			readFields = append(readFields, fmt.Sprintf("\t%s %s %s", fieldName, fieldType, tag))
			continue
		}
		dtoFields = append(dtoFields, fmt.Sprintf("\t%s %s %s", fieldName, fieldType, tag))
	}
	imports["github.com/gookit/validate"] = struct{}{}
	imports["github.com/0xelden/common-libs-go/shared"] = struct{}{}
	createValidationFields := buildCreateValidationFields(table)

	var builder strings.Builder
	builder.WriteString("package model\n\n")
	writeImportBlock(&builder, imports)
	builder.WriteString(fmt.Sprintf("type %sDto struct {\n", structName))
	if len(dtoFields) > 0 {
		builder.WriteString(strings.Join(dtoFields, "\n"))
		builder.WriteString("\n")
	}
	builder.WriteString("}\n")
	builder.WriteString("\n")
	builder.WriteString(fmt.Sprintf("type %s struct {\n", structName))
	if len(readFields) > 0 {
		builder.WriteString("\t// Database-generated columns are exposed on the read model only.\n")
		builder.WriteString(strings.Join(readFields, "\n"))
		builder.WriteString("\n")
	}
	builder.WriteString(fmt.Sprintf("\t%sDto\n", structName))
	builder.WriteString("}\n")
	builder.WriteString("\n")
	builder.WriteString(fmt.Sprintf("func (%s) ConfigValidation(v *validate.Validation) {\n", structName))
	builder.WriteString("\tv.WithScenes(validate.SValues{\n")
	builder.WriteString(fmt.Sprintf("\t\tshared.SceneCreate: []string{%s},\n", createValidationFields))
	builder.WriteString("\t\tshared.SceneEdit:   []string{\"Id\"},\n")
	builder.WriteString("\t})\n")
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
	schemaQualified := table.Name
	if table.Schema != "" {
		schemaQualified = fmt.Sprintf("%s.%s", table.Schema, table.Name)
	}

	template := `package %[1]s

import (
	"github.com/0xelden/common-libs-go/db"
	csvc "github.com/0xelden/common-libs-go/service"
	"github.com/0xelden/common-libs-go/service/repository/general"
	"%[2]s/modules/%[1]s/model"
)

//goland:noinspection GoNameStartsWithPackageName
type %[3]sRepo struct {
	table  string
	driver db.Driver
	gen    csvc.GeneralRepo
	dto    csvc.DTO[model.%[3]sDto]
	repo   csvc.Repository[model.%[3]sDto, model.%[3]s]
}

func New%[3]sRepo(driver db.Driver) (result *%[3]sRepo) {
	table := "%[4]s"
	gen := general.NewGeneralRepo(driver, table)
	result = &%[3]sRepo{
		driver: driver,
		gen:    gen,
		table:  table,
		dto:    general.NewDTO[model.%[3]sDto](table, gen),
	}
	repo := general.
		NewRepositoryBuilder[model.%[3]sDto, model.%[3]s](driver, table).
		WithIndexQuery(Count%[3]sQuery, Index%[3]sQuery).
		WithOverride(result).
		WithDtoInterface(result.dto).
		WithGeneralRepoInterface(gen).
		New()
	result.repo = repo
	return result
}

func (r *%[3]sRepo) Repository() csvc.Repository[model.%[3]sDto, model.%[3]s] {
	return r.repo
}
`

	return fmt.Sprintf(template, moduleName, modulePath, structName, schemaQualified)
}

func buildControllerFile(table *Table, modulePath string) string {
	moduleName := moduleNameForTable(table.Name)
	structName := structNameForTable(table.Name)
	repoField := lowerCamelCase(structName)
	editableFields := buildEditableDTOFields(table)

	template := `package %[1]s

import (
	"context"

	"github.com/0xelden/common-libs-go/api"
	"github.com/0xelden/common-libs-go/db"
	"github.com/0xelden/common-libs-go/helper"
	"%[2]s/modules/%[1]s/model"
	"github.com/0xelden/common-libs-go/serror"
)

//goland:noinspection GoNameStartsWithPackageName
type %[3]sCtrl struct {
	%[4]s *%[3]sRepo
}

func New%[3]sUsecase(driver db.Driver) *%[3]sCtrl {
	return &%[3]sCtrl{
		%[4]s: New%[3]sRepo(driver),
	}
}

func (c *%[3]sCtrl) Index%[3]s(ctx context.Context, param api.IndexParam) (result api.RowIndex[model.%[3]s], serr serror.SError) {
	return c.%[4]s.Repository().List(ctx, param)
}

func (c *%[3]sCtrl) Create%[3]s(ctx context.Context, form model.%[3]sDto) (id string, serr serror.SError) {
	res, serr := c.%[4]s.Repository().Insert(ctx, form)
	if serr != nil {
		return id, serr
	}

	return res.Id, nil
}

func (c *%[3]sCtrl) View%[3]s(ctx context.Context, param api.ViewParam) (result model.%[3]s, serr serror.SError) {
	return c.%[4]s.Repository().Get(ctx, param)
}

func (c *%[3]sCtrl) Edit%[3]s(ctx context.Context, form model.%[3]sDto) (affected int64, serr serror.SError) {
	editable := model.%[3]sDto{
%[5]s
	}
	_, serr = c.%[4]s.Repository().DTO().PatchRow(ctx, editable, nil)
	if serr != nil {
		return affected, serr
	}
	return 1, nil
}

func (c *%[3]sCtrl) Delete%[3]s(ctx context.Context, id string) (affected int64, serr serror.SError) {
	if !helper.IsValidUUID(id) {
		return affected, serror.New("invalid id, expected to be a uuid")
	}
	return c.%[4]s.Repository().Remove(ctx, id)
}
`

	return fmt.Sprintf(template, moduleName, modulePath, structName, repoField, editableFields)
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

func buildCreateValidationFields(table *Table) string {
	fields := make([]string, 0, len(table.Columns))
	for _, col := range table.Columns {
		if col.IsGenerated || isIDColumn(col) || isAuditColumn(col.Name) || strings.EqualFold(col.Name, "status") {
			continue
		}
		fields = append(fields, strconv.Quote(pascalCase(col.Name)))
	}
	return strings.Join(fields, ", ")
}

func buildHTTPFile(table *Table, modulePath string) string {
	moduleName := moduleNameForTable(table.Name)
	structName := structNameForTable(table.Name)

	template := `package %[1]s

import (
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/0xelden/common-libs-go/api"
	"github.com/0xelden/common-libs-go/db"
	"github.com/0xelden/common-libs-go/helper"
	"github.com/0xelden/common-libs-go/shared"
	"%[2]s/modules/%[1]s/model"
	"github.com/0xelden/common-libs-go/serror"
)

func Index%[3]s(ctrl *%[3]sCtrl) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := api.SetContext(c, shared.OmitTotal, c.Query(shared.OmitTotal))
		param, err := api.NewIndexParam(c, binding.Query)
		if err != nil {
			api.Error(c, err)
			return
		}
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
		form := model.%[3]s{}
		if err := api.BindValidate(c, &form, shared.SceneCreate); err != nil {
			api.Error(c, err)
			return
		}
		ctx, trx := api.SetCtxAndTrx(c, txb)
		result, serr := New%[3]sUsecase(trx).Create%[3]s(ctx, form.%[3]sDto)
		if serr != nil {
			api.Error(c, serr)
			return
		}
		api.Success(c, result)
	}
}

func View%[3]s(ctrl *%[3]sCtrl) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if !helper.IsValidUUID(id) {
			api.Error(c, serror.New("invalid id, expected to be a uuid"))
			return
		}
		param, err := api.NewViewParam(c, binding.Query, id)
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
		form := model.%[3]s{}
		if err := api.BindValidate(c, &form, shared.SceneEdit); err != nil {
			api.Error(c, err)
			return
		}
		ctx, trx := api.SetCtxAndTrx(c, txb)
		result, serr := New%[3]sUsecase(trx).Edit%[3]s(ctx, form.%[3]sDto)
		if serr != nil {
			api.Error(c, serr)
			return
		}
		api.Success(c, result)
	}
}

func Delete%[3]s(txb db.TrxBuilder) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if !helper.IsValidUUID(id) {
			api.Error(c, serror.New("invalid id, expected to be a uuid"))
			return
		}
		ctx, trx := api.SetCtxAndTrx(c, txb)
		result, serr := New%[3]sUsecase(trx).Delete%[3]s(ctx, id)
		if serr != nil {
			api.Error(c, serr)
			return
		}
		api.Success(c, result)
	}
}
`

	return fmt.Sprintf(template, moduleName, modulePath, structName)
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
