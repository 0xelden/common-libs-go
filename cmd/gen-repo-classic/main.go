package main

import (
	"database/sql"
	"flag"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-errors/errors"
	"github.com/0xelden/common-libs-go/connection"
)

type config struct {
	ddlPath string
	inline  string
	outDir  string
	route   string
	urlPath string
	force   bool
	hard    bool
	dryRun  bool
	tables  []string
	withCtl bool
	postman bool
}

const defaultServiceHTTPRoutePath = "service/http/route.go"

func main() {
	cfg := parseFlags()
	if err := run(cfg); err != nil {
		//goland:noinspection GoUnhandledErrorResult
		fmt.Fprintf(os.Stderr, "gen-repo: %v\n", err)
		os.Exit(1)
	}
}

func parseFlags() config {
	var cfg config
	var tables string
	flag.StringVar(&cfg.ddlPath, "ddl", "", "path to the SQL DDL file; leave empty to read from stdin")
	flag.StringVar(&tables, "tables", "", "comma-separated fully qualified table names to generate repositories from (schema.table) or schema names to include all tables")
	flag.StringVar(&cfg.outDir, "out", ".", "repository root where files will be generated")
	flag.StringVar(&cfg.route, "route", defaultServiceHTTPRoutePath, "path to the HTTP route file to patch; relative paths are resolved from --out")
	flag.StringVar(&cfg.urlPath, "url-path", "", "optional URL path prefix for generated Postman requests, inserted between {{gateway}} and the controller path")
	flag.BoolVar(&cfg.force, "force", false, "overwrite generated files if they already exist")
	flag.BoolVar(&cfg.hard, "hard", false, "rewrite generated files and ignore existing content")
	flag.BoolVar(&cfg.dryRun, "dry-run", false, "print generated files without writing to disk")
	flag.BoolVar(&cfg.withCtl, "ctl", false, "generate controller scaffold")
	flag.BoolVar(&cfg.postman, "postman", false, "generate a standalone {name}.collections.json Postman collection from the built-in ERP HKHD template")
	flag.Parse()
	cfg.tables = parseTableList(tables)
	if len(cfg.tables) == 0 && cfg.ddlPath == "" && flag.NArg() > 0 {
		cfg.inline = strings.Join(flag.Args(), " ")
	}
	return cfg
}

func run(cfg config) error {
	outDir, err := filepath.Abs(cfg.outDir)
	if err != nil {
		return errors.Errorf("resolve output directory: %v", err)
	}
	modulePath, err := modulePathFromGoMod(outDir)
	if err != nil {
		return errors.Errorf("determine module path: %v", err)
	}

	if len(cfg.tables) > 0 && (cfg.ddlPath != "" || cfg.inline != "") {
		return errors.New("use --tables or provide DDL, not both")
	}

	if len(cfg.tables) > 0 {
		db, err := connection.ConnectPG()
		if err != nil {
			return errors.Errorf("connect to postgres: %v", err)
		}
		//goland:noinspection GoUnhandledErrorResult
		defer db.Close()

		targets := make([]string, 0, len(cfg.tables))
		seen := make(map[string]struct{}, len(cfg.tables))
		for _, ident := range cfg.tables {
			if strings.Contains(ident, ".") {
				if _, ok := seen[ident]; ok {
					continue
				}
				seen[ident] = struct{}{}
				targets = append(targets, ident)
				continue
			}

			exists, err := schemaExists(db, ident)
			if err != nil {
				return err
			}
			if exists {
				tableNames, err := loadTablesForSchema(db, ident)
				if err != nil {
					return err
				}
				if len(tableNames) == 0 {
					return errors.Errorf("schema %s has no tables", ident)
				}
				for _, tableName := range tableNames {
					fqn := fmt.Sprintf("%s.%s", ident, tableName)
					if _, ok := seen[fqn]; ok {
						continue
					}
					seen[fqn] = struct{}{}
					targets = append(targets, fqn)
				}
				continue
			}

			fqn := fmt.Sprintf("public.%s", ident)
			if _, ok := seen[fqn]; ok {
				continue
			}
			seen[fqn] = struct{}{}
			targets = append(targets, fqn)
		}

		for _, fqn := range targets {
			table, err := loadTableDefinition(db, fqn)
			if err != nil {
				return err
			}
			if err := processTable(cfg, outDir, modulePath, table); err != nil {
				return err
			}
		}
		return nil
	}

	sqlDDL, err := readDDL(cfg.ddlPath, cfg.inline)
	if err != nil {
		return err
	}

	table, err := ParseCreateTable(sqlDDL)
	if err != nil {
		return err
	}

	return processTable(cfg, outDir, modulePath, table)
}

