package main

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/go-errors/errors"
)

type routeArgKind int

const (
	routeArgUnknown routeArgKind = iota
	routeArgTrxBuilder
	routeArgUsecase
)

type httpRouteSpec struct {
	ModuleName string
	ImportPath string
	Usecase    httpUsecaseSpec
	Routes     []httpRoute
}

type httpUsecaseSpec struct {
	TypeName  string
	FieldName string
}

type httpRoute struct {
	Method       string
	Path         string
	FuncName     string
	ArgKind      routeArgKind
	UsecaseField string
}

func patchServiceHTTPRoutes(targetPath, modulePath, generatedHTTP string) (string, bool, error) {
	spec, err := inferHTTPRouteSpec(generatedHTTP, modulePath)
	if err != nil {
		return "", false, err
	}
	if len(spec.Routes) == 0 {
		return "", false, nil
	}

	content, err := os.ReadFile(targetPath)
	if err != nil {
		return "", false, errors.Errorf("read %s: %v", targetPath, err)
	}

	updated, changed, err := mergeHTTPRouteFile(string(content), spec)
	if err != nil {
		return "", false, errors.Errorf("patch %s: %v", targetPath, err)
	}
	return updated, changed, nil
}

func patchServiceHTTPHandler(outDir, generatedHTTP string) (string, bool, error) {
	spec, err := inferHTTPRouteSpec(generatedHTTP, "")
	if err != nil {
		return "", false, err
	}
	if spec.Usecase.TypeName == "" || spec.Usecase.FieldName == "" {
		return "", false, nil
	}

	targetPath := filepath.Join(outDir, "service", "http", "handler.go")
	content, err := os.ReadFile(targetPath)
	if err != nil {
		return "", false, errors.Errorf("read service/http/handler.go: %v", err)
	}

	updated, changed, err := mergeHTTPHandlerFile(string(content), spec)
	if err != nil {
		return "", false, errors.Errorf("patch service/http/handler.go: %v", err)
	}
	return updated, changed, nil
}

func patchHTTPServicesConfig(outDir, modulePath, generatedHTTP string) (string, bool, error) {
	spec, err := inferHTTPRouteSpec(generatedHTTP, modulePath)
	if err != nil {
		return "", false, err
	}
	if spec.ImportPath == "" || spec.Usecase.TypeName == "" || spec.Usecase.FieldName == "" {
		return "", false, nil
	}

	targetPath := filepath.Join(outDir, "config", "http", "services.go")
	content, err := os.ReadFile(targetPath)
	if err != nil {
		return "", false, errors.Errorf("read config/http/services.go: %v", err)
	}

	updated, changed, err := mergeHTTPServicesConfigFile(string(content), spec)
	if err != nil {
		return "", false, errors.Errorf("patch config/http/services.go: %v", err)
	}
	return updated, changed, nil
}

func inferHTTPRouteSpec(generatedHTTP, modulePath string) (httpRouteSpec, error) {
	file, err := parser.ParseFile(token.NewFileSet(), "", generatedHTTP, parser.ParseComments)
	if err != nil {
		return httpRouteSpec{}, errors.Errorf("parse generated http handler: %v", err)
	}

	spec := httpRouteSpec{
		ModuleName: file.Name.Name,
	}
	if modulePath != "" {
		spec.ImportPath = path.Join(modulePath, "modules", file.Name.Name)
	}

	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Recv != nil || funcDecl.Name == nil {
			continue
		}

		method, routePath, ok := inferHandlerRouteMeta(funcDecl.Name.Name)
		if !ok {
			continue
		}

		argKind, usecaseField, ok := inferHandlerRouteArg(funcDecl.Type)
		if !ok {
			continue
		}
		if argKind == routeArgUsecase && spec.Usecase.TypeName == "" {
			if typeName := inferUsecaseTypeName(funcDecl.Type); typeName != "" {
				spec.Usecase = httpUsecaseSpec{
					TypeName:  typeName,
					FieldName: usecaseField,
				}
			}
		}

		spec.Routes = append(spec.Routes, httpRoute{
			Method:       method,
			Path:         strings.ReplaceAll(routePath, "_", "-"),
			FuncName:     funcDecl.Name.Name,
			ArgKind:      argKind,
			UsecaseField: usecaseField,
		})
	}

	return spec, nil
}

