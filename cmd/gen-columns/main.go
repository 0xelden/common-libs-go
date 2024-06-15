package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"unicode"

	"github.com/go-errors/errors"
)

type structInfo struct {
	Name   string
	Struct *ast.StructType
	File   string
}

type fieldInfo struct {
	Name   string
	Column string
}

type generatedStruct struct {
	Name   string
	Fields []fieldInfo
}

type config struct {
	Root       string
	StructPath string
	OutputPath string
	Full       bool
	Generator  string
}

// main loads flags and runs the command.
func main() {
	os.Exit(runCommand(os.Args[1:], os.Stdout, os.Stderr))
}

func runCommand(args []string, stdout, stderr io.Writer) int {
	cfg, err := loadConfigWithOutput(args, stdout)
	if err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintf(stderr, "gen-columns: %v\n", err)
		return 1
	}

	if err := run(cfg); err != nil {
		fmt.Fprintf(stderr, "gen-columns: %v\n", err)
		return 1
	}

	return 0
}

func loadConfig(args []string) (cfg config, err error) {
	return loadConfigWithOutput(args, io.Discard)
}

func loadConfigWithOutput(args []string, output io.Writer) (cfg config, err error) {
	fs := flag.NewFlagSet("gen-columns", flag.ContinueOnError)
	fs.SetOutput(output)
	fs.BoolVar(&cfg.Full, "full", false, "write per-struct column files alongside col.go")
	fs.StringVar(&cfg.StructPath, "struct-path", "", "root path to recursively search for Go structs")
	fs.StringVar(&cfg.OutputPath, "output-path", "", "root path for generated output containing col and dao directories")
	cfg.Generator = buildGeneratorCommand(fs.Name(), args)
	if err = fs.Parse(args); err != nil {
		return cfg, err
	}

	cfg.Root, err = os.Getwd()
	if err != nil {
		return cfg, err
	}

	if cfg.OutputPath == "" {
		cfg.OutputPath = filepath.Join(cfg.Root, "pkg", "model")
	} else {
		cfg.OutputPath, err = filepath.Abs(cfg.OutputPath)
		if err != nil {
			return cfg, err
		}
	}

	if cfg.StructPath == "" {
		cfg.StructPath = cfg.Root
	} else {
		cfg.StructPath, err = filepath.Abs(cfg.StructPath)
		if err != nil {
			return cfg, err
		}
	}

	return cfg, nil
}

// run generates flat column constants and optionally per-struct column files.
func run(cfg config) error {
	outputPath := cfg.OutputPath
	if outputPath == "" {
		outputPath = filepath.Join(cfg.Root, "pkg", "model")
	}

	structPath := cfg.StructPath
	if structPath == "" {
		structPath = filepath.Join(outputPath, "dao")
	}

	colDir := filepath.Join(outputPath, "col")
	daoDir := filepath.Join(outputPath, "dao")

	if err := os.MkdirAll(colDir, 0o755); err != nil {
		return errors.Errorf("create col dir: %v", err)
	}

	if err := cleanColDir(colDir); err != nil {
		return err
	}

	if cfg.Full {
		if err := os.MkdirAll(daoDir, 0o755); err != nil {
			return errors.Errorf("create dao dir: %v", err)
		}
		if err := cleanColDir(daoDir); err != nil {
			return err
		}
	}

	structs, fileStructs, err := loadStructs(structPath)
	if err != nil {
		return err
	}

	allColumns := make(map[string]struct{})

	for file, names := range fileStructs {
		base, _ := baseName(file)
		generatedStructs := collectGeneratedStructs(structs, names)
		if len(generatedStructs) == 0 {
			continue
		}

		for _, generated := range generatedStructs {
			for _, field := range generated.Fields {
				allColumns[field.Column] = struct{}{}
			}
		}

		if cfg.Full {
			if err := writeColumnFile(daoDir, "dao", base, cfg.Generator, generatedStructs); err != nil {
				return err
			}
		}
	}

	if err := writeAllColumns(colDir, cfg.Generator, allColumns); err != nil {
		return err
	}

	return nil
}

