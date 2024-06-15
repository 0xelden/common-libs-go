package main

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"

	"github.com/go-errors/errors"
)

// renderGeneratedFile returns the new content and whether it should be written.
func renderGeneratedFile(cfg config, targetPath string, file GeneratedFile, generated string) (string, bool, error) {
	if cfg.hard && !file.SkipHard {
		if !cfg.force {
			if _, statErr := os.Stat(targetPath); statErr == nil {
				return "", false, errors.Errorf("target file already exists (use --force to overwrite): %s", file.Path)
			}
		}
		return generated, true, nil
	}

	existing, err := os.ReadFile(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return generated, true, nil
		}
		return "", false, errors.Errorf("read %s: %v", targetPath, err)
	}

	existingContent := string(existing)
	if strings.TrimSpace(existingContent) == "" {
		return generated, true, nil
	}

	found, err := declExists(existingContent, file.Targets)
	if err != nil {
		return "", false, errors.Errorf("inspect %s: %v", targetPath, err)
	}
	if found {
		if cfg.force && !file.SkipHard {
			return generated, true, nil
		}
		return "", false, nil
	}

	updated, err := mergeGeneratedContent(existingContent, generated)
	if err != nil {
		return "", false, errors.Errorf("merge into %s: %v", targetPath, err)
	}
	return updated, true, nil
}

// declExists reports whether any target declaration is present.
func declExists(content string, targets []DeclTarget) (bool, error) {
	if len(targets) == 0 {
		return false, nil
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", content, parser.ParseComments)
	if err != nil {
		return false, err
	}

	names := map[token.Token]map[string]struct{}{
		token.TYPE:  {},
		token.CONST: {},
		token.VAR:   {},
		token.FUNC:  {},
	}

	for _, decl := range file.Decls {
		switch typed := decl.(type) {
		case *ast.GenDecl:
			switch typed.Tok {
			case token.TYPE:
				for _, spec := range typed.Specs {
					typeSpec, ok := spec.(*ast.TypeSpec)
					if !ok || typeSpec.Name == nil {
						continue
					}
					names[token.TYPE][typeSpec.Name.Name] = struct{}{}
				}
			case token.CONST:
				for _, spec := range typed.Specs {
					valueSpec, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for _, name := range valueSpec.Names {
						names[token.CONST][name.Name] = struct{}{}
					}
				}
			case token.VAR:
				for _, spec := range typed.Specs {
					valueSpec, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for _, name := range valueSpec.Names {
						names[token.VAR][name.Name] = struct{}{}
					}
				}
			}
		case *ast.FuncDecl:
			if typed.Name != nil {
				names[token.FUNC][typed.Name.Name] = struct{}{}
			}
		}
	}

	for _, target := range targets {
		if _, ok := names[target.Kind][target.Name]; ok {
			return true, nil
		}
	}
	return false, nil
}

func mergeGeneratedContent(existing, generated string) (string, error) {
	fset := token.NewFileSet()
	existingFile, err := parser.ParseFile(fset, "", existing, parser.ParseComments)
	if err != nil {
		return "", err
	}

	generatedSet := token.NewFileSet()
	generatedFile, err := parser.ParseFile(generatedSet, "", generated, parser.ParseComments)
	if err != nil {
		return "", err
	}

	mergeImports(existingFile, generatedFile)
	mergeDecls(existingFile, generatedFile)

	var buf bytes.Buffer
	if err := format.Node(&buf, fset, existingFile); err != nil {
		return "", err
	}
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return buf.String(), nil
	}
	return string(formatted), nil
}

func mergeImports(existingFile, generatedFile *ast.File) {
	if existingFile == nil || generatedFile == nil {
		return
	}

	existingImports := map[string]struct{}{}
	var targetDecl *ast.GenDecl

	for _, decl := range existingFile.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.IMPORT {
			continue
		}
		if targetDecl == nil {
			targetDecl = genDecl
		}
		for _, spec := range genDecl.Specs {
			importSpec, ok := spec.(*ast.ImportSpec)
			if !ok {
				continue
			}
			existingImports[importSpecKey(importSpec)] = struct{}{}
		}
	}

	newSpecs := make([]ast.Spec, 0)
	for _, importSpec := range generatedFile.Imports {
		key := importSpecKey(importSpec)
		if _, ok := existingImports[key]; ok {
			continue
		}
		newSpecs = append(newSpecs, cloneImportSpec(importSpec))
		existingImports[key] = struct{}{}
	}
	if len(newSpecs) == 0 {
		return
	}

	if targetDecl == nil {
		newDecls := make([]ast.Decl, 0, len(newSpecs)+len(existingFile.Decls))
		for _, spec := range newSpecs {
			newDecls = append(newDecls, &ast.GenDecl{
				Tok:   token.IMPORT,
				Specs: []ast.Spec{spec},
			})
		}
		existingFile.Decls = append(newDecls, existingFile.Decls...)
		return
	}

	targetDecl.Specs = append(targetDecl.Specs, newSpecs...)
	if len(targetDecl.Specs) > 1 {
		targetDecl.Lparen = 1
	}
}

func mergeDecls(existingFile, generatedFile *ast.File) {
	if existingFile == nil || generatedFile == nil {
		return
	}
	for _, decl := range generatedFile.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if ok && genDecl.Tok == token.IMPORT {
			continue
		}
		existingFile.Decls = append(existingFile.Decls, decl)
	}
}

func importSpecKey(spec *ast.ImportSpec) string {
	if spec == nil || spec.Path == nil {
		return ""
	}
	name := ""
	if spec.Name != nil {
		name = spec.Name.Name
	}
	return name + "|" + spec.Path.Value
}

func cloneImportSpec(spec *ast.ImportSpec) *ast.ImportSpec {
	if spec == nil || spec.Path == nil {
		return nil
	}

	clone := &ast.ImportSpec{
		Path: &ast.BasicLit{
			Kind:  token.STRING,
			Value: spec.Path.Value,
		},
	}
	if spec.Name != nil {
		clone.Name = ast.NewIdent(spec.Name.Name)
	}
	if spec.Comment != nil {
		clone.Comment = &ast.CommentGroup{List: append([]*ast.Comment(nil), spec.Comment.List...)}
	}
	if spec.Doc != nil {
		clone.Doc = &ast.CommentGroup{List: append([]*ast.Comment(nil), spec.Doc.List...)}
	}
	if unquoted, err := strconv.Unquote(spec.Path.Value); err == nil {
		clone.EndPos = token.Pos(len(unquoted))
	}
	return clone
}