func inferHandlerRouteMeta(funcName string) (method string, routePath string, ok bool) {
	switch {
	case strings.HasPrefix(funcName, "Index"):
		return "GET", "/index", true
	case strings.HasPrefix(funcName, "Save"):
		return "POST", "/add", true
	case strings.HasPrefix(funcName, "View"):
		return "GET", "/view/:id", true
	case strings.HasPrefix(funcName, "Edit"):
		return "PUT", "/edit", true
	case strings.HasPrefix(funcName, "UpdateStatus"):
		return "PUT", "/update-status", true
	case strings.HasPrefix(funcName, "Delete"):
		return "DELETE", "/delete/:id", true
	default:
		return "", "", false
	}
}

func inferHandlerRouteArg(funcType *ast.FuncType) (routeArgKind, string, bool) {
	if funcType == nil || funcType.Params == nil || len(funcType.Params.List) != 1 {
		return routeArgUnknown, "", false
	}

	selector, ok := funcType.Params.List[0].Type.(*ast.SelectorExpr)
	if !ok {
		return routeArgUnknown, "", false
	}

	pkgIdent, ok := selector.X.(*ast.Ident)
	if !ok {
		return routeArgUnknown, "", false
	}

	switch {
	case pkgIdent.Name == "db" && selector.Sel.Name == "TrxBuilder":
		return routeArgTrxBuilder, "", true
	case pkgIdent.Name == "service" && strings.HasSuffix(selector.Sel.Name, "Usecase"):
		usecaseName := strings.TrimSuffix(selector.Sel.Name, "Usecase")
		return routeArgUsecase, lowerCamelCase(usecaseName) + "Usecase", true
	default:
		return routeArgUnknown, "", false
	}
}

func inferUsecaseTypeName(funcType *ast.FuncType) string {
	if funcType == nil || funcType.Params == nil || len(funcType.Params.List) != 1 {
		return ""
	}

	selector, ok := funcType.Params.List[0].Type.(*ast.SelectorExpr)
	if !ok || selector.Sel == nil {
		return ""
	}
	pkgIdent, ok := selector.X.(*ast.Ident)
	if !ok || pkgIdent.Name != "service" {
		return ""
	}
	if !strings.HasSuffix(selector.Sel.Name, "Usecase") {
		return ""
	}
	return selector.Sel.Name
}

func mergeHTTPRouteFile(existing string, spec httpRouteSpec) (string, bool, error) {
	fset := token.NewFileSet()
	file, err := decorator.ParseFile(fset, "", existing, parser.ParseComments)
	if err != nil {
		return "", false, err
	}

	changed := ensureRouteImport(file, spec.ImportPath)
	routeChanged, err := ensureRouteRegistrations(file, spec)
	if err != nil {
		return "", false, err
	}
	changed = changed || routeChanged
	if !changed {
		return "", false, nil
	}

	return restorePatchedFile(file)
}

func mergeHTTPHandlerFile(existing string, spec httpRouteSpec) (string, bool, error) {
	fset := token.NewFileSet()
	file, err := decorator.ParseFile(fset, "", existing, parser.ParseComments)
	if err != nil {
		return "", false, err
	}

	changed := false

	structChanged, err := ensureGatewayHandlerField(file, spec.Usecase)
	if err != nil {
		return "", false, err
	}
	changed = changed || structChanged

	paramChanged, err := ensureNewGatewayHandlerParam(file, spec.Usecase)
	if err != nil {
		return "", false, err
	}
	changed = changed || paramChanged

	literalChanged, err := ensureGatewayHandlerLiteralField(file, spec.Usecase)
	if err != nil {
		return "", false, err
	}
	changed = changed || literalChanged

	if !changed {
		return "", false, nil
	}

	return restorePatchedFile(file)
}

func mergeHTTPServicesConfigFile(existing string, spec httpRouteSpec) (string, bool, error) {
	fset := token.NewFileSet()
	file, err := decorator.ParseFile(fset, "", existing, parser.ParseComments)
	if err != nil {
		return "", false, err
	}

	changed := ensureRouteImport(file, spec.ImportPath)

	varChanged, err := ensureRegisterServiceVar(file, spec)
	if err != nil {
		return "", false, err
	}
	changed = changed || varChanged

	argChanged, err := ensureRegisterServiceGatewayArg(file, spec)
	if err != nil {
		return "", false, err
	}
	changed = changed || argChanged

	if !changed {
		return "", false, nil
	}

	return restorePatchedFile(file)
}