func loadStructs(sourceDir string) (map[string]structInfo, map[string][]string, error) {
	fset := token.NewFileSet()
	structs := make(map[string]structInfo)
	fileStructs := make(map[string][]string)

	err := filepath.WalkDir(sourceDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == "vendor" || strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), "_test.go") || !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}

		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return errors.Errorf("parse file %s: %v", path, err)
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return errors.Errorf("relative path for %s: %v", path, err)
		}

		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				name := ts.Name.Name
				structs[name] = structInfo{
					Name:   name,
					Struct: st,
					File:   relPath,
				}
				fileStructs[relPath] = append(fileStructs[relPath], name)
			}
		}

		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	for _, names := range fileStructs {
		sort.Strings(names)
	}

	return structs, fileStructs, nil
}

func collectGeneratedStructs(structs map[string]structInfo, names []string) []generatedStruct {
	var generated []generatedStruct
	for _, name := range names {
		fields := collectFields(structs, name, map[string]bool{})
		if len(fields) == 0 {
			continue
		}
		generated = append(generated, generatedStruct{
			Name:   name,
			Fields: fields,
		})
	}

	return generated
}

func collectFields(structs map[string]structInfo, name string, seen map[string]bool) []fieldInfo {
	info, ok := structs[name]
	if !ok || info.Struct == nil {
		return nil
	}
	if seen[name] {
		return nil
	}
	seen[name] = true

	var fields []fieldInfo
	for _, field := range info.Struct.Fields.List {
		if len(field.Names) == 0 {
			embedded := embeddedName(field.Type)
			if embedded == "" {
				continue
			}
			fields = append(fields, collectFields(structs, embedded, seen)...)
			continue
		}

		dbTag := dbTagValue(field.Tag)
		if dbTag == "" {
			continue
		}

		for _, nameIdent := range field.Names {
			fields = append(fields, fieldInfo{
				Name:   nameIdent.Name,
				Column: dbTag,
			})
		}
	}

	seen[name] = false
	return fields
}

func hasDBTags(structs map[string]structInfo, name string) bool {
	return len(collectFields(structs, name, map[string]bool{})) > 0
}

func embeddedName(expr ast.Expr) string {
	switch v := expr.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.StarExpr:
		return embeddedName(v.X)
	case *ast.SelectorExpr:
		return v.Sel.Name
	default:
		return ""
	}
}

func dbTagValue(tag *ast.BasicLit) string {
	if tag == nil || tag.Kind != token.STRING {
		return ""
	}
	raw := strings.Trim(tag.Value, "`")
	return reflect.StructTag(raw).Get("db")
}

func generatedHeader(command string) string {
	if strings.TrimSpace(command) == "" {
		command = "gen-columns"
	}

	var builder strings.Builder
	builder.WriteString("// Code generated by gen-columns DO NOT EDIT.\n")
	builder.WriteString("//\n")
	builder.WriteString("// WARNING: Changes to this file may cause incorrect behavior\n")
	builder.WriteString("// and will be lost if the code is regenerated\n")
	builder.WriteString("//\n")
	builder.WriteString("// Generator:\n")
	builder.WriteString(fmt.Sprintf("// %s\n\n", command))

	return builder.String()
}

func buildGeneratorCommand(name string, args []string) string {
	parts := []string{name}
	parts = append(parts, args...)
	return strings.Join(parts, " ")
}