func processTable(cfg config, outDir, modulePath string, table *Table) error {
	dtoTags, err := loadDTOFieldTags(outDir, table)
	if err != nil {
		return err
	}
	files, err := generateFiles(table, modulePath, dtoTags, cfg.withCtl)
	if err != nil {
		return err
	}

	for _, file := range files {
		data := []byte(file.Content)
		if strings.HasSuffix(file.Path, ".go") {
			formatted, fmtErr := format.Source(data)
			if fmtErr == nil {
				data = formatted
			}
		}

		targetPath := filepath.Join(outDir, file.Path)
		content, shouldWrite, err := renderGeneratedFile(cfg, targetPath, file, string(data))
		if err != nil {
			return err
		}
		if !shouldWrite {
			continue
		}
		if cfg.dryRun {
			fmt.Printf("---- %s ----\n%s\n", file.Path, content)
			continue
		}

		if err = os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return errors.Errorf("ensure directory %s: %v", filepath.Dir(targetPath), err)
		}

		if err = os.WriteFile(targetPath, []byte(content), 0o644); err != nil {
			return errors.Errorf("write %s: %v", targetPath, err)
		}
	}

	registryFiles, err := updateRegistryFiles(outDir, modulePath, table, cfg.withCtl)
	if err != nil {
		return err
	}
	for _, file := range registryFiles {
		data := []byte(file.Content)
		if strings.HasSuffix(file.Path, ".go") {
			formatted, fmtErr := format.Source(data)
			if fmtErr == nil {
				data = formatted
			}
		}

		targetPath := filepath.Join(outDir, file.Path)
		content, shouldWrite, err := renderGeneratedFile(cfg, targetPath, file, string(data))
		if err != nil {
			return err
		}
		if !shouldWrite {
			continue
		}
		if cfg.dryRun {
			fmt.Printf("---- %s ----\n%s\n", file.Path, content)
			continue
		}

		if err = os.WriteFile(targetPath, []byte(content), 0o644); err != nil {
			return errors.Errorf("write %s: %v", targetPath, err)
		}
	}

	if cfg.withCtl {
		generatedHTTP := ""
		for _, file := range files {
			if strings.HasSuffix(file.Path, ".http.go") {
				generatedHTTP = file.Content
				break
			}
		}
		if generatedHTTP != "" {
			routeTargetPath := resolveOutputPath(outDir, cfg.route)
			content, shouldWrite, err := patchServiceHTTPRoutes(routeTargetPath, modulePath, generatedHTTP)
			if err != nil {
				return err
			}
			if shouldWrite {
				if cfg.dryRun {
					fmt.Printf("---- %s ----\n%s\n", cfg.route, content)
				} else if err = os.WriteFile(routeTargetPath, []byte(content), 0o644); err != nil {
					return errors.Errorf("write %s: %v", routeTargetPath, err)
				}
			}

			content, shouldWrite, err = patchServiceHTTPHandler(outDir, generatedHTTP)
			if err != nil {
				return err
			}
			if shouldWrite {
				if cfg.dryRun {
					fmt.Printf("---- %s ----\n%s\n", filepath.Join("service", "http", "handler.go"), content)
				} else if err = os.WriteFile(filepath.Join(outDir, "service", "http", "handler.go"), []byte(content), 0o644); err != nil {
					return errors.Errorf("write %s: %v", filepath.Join(outDir, "service", "http", "handler.go"), err)
				}
			}

			content, shouldWrite, err = patchHTTPServicesConfig(outDir, modulePath, generatedHTTP)
			if err != nil {
				return err
			}
			if shouldWrite {
				if cfg.dryRun {
					fmt.Printf("---- %s ----\n%s\n", filepath.Join("config", "http", "services.go"), content)
				} else if err = os.WriteFile(filepath.Join(outDir, "config", "http", "services.go"), []byte(content), 0o644); err != nil {
					return errors.Errorf("write %s: %v", filepath.Join(outDir, "config", "http", "services.go"), err)
				}
			}
		}
	}

	if cfg.postman {
		fileName, content, shouldWrite, err := patchPostmanCollection(outDir, modulePath, table, cfg.urlPath)
		if err != nil {
			return err
		}
		if shouldWrite {
			targetPath := postmanCollectionPath(outDir, fileName)
			if cfg.dryRun {
				fmt.Printf("---- %s ----\n%s\n", filepath.Join("postman", fileName), content)
			} else {
				if err = os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
					return errors.Errorf("ensure directory %s: %v", filepath.Dir(targetPath), err)
				}
				if err = os.WriteFile(targetPath, []byte(content), 0o644); err != nil {
					return errors.Errorf("write %s: %v", targetPath, err)
				}
			}
		}
	}

	if cfg.dryRun {
		return nil
	}
	target := table.Name
	if table.Schema != "" {
		target = fmt.Sprintf("%s.%s", table.Schema, table.Name)
	}
	fmt.Printf("Generated repository scaffolding for %s\n", target)
	return nil
}