func restorePatchedFile(file *dst.File) (string, bool, error) {
	var buf bytes.Buffer
	if err := decorator.Fprint(&buf, file); err != nil {
		return "", false, err
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return buf.String(), true, nil
	}
	return string(formatted), true, nil
}

func ensureRouteImport(file *dst.File, importPath string) bool {
	if file == nil || importPath == "" {
		return false
	}

	quoted := strconv.Quote(importPath)
	var importDecl *dst.GenDecl
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*dst.GenDecl)
		if !ok || genDecl.Tok != token.IMPORT {
			continue
		}
		if importDecl == nil {
			importDecl = genDecl
		}
		for _, spec := range genDecl.Specs {
			importSpec, ok := spec.(*dst.ImportSpec)
			if !ok || importSpec.Path == nil {
				continue
			}
			if importSpec.Path.Value == quoted {
				return false
			}
		}
	}

	newSpec := &dst.ImportSpec{
		Path: &dst.BasicLit{Kind: token.STRING, Value: quoted},
	}
	if importDecl == nil {
		file.Decls = append([]dst.Decl{
			&dst.GenDecl{
				Tok:   token.IMPORT,
				Specs: []dst.Spec{newSpec},
			},
		}, file.Decls...)
		return true
	}

	importDecl.Specs = append(importDecl.Specs, newSpec)
	if len(importDecl.Specs) > 1 {
		importDecl.Lparen = true
		importDecl.Rparen = true
	}
	return true
}

func ensureRouteRegistrations(file *dst.File, spec httpRouteSpec) (bool, error) {
	initRoute := findInitRouteFunc(file)
	if initRoute == nil || initRoute.Body == nil {
		return false, errors.New("initRoute function not found in service/http/route.go")
	}

	block := findModuleRouteBlock(initRoute.Body, "/"+spec.ModuleName)
	if block == nil {
		initRoute.Body.List = append(initRoute.Body.List, newModuleRouteBlock(spec))
		return true, nil
	}

	existingRoutes := existingRouteStatements(block)
	changed := false
	for _, route := range spec.Routes {
		key := routeStatementKey(route.Method, route.Path, route.FuncName)
		if _, ok := existingRoutes[key]; ok {
			continue
		}
		block.List = append(block.List, newRouteStatement(spec.ModuleName, route))
		changed = true
	}
	return changed, nil
}

func ensureGatewayHandlerField(file *dst.File, usecase httpUsecaseSpec) (bool, error) {
	structType, err := findGatewayHandlerStruct(file)
	if err != nil {
		return false, err
	}
	if structType.Fields == nil {
		structType.Fields = &dst.FieldList{}
	}

	for _, field := range structType.Fields.List {
		for _, name := range field.Names {
			if name != nil && name.Name == usecase.FieldName {
				return false, nil
			}
		}
	}

	structType.Fields.List = append(structType.Fields.List, &dst.Field{
		Names: []*dst.Ident{dst.NewIdent(usecase.FieldName)},
		Type: &dst.SelectorExpr{
			X:   dst.NewIdent("service"),
			Sel: dst.NewIdent(usecase.TypeName),
		},
		Decs: dst.FieldDecorations{
			NodeDecs: dst.NodeDecs{
				Before: dst.NewLine,
				After:  dst.NewLine,
			},
		},
	})
	return true, nil
}

func ensureNewGatewayHandlerParam(file *dst.File, usecase httpUsecaseSpec) (bool, error) {
	constructor, err := findNewGatewayHandlerFunc(file)
	if err != nil {
		return false, err
	}
	if constructor.Type.Params == nil {
		constructor.Type.Params = &dst.FieldList{}
	}

	for _, field := range constructor.Type.Params.List {
		for _, name := range field.Names {
			if name != nil && name.Name == usecase.FieldName {
				return false, nil
			}
		}
	}

	constructor.Type.Params.List = append(constructor.Type.Params.List, &dst.Field{
		Names: []*dst.Ident{dst.NewIdent(usecase.FieldName)},
		Type: &dst.SelectorExpr{
			X:   dst.NewIdent("service"),
			Sel: dst.NewIdent(usecase.TypeName),
		},
		Decs: dst.FieldDecorations{
			NodeDecs: dst.NodeDecs{
				Before: dst.NewLine,
				After:  dst.NewLine,
			},
		},
	})
	return true, nil
}

