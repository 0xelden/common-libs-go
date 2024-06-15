package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/go-errors/errors"
)

// loadDTOFieldTags loads struct tags from an existing generated model or legacy dto file.
func loadDTOFieldTags(outDir string, table *Table) (map[string]map[string]string, error) {
	moduleName := moduleNameForTable(table.Name)
	paths := []string{
		filepath.Join(outDir, "modules", moduleName, "model", fmt.Sprintf("%s_model.go", moduleName)),
		filepath.Join(outDir, "modules", moduleName, "dto", fmt.Sprintf("%s_dto.go", moduleName)),
	}
	structName := structNameForTable(table.Name) + "Dto"

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, errors.Errorf("read existing model file: %v", err)
		}

		fset := token.NewFileSet()
		file, err := decorator.ParseFile(fset, path, data, parser.ParseComments)
		if err != nil {
			return nil, errors.Errorf("parse existing model file: %v", err)
		}

		tags := findStructFieldTags(file, structName)
		if len(tags) > 0 {
			return tags, nil
		}
	}

	return nil, nil
}

// findStructFieldTags returns tags keyed by struct field name.
func findStructFieldTags(file *dst.File, structName string) map[string]map[string]string {
	if file == nil {
		return nil
	}
	result := map[string]map[string]string{}
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*dst.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*dst.TypeSpec)
			if !ok || typeSpec.Name == nil || typeSpec.Name.Name != structName {
				continue
			}
			structType, ok := typeSpec.Type.(*dst.StructType)
			if !ok || structType.Fields == nil {
				continue
			}
			for _, field := range structType.Fields.List {
				if len(field.Names) == 0 {
					continue
				}
				tags := parseStructTagLiteral(field.Tag)
				if len(tags) == 0 {
					continue
				}
				for _, name := range field.Names {
					result[name.Name] = cloneTagMap(tags)
				}
			}
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// parseStructTagLiteral parses a dst tag literal into a map.
func parseStructTagLiteral(lit *dst.BasicLit) map[string]string {
	if lit == nil || lit.Value == "" {
		return nil
	}
	unquoted, err := strconv.Unquote(lit.Value)
	if err != nil {
		return nil
	}
	return splitStructTags(unquoted)
}

// splitStructTags parses a struct tag string into key value pairs.
func splitStructTags(raw string) map[string]string {
	result := map[string]string{}
	for {
		raw = strings.TrimLeft(raw, " \t")
		if raw == "" {
			break
		}
		colon := strings.IndexByte(raw, ':')
		if colon <= 0 {
			break
		}
		key := raw[:colon]
		raw = raw[colon+1:]
		value, rest, ok := readQuotedTagValue(raw)
		if !ok {
			break
		}
		result[key] = value
		raw = rest
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// readQuotedTagValue reads a double quoted tag value and the remaining string.
func readQuotedTagValue(input string) (string, string, bool) {
	if input == "" || input[0] != '"' {
		return "", input, false
	}
	escaped := false
	for i := 1; i < len(input); i++ {
		ch := input[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == '"' {
			value, err := strconv.Unquote(input[:i+1])
			if err != nil {
				return "", input, false
			}
			return value, input[i+1:], true
		}
	}
	return "", input, false
}

// mergeDTOStructTags keeps existing tags when present and fills in dto defaults.
func mergeDTOStructTags(column string, existing map[string]string) string {
	tags := cloneTagMap(existing)
	if tags == nil {
		tags = map[string]string{}
	}
	delete(tags, "query")
	for key, value := range map[string]string{
		"form": column,
		"json": column,
		"db":   column,
	} {
		if tags[key] == "" {
			tags[key] = value
		}
	}
	if strings.EqualFold(column, "id") {
		tags["validate"] = "required|uuid"
	}

	parts := make([]string, 0, len(tags))
	seen := map[string]struct{}{}
	for _, key := range []string{"form", "json", "db", "validate"} {
		value, ok := tags[key]
		if !ok {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s:%s", key, strconv.Quote(value)))
		seen[key] = struct{}{}
	}

	keys := make([]string, 0, len(tags))
	for key := range tags {
		if _, ok := seen[key]; ok {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s:%s", key, strconv.Quote(tags[key])))
	}

	return "`" + strings.Join(parts, " ") + "`"
}

func cloneTagMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