func resolveOutputPath(outDir, target string) string {
	if filepath.IsAbs(target) {
		return target
	}
	return filepath.Join(outDir, filepath.FromSlash(target))
}

func parseTableList(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func readDDL(path, inline string) (string, error) {
	if inline != "" {
		return inline, nil
	}

	if path == "" {
		stat, err := os.Stdin.Stat()
		if err != nil {
			return "", errors.Errorf("stat stdin: %v", err)
		}
		if (stat.Mode() & os.ModeCharDevice) != 0 {
			return "", errors.New("no DDL provided; pass --ddl or pipe SQL via stdin")
		}
		data, err := os.ReadFile("/dev/stdin")
		if err != nil {
			return "", errors.Errorf("read stdin: %v", err)
		}
		return string(data), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", errors.Errorf("read ddl file: %v", err)
	}
	return string(data), nil
}

func loadTableDefinition(db *sql.DB, fqn string) (*Table, error) {
	schema, name := splitSchemaAndTable(fqn)
	if name == "" {
		return nil, errors.Errorf("invalid table identifier: %s", fqn)
	}
	if schema == "" {
		schema = "public"
	}

	pkColumns, err := loadPrimaryKeyColumns(db, schema, name)
	if err != nil {
		return nil, errors.Errorf("load primary keys for %s.%s: %v", schema, name, err)
	}

	query := `
select
    column_name,
    data_type,
    udt_name,
    is_nullable = 'NO' as not_null,
    column_default is not null as has_default,
    is_generated = 'ALWAYS' as is_generated,
    coalesce(generation_expression, '') as generation_expression
from information_schema.columns
where table_schema = $1 and table_name = $2
order by ordinal_position;
`
	rows, err := db.Query(query, schema, name)
	if err != nil {
		return nil, errors.Errorf("describe columns for %s.%s: %v", schema, name, err)
	}
	//goland:noinspection GoUnhandledErrorResult
	defer rows.Close()

	var columns []Column
	for rows.Next() {
		var (
			colName              string
			dataType             string
			udtName              string
			notNull              bool
			hasDefault           bool
			isGenerated          bool
			generationExpression string
		)
		if err = rows.Scan(&colName, &dataType, &udtName, &notNull, &hasDefault, &isGenerated, &generationExpression); err != nil {
			return nil, errors.Errorf("scan columns for %s.%s: %v", schema, name, err)
		}
		rawType := rawTypeFromColumn(dataType, udtName)
		column := Column{
			Name:                 colName,
			RawType:              rawType,
			NotNull:              notNull,
			HasDefault:           hasDefault,
			IsGenerated:          isGenerated,
			GenerationExpression: strings.TrimSpace(generationExpression),
		}
		if _, ok := pkColumns[strings.ToLower(colName)]; ok {
			column.IsPrimary = true
			column.NotNull = true
		}
		columns = append(columns, column)
	}
	if err = rows.Err(); err != nil {
		return nil, errors.Errorf("iterate columns for %s.%s: %v", schema, name, err)
	}
	if len(columns) == 0 {
		return nil, errors.Errorf("table %s.%s not found or has no columns", schema, name)
	}

	return &Table{
		Schema:  schema,
		Name:    name,
		Columns: columns,
	}, nil
}

func schemaExists(db *sql.DB, schema string) (bool, error) {
	query := `select 1 from information_schema.schemata where schema_name = $1;`
	var marker int
	err := db.QueryRow(query, schema).Scan(&marker)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, errors.Errorf("check schema %s: %v", schema, err)
	}
	return true, nil
}