func ensureGatewayHandlerLiteralField(file *dst.File, usecase httpUsecaseSpec) (bool, error) {
	constructor, err := findNewGatewayHandlerFunc(file)
	if err != nil {
		return false, err
	}
	if constructor.Body == nil {
		return false, errors.New("NewGatewayHandler body not found in service/http/handler.go")
	}

	for _, stmt := range constructor.Body.List {
		assignStmt, ok := stmt.(*dst.AssignStmt)
		if !ok || len(assignStmt.Rhs) != 1 {
			continue
		}

		composite, ok := assignStmt.Rhs[0].(*dst.CompositeLit)
		if !ok {
			continue
		}
		typeIdent, ok := composite.Type.(*dst.Ident)
		if !ok || typeIdent.Name != "gatewayHandler" {
			continue
		}

		for _, elt := range composite.Elts {
			keyValue, ok := elt.(*dst.KeyValueExpr)
			if !ok {
				continue
			}
			key, ok := keyValue.Key.(*dst.Ident)
			if !ok {
				continue
			}
			if key.Name == usecase.FieldName {
				return false, nil
			}
		}

		composite.Elts = append(composite.Elts, &dst.KeyValueExpr{
			Key:   dst.NewIdent(usecase.FieldName),
			Value: dst.NewIdent(usecase.FieldName),
			Decs: dst.KeyValueExprDecorations{
				NodeDecs: dst.NodeDecs{
					Before: dst.NewLine,
					After:  dst.NewLine,
				},
			},
		})
		return true, nil
	}

	return false, errors.New("gatewayHandler composite literal not found in service/http/handler.go")
}

func findInitRouteFunc(file *dst.File) *dst.FuncDecl {
	if file == nil {
		return nil
	}
	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*dst.FuncDecl)
		if !ok || funcDecl.Name == nil {
			continue
		}
		if funcDecl.Name.Name == "initRoute" || funcDecl.Name.Name == "initHttpRoute" {
			return funcDecl
		}
	}
	return nil
}

func findGatewayHandlerStruct(file *dst.File) (*dst.StructType, error) {
	if file == nil {
		return nil, errors.New("service/http/handler.go is empty")
	}
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*dst.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*dst.TypeSpec)
			if !ok || typeSpec.Name == nil || typeSpec.Name.Name != "gatewayHandler" {
				continue
			}
			structType, ok := typeSpec.Type.(*dst.StructType)
			if !ok {
				return nil, errors.New("gatewayHandler is not a struct")
			}
			return structType, nil
		}
	}
	return nil, errors.New("gatewayHandler struct not found in service/http/handler.go")
}

func findNewGatewayHandlerFunc(file *dst.File) (*dst.FuncDecl, error) {
	if file == nil {
		return nil, errors.New("service/http/handler.go is empty")
	}
	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*dst.FuncDecl)
		if !ok || funcDecl.Name == nil {
			continue
		}
		if funcDecl.Name.Name == "NewGatewayHandler" {
			return funcDecl, nil
		}
	}
	return nil, errors.New("NewGatewayHandler not found in service/http/handler.go")
}

func findRegisterServiceFunc(file *dst.File) (*dst.FuncDecl, error) {
	if file == nil {
		return nil, errors.New("config/http/services.go is empty")
	}
	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*dst.FuncDecl)
		if !ok || funcDecl.Name == nil {
			continue
		}
		if funcDecl.Name.Name == "RegisterService" {
			return funcDecl, nil
		}
	}
	return nil, errors.New("RegisterService not found in config/http/services.go")
}

