package main

import (
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-errors/errors"
)

func updateRegistryFiles(outDir, modulePath string, table *Table, withCtl bool) ([]GeneratedFile, error) {
	files := make([]GeneratedFile, 0, 2)

	repoFile, err := updateServiceRepositoryRegistry(outDir, modulePath, table)
	if err != nil {
		return nil, err
	}
	if repoFile != nil {
		files = append(files, *repoFile)
	}

	if !withCtl {
		return files, nil
	}

	usecaseFile, err := updateServiceUsecaseRegistry(outDir, modulePath, table)
	if err != nil {
		return nil, err
	}
	if usecaseFile != nil {
		files = append(files, *usecaseFile)
	}

	return files, nil
}

func updateServiceRepositoryRegistry(outDir, modulePath string, table *Table) (*GeneratedFile, error) {
	targetPath := filepath.Join(outDir, "service", "repository.go")
	content, err := os.ReadFile(targetPath)
	if err != nil {
		return nil, errors.Errorf("read service/repository.go: %v", err)
	}

	interfaceName, definition := repositoryInterfaceDefinition(modulePath, table)
	exists, err := declExists(string(content), []DeclTarget{{Kind: token.TYPE, Name: interfaceName}})
	if err != nil {
		return nil, errors.Errorf("inspect service/repository.go: %v", err)
	}
	if exists {
		return nil, nil
	}

	return &GeneratedFile{
		Path:     filepath.Join("service", "repository.go"),
		Content:  definition,
		Targets:  []DeclTarget{{Kind: token.TYPE, Name: interfaceName}},
		SkipHard: true,
	}, nil
}

func updateServiceUsecaseRegistry(outDir, modulePath string, table *Table) (*GeneratedFile, error) {
	targetPath := filepath.Join(outDir, "service", "usecase.go")
	content, err := os.ReadFile(targetPath)
	if err != nil {
		return nil, errors.Errorf("read service/usecase.go: %v", err)
	}

	interfaceName, definition := usecaseInterfaceDefinition(modulePath, table)
	exists, err := declExists(string(content), []DeclTarget{{Kind: token.TYPE, Name: interfaceName}})
	if err != nil {
		return nil, errors.Errorf("inspect service/usecase.go: %v", err)
	}
	if exists {
		return nil, nil
	}

	return &GeneratedFile{
		Path:     filepath.Join("service", "usecase.go"),
		Content:  definition,
		Targets:  []DeclTarget{{Kind: token.TYPE, Name: interfaceName}},
		SkipHard: true,
	}, nil
}

func repositoryInterfaceDefinition(modulePath string, table *Table) (string, string) {
	structName := structNameForTable(table.Name)
	interfaceName := repoInterfaceName(table.Name)
	dtoAlias := dtoImportAlias(table.Name)
	modelAlias := modelImportAlias(table.Name)
	moduleName := moduleNameForTable(table.Name)

	definition := fmt.Sprintf(`package service

import (
	"github.com/0xelden/common-libs-go/service"
	%s "%s/modules/%s/dto"
	%s "%s/modules/%s/model"
)

type %s interface {
	Repository() service.Repository[%s.%sDto, %s.%s]
}
`, dtoAlias, modulePath, moduleName, modelAlias, modulePath, moduleName, interfaceName, dtoAlias, structName, modelAlias, structName)

	return interfaceName, definition
}

func usecaseInterfaceDefinition(modulePath string, table *Table) (string, string) {
	structName := structNameForTable(table.Name)
	interfaceName := usecaseInterfaceName(table.Name)
	dtoAlias := dtoImportAlias(table.Name)
	modelAlias := modelImportAlias(table.Name)
	moduleName := moduleNameForTable(table.Name)

	definition := fmt.Sprintf(`package service

import (
	"context"

	"github.com/0xelden/common-libs-go/api"
	%s "%s/modules/%s/dto"
	%s "%s/modules/%s/model"
	"github.com/0xelden/common-libs-go/serror"
)

type %s interface {
	Index%s(ctx context.Context, param api.IndexParam) (result api.RowIndex[%s.%s], serr serror.SError)
	Create%s(ctx context.Context, form %s.%sDto) (id string, serr serror.SError)
	View%s(ctx context.Context, param api.ViewParam) (result %s.%s, serr serror.SError)
	Edit%s(ctx context.Context, form %s.%sDto) (affected int64, serr serror.SError)
	Delete%s(ctx context.Context, id string) (affected int64, serr serror.SError)
}
`, dtoAlias, modulePath, moduleName, modelAlias, modulePath, moduleName, interfaceName, structName, modelAlias, structName, structName, dtoAlias, structName, structName, modelAlias, structName, structName, dtoAlias, structName, structName)

	return interfaceName, definition
}

func repoInterfaceName(tableName string) string {
	base := structNameForTable(tableName)
	if base == "" {
		base = pascalCase(tableName)
	}
	return base + "Repo"
}

func usecaseInterfaceName(tableName string) string {
	base := structNameForTable(tableName)
	if base == "" {
		base = pascalCase(tableName)
	}
	return base + "Usecase"
}

func dtoImportAlias(tableName string) string {
	return strings.ReplaceAll(moduleNameForTable(tableName), "_", "") + "dto"
}

func modelImportAlias(tableName string) string {
	return strings.ReplaceAll(moduleNameForTable(tableName), "_", "") + "model"
}