func loadTablesForSchema(db *sql.DB, schema string) ([]string, error) {
	query := `
select table_name
from information_schema.tables
where table_schema = $1
  and table_type = 'BASE TABLE'
order by table_name;
`
	rows, err := db.Query(query, schema)
	if err != nil {
		return nil, errors.Errorf("list tables for schema %s: %v", schema, err)
	}
	//goland:noinspection GoUnhandledErrorResult
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			return nil, errors.Errorf("scan tables for schema %s: %v", schema, err)
		}
		tables = append(tables, name)
	}
	if err = rows.Err(); err != nil {
		return nil, errors.Errorf("iterate tables for schema %s: %v", schema, err)
	}
	return tables, nil
}

func loadPrimaryKeyColumns(db *sql.DB, schema, name string) (map[string]struct{}, error) {
	query := `
select kcu.column_name
from information_schema.table_constraints tc
join information_schema.key_column_usage kcu
	on tc.constraint_name = kcu.constraint_name
	and tc.table_schema = kcu.table_schema
where tc.constraint_type = 'PRIMARY KEY'
  and tc.table_schema = $1
  and tc.table_name = $2;
`
	rows, err := db.Query(query, schema, name)
	if err != nil {
		return nil, errors.Errorf("query primary key columns for %s.%s: %v", schema, name, err)
	}
	//goland:noinspection GoUnhandledErrorResult
	defer rows.Close()

	result := map[string]struct{}{}
	for rows.Next() {
		var column string
		if err = rows.Scan(&column); err != nil {
			return nil, errors.Errorf("scan primary key columns for %s.%s: %v", schema, name, err)
		}
		result[strings.ToLower(column)] = struct{}{}
	}
	if err = rows.Err(); err != nil {
		return nil, errors.Errorf("iterate primary key columns for %s.%s: %v", schema, name, err)
	}
	return result, nil
}

func rawTypeFromColumn(dataType, udtName string) string {
	lowerData := strings.ToLower(strings.TrimSpace(dataType))
	switch lowerData {
	case "user-defined":
		if udtName != "" {
			return udtName
		}
	case "array":
		if udtName != "" {
			trimmed := strings.TrimPrefix(udtName, "_")
			if trimmed != udtName {
				return trimmed + "[]"
			}
			return udtName + "[]"
		}
	}
	if lowerData != "" {
		return lowerData
	}
	return udtName
}