func ensureRegisterServiceVar(file *dst.File, spec httpRouteSpec) (bool, error) {
	registerFunc, err := findRegisterServiceFunc(file)
	if err != nil {
		return false, err
	}
	if registerFunc.Body == nil {
		return false, errors.New("RegisterService body not found in config/http/services.go")
	}

	varName := usecaseCtrlName(spec.Usecase)
	for _, stmt := range registerFunc.Body.List {
		declStmt, ok := stmt.(*dst.DeclStmt)
		if !ok {
			continue
		}
		genDecl, ok := declStmt.Decl.(*dst.GenDecl)
		if !ok || genDecl.Tok != token.VAR {
			continue
		}
		for _, valueSpec := range valueSpecs(genDecl) {
			for _, name := range valueSpec.Names {
				if name != nil && name.Name == varName {
					return false, nil
				}
			}
		}
		genDecl.Specs = append(genDecl.Specs, &dst.ValueSpec{
			Names: []*dst.Ident{dst.NewIdent(varName)},
			Values: []dst.Expr{
				&dst.CallExpr{
					Fun: &dst.SelectorExpr{
						X:   dst.NewIdent(spec.ModuleName),
						Sel: dst.NewIdent(usecaseConstructorName(spec.Usecase)),
					},
					Args: []dst.Expr{dst.NewIdent("pgDriver")},
				},
			},
			Decs: dst.ValueSpecDecorations{
				NodeDecs: dst.NodeDecs{
					Before: dst.NewLine,
					After:  dst.NewLine,
				},
			},
		})
		return true, nil
	}

	return false, errors.New("var block not found in config/http/services.go")
}

func ensureRegisterServiceGatewayArg(file *dst.File, spec httpRouteSpec) (bool, error) {
	registerFunc, err := findRegisterServiceFunc(file)
	if err != nil {
		return false, err
	}
	if registerFunc.Body == nil {
		return false, errors.New("RegisterService body not found in config/http/services.go")
	}

	argName := usecaseCtrlName(spec.Usecase)
	for _, stmt := range registerFunc.Body.List {
		assignStmt, ok := stmt.(*dst.AssignStmt)
		if !ok || len(assignStmt.Rhs) != 1 {
			continue
		}
		callExpr, ok := assignStmt.Rhs[0].(*dst.CallExpr)
		if !ok {
			continue
		}
		selector, ok := callExpr.Fun.(*dst.SelectorExpr)
		if !ok || selector.Sel == nil || selector.Sel.Name != "NewGatewayHandler" {
			continue
		}
		for _, arg := range callExpr.Args {
			ident, ok := arg.(*dst.Ident)
			if ok && ident.Name == argName {
				return false, nil
			}
		}
		arg := dst.NewIdent(argName)
		arg.Decorations().Before = dst.NewLine
		arg.Decorations().After = dst.NewLine
		callExpr.Args = append(callExpr.Args, arg)
		return true, nil
	}

	return false, errors.New("http.NewGatewayHandler call not found in config/http/services.go")
}

func valueSpecs(genDecl *dst.GenDecl) []*dst.ValueSpec {
	result := make([]*dst.ValueSpec, 0, len(genDecl.Specs))
	for _, spec := range genDecl.Specs {
		valueSpec, ok := spec.(*dst.ValueSpec)
		if ok {
			result = append(result, valueSpec)
		}
	}
	return result
}

func usecaseConstructorName(usecase httpUsecaseSpec) string {
	base := strings.TrimSuffix(usecase.TypeName, "Usecase")
	return "New" + base + "Usecase"
}

func usecaseCtrlName(usecase httpUsecaseSpec) string {
	return strings.TrimSuffix(usecase.FieldName, "Usecase") + "Ctrl"
}

func findModuleRouteBlock(body *dst.BlockStmt, controllerPath string) *dst.BlockStmt {
	if body == nil {
		return nil
	}
	for _, stmt := range body.List {
		block, ok := stmt.(*dst.BlockStmt)
		if !ok {
			continue
		}
		if blockControllerPath(block) == controllerPath {
			return block
		}
	}
	return nil
}

func blockControllerPath(block *dst.BlockStmt) string {
	if block == nil {
		return ""
	}
	for _, stmt := range block.List {
		assignStmt, ok := stmt.(*dst.AssignStmt)
		if !ok || len(assignStmt.Rhs) != 1 {
			continue
		}
		literal, ok := assignStmt.Rhs[0].(*dst.BasicLit)
		if !ok || literal.Kind != token.STRING {
			continue
		}
		value, err := strconv.Unquote(literal.Value)
		if err != nil {
			continue
		}
		if strings.HasPrefix(value, "/") {
			return value
		}
	}
	return ""
}