func writeColumnFile(targetDir, packageName, base, generator string, structs []generatedStruct) error {
	filename := filepath.Join(targetDir, fmt.Sprintf("%s.col.go", base))

	var builder strings.Builder
	builder.WriteString(generatedHeader(generator))
	builder.WriteString(fmt.Sprintf("package %s\n\n", packageName))

	for i, generated := range structs {
		if i > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(fmt.Sprintf("var %s = struct {\n", generated.Name))
		for _, field := range generated.Fields {
			builder.WriteString(fmt.Sprintf("\t%s string\n", field.Name))
		}
		builder.WriteString("}{\n")
		for _, field := range generated.Fields {
			builder.WriteString(fmt.Sprintf("\t%s: \"%s\",\n", field.Name, field.Column))
		}
		builder.WriteString("}\n")
	}

	src, err := format.Source([]byte(builder.String()))
	if err != nil {
		return errors.Errorf("format generated code for %s: %v", filename, err)
	}

	if err := os.WriteFile(filename, src, 0o644); err != nil {
		return errors.Errorf("write file %s: %v", filename, err)
	}

	return nil
}

func pascalCase(name string) string {
	parts := strings.FieldsFunc(name, func(r rune) bool {
		return r == '_' || r == '-' || r == ' ' || r == '.'
	})
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
	}
	return strings.Join(parts, "")
}

func writeAllColumns(colDir, generator string, columns map[string]struct{}) error {
	if len(columns) == 0 {
		return nil
	}

	filename := filepath.Join(colDir, "col.go")
	keys := make([]string, 0, len(columns))
	for col := range columns {
		keys = append(keys, col)
	}
	sort.Strings(keys)

	warnIdentifierCollisions(keys)

	var builder strings.Builder
	builder.WriteString(generatedHeader(generator))
	builder.WriteString("package col\n\n")
	builder.WriteString("// All column names compiled from `db` tags.\n")
	builder.WriteString("const (\n")
	builder.WriteString("\tAll = \"*\"\n")
	for _, col := range keys {
		name := pascalCase(col)
		if !isExportedIdent(name) {
			continue
		}
		builder.WriteString(fmt.Sprintf("\t%s = \"%s\"\n", name, col))
	}
	builder.WriteString(")\n")

	src, err := format.Source([]byte(builder.String()))
	if err != nil {
		return errors.Errorf("format generated col constants: %v", err)
	}

	if err := os.WriteFile(filename, src, 0o644); err != nil {
		return errors.Errorf("write file %s: %v", filename, err)
	}

	return nil
}

func isExportedIdent(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		if i == 0 {
			if !unicode.IsLetter(r) || !unicode.IsUpper(r) {
				return false
			}
			continue
		}
		if !(unicode.IsLetter(r) || unicode.IsDigit(r)) {
			return false
		}
	}
	return true
}

func warnIdentifierCollisions(columns []string) {
	identifierColumns := make(map[string][]string)
	for _, col := range columns {
		name := pascalCase(col)
		if !isExportedIdent(name) {
			continue
		}
		identifierColumns[name] = append(identifierColumns[name], col)
	}

	var duplicates []string
	for name, cols := range identifierColumns {
		if len(cols) > 1 {
			duplicates = append(duplicates, name)
		}
	}
	sort.Strings(duplicates)

	for _, name := range duplicates {
		cols := identifierColumns[name]
		sort.Strings(cols)
		_, err := fmt.Fprintf(os.Stderr, "warning: pkg/model/col/col.go constant %s has multiple column values: %s\n", name, strings.Join(cols, ", "))
		if err != nil {
			panic(err)
		}
	}
}

func cleanColDir(colDir string) error {
	entries, err := os.ReadDir(colDir)
	if err != nil {
		if os.IsNotExist(err) {
			return os.MkdirAll(colDir, 0o755)
		}
		return errors.Errorf("read col dir: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".col.go") {
			if err := os.Remove(filepath.Join(colDir, entry.Name())); err != nil {
				return errors.Errorf("remove %s: %v", entry.Name(), err)
			}
		}
	}
	return nil
}

func baseName(file string) (string, bool) {
	file = filepath.Base(file)
	base := strings.TrimSuffix(file, ".go")
	isDAO := strings.HasSuffix(base, ".dao")
	base = strings.TrimSuffix(base, ".dao")
	base = strings.TrimSuffix(base, ".model")
	base = strings.TrimSuffix(base, ".dao")
	return base, isDAO
}