func existingRouteStatements(block *dst.BlockStmt) map[string]struct{} {
	result := make(map[string]struct{})
	if block == nil {
		return result
	}
	for _, stmt := range block.List {
		method, routePath, funcName, ok := parseRouteStatement(stmt)
		if !ok {
			continue
		}
		result[routeStatementKey(method, routePath, funcName)] = struct{}{}
	}
	return result
}

func parseRouteStatement(stmt dst.Stmt) (method string, routePath string, funcName string, ok bool) {
	exprStmt, ok := stmt.(*dst.ExprStmt)
	if !ok {
		return "", "", "", false
	}
	callExpr, ok := exprStmt.X.(*dst.CallExpr)
	if !ok {
		return "", "", "", false
	}
	methodSelector, ok := callExpr.Fun.(*dst.SelectorExpr)
	if !ok || len(callExpr.Args) < 2 {
		return "", "", "", false
	}
	routerIdent, ok := methodSelector.X.(*dst.Ident)
	if !ok || routerIdent.Name != "router" {
		return "", "", "", false
	}

	pathLiteral, ok := callExpr.Args[0].(*dst.BasicLit)
	if !ok || pathLiteral.Kind != token.STRING {
		return "", "", "", false
	}
	unquotedPath, err := strconv.Unquote(pathLiteral.Value)
	if err != nil {
		return "", "", "", false
	}

	handlerCall, ok := callExpr.Args[1].(*dst.CallExpr)
	if !ok {
		return "", "", "", false
	}
	handlerSelector, ok := handlerCall.Fun.(*dst.SelectorExpr)
	if !ok || handlerSelector.Sel == nil {
		return "", "", "", false
	}

	return methodSelector.Sel.Name, unquotedPath, handlerSelector.Sel.Name, true
}

func routeStatementKey(method, routePath, funcName string) string {
	return method + "|" + routePath + "|" + funcName
}

func newModuleRouteBlock(spec httpRouteSpec) *dst.BlockStmt {
	stmts := []dst.Stmt{
		&dst.AssignStmt{
			Lhs: []dst.Expr{dst.NewIdent("controller")},
			Tok: token.DEFINE,
			Rhs: []dst.Expr{
				&dst.BasicLit{Kind: token.STRING, Value: strconv.Quote("/" + spec.ModuleName)},
			},
		},
		&dst.AssignStmt{
			Lhs: []dst.Expr{dst.NewIdent("router")},
			Tok: token.DEFINE,
			Rhs: []dst.Expr{
				&dst.CallExpr{
					Fun: &dst.SelectorExpr{
						X: &dst.SelectorExpr{
							X:   dst.NewIdent("ox"),
							Sel: dst.NewIdent("service"),
						},
						Sel: dst.NewIdent("Group"),
					},
					Args: []dst.Expr{dst.NewIdent("controller")},
				},
			},
		},
	}

	for _, route := range spec.Routes {
		stmts = append(stmts, newRouteStatement(spec.ModuleName, route))
	}

	return &dst.BlockStmt{List: stmts}
}

func newRouteStatement(moduleName string, route httpRoute) dst.Stmt {
	handlerArgs := []dst.Expr{}
	switch route.ArgKind {
	case routeArgTrxBuilder:
		handlerArgs = append(handlerArgs, &dst.SelectorExpr{
			X:   dst.NewIdent("ox"),
			Sel: dst.NewIdent("trxBuilder"),
		})
	case routeArgUsecase:
		handlerArgs = append(handlerArgs, &dst.SelectorExpr{
			X:   dst.NewIdent("ox"),
			Sel: dst.NewIdent(route.UsecaseField),
		})
	}

	return &dst.ExprStmt{
		X: &dst.CallExpr{
			Fun: &dst.SelectorExpr{
				X:   dst.NewIdent("router"),
				Sel: dst.NewIdent(route.Method),
			},
			Args: []dst.Expr{
				&dst.BasicLit{Kind: token.STRING, Value: strconv.Quote(route.Path)},
				&dst.CallExpr{
					Fun: &dst.SelectorExpr{
						X:   dst.NewIdent(moduleName),
						Sel: dst.NewIdent(route.FuncName),
					},
					Args: handlerArgs,
				},
			},
		},
	}
}
