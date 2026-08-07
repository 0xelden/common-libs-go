package helper

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-errors/errors"
	"github.com/0xelden/common-libs-go/helper/sqlstring"
	"github.com/0xelden/common-libs-go/logger"
)

// EscapeSQL escape sql string untuk menghindari sql injection attack
func EscapeSQL(str string) string {
	tz, _ := time.LoadLocation(Env("APP_TIMEZONE", "Asia/Jakarta"))
	return sqlstring.EscapeInLocation(str, tz)
}

// FormatSQL format dan escape sql string untuk menghindari sql injection attack
func FormatSQL(stmt string, args ...interface{}) string {
	tz, _ := time.LoadLocation(Env("APP_TIMEZONE", "Asia/Jakarta"))
	return sqlstring.FormatInLocation(stmt, tz, args...)
}

// NamedToNumberedSQL mengubah named sql statement parameter menjadi numbered parameter
// untuk menggunakan fitur named sql param di database dan atau driver yg tidak support
// see: https://github.com/lib/pq/issues/534
func NamedToNumberedSQL(stmt string, args ...sql.NamedArg) (newStmt string, newArgs []interface{}, err error) {
	if len(args) == 0 {
		return "", nil, errors.New("error empty parameter")
	}
	var (
		seen = make(map[string]int, len(args))
		keys = make([]string, 0, len(args)*2)
		vals = make([]interface{}, 0, len(args))
	)
	sort.Slice(args, func(i, j int) bool {
		return len(args[i].Name) > len(args[j].Name)
	})
	for i, a := range args {
		if _, ok := seen[a.Name]; ok {
			return "", nil, errors.New("error duplicate keys provided: " + a.Name)
		}
		keys = append(keys, fmt.Sprintf("@%s", a.Name), fmt.Sprintf("$%d", i+1))
		vals = append(vals, a.Value)
		seen[a.Name] = 1
	}
	return strings.NewReplacer(keys...).Replace(stmt), vals, nil
}

// GenPatchStatement membuat sql statement lengkap dengan parameter beserta argumentnya
// patchData adalah struct dengan field yg bersifat nullable
// default id field adalah "id"
func GenPatchStatement(table string, patchData interface{}, id ...string) (stmt string, args []interface{}) {
	var (
		num     = 1
		maps    = StructToMap(patchData, "db")
		sets    = make([]string, 0, len(maps))
		idField = "id"
	)

	// remove nullable from map
	for k, v := range maps {
		if v == nil {
			delete(maps, k)
		}
	}

	if len(id) > 0 && id[0] != "" {
		idField = id[0]
	}

	idValue := maps[idField]
	args = make([]interface{}, 0, len(maps))
	delete(maps, idField)

	for k, v := range maps {
		sets = append(sets, k+" = $"+strconv.Itoa(num))
		args = append(args, v)
		num++
	}

	args = append(args, idValue)
	stmt = fmt.Sprintf(`UPDATE %s SET %s WHERE id = $%d returning *`,
		table,
		strings.Join(sets, ", "),
		num,
	)

	return stmt, args
}

var (
	// safeIdentRe matches an unquoted identifier that PostgreSQL parses as a
	// plain name, so it can be emitted as-is.
	safeIdentRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_$]*$`)

	// quotedIdentRe matches an identifier that is already correctly quoted,
	// i.e. wrapped in double quotes with every inner quote doubled.
	quotedIdentRe = regexp.MustCompile(`^"([^"]|"")*"$`)
)

// QuoteIdent quotes s as a PostgreSQL identifier, doubling every embedded
// double quote.
//
// It exists because strconv.Quote is not a SQL escaper: it emits Go escaping
// (\") where PostgreSQL expects SQL escaping (""). PostgreSQL treats the
// backslash inside a quoted identifier as an ordinary character, so the \"
// closes the identifier early and everything after it is parsed as SQL --
// turning a sort/column parameter into an injection point.
func QuoteIdent(s string) string {
	// NUL cannot appear in an identifier and would truncate the statement.
	s = strings.ReplaceAll(s, "\x00", "")
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// quoteIdentSegment renders one dot-separated segment of a column reference.
// Bare safe identifiers and already-quoted identifiers pass through unchanged
// so existing "alias.column" and `"schema".table.column` forms keep resolving
// the same way; anything else is quoted, which makes it inert -- it can then
// only ever name a (probably non-existent) column, never break out into SQL.
func quoteIdentSegment(seg string) string {
	if safeIdentRe.MatchString(seg) || quotedIdentRe.MatchString(seg) {
		return seg
	}
	return QuoteIdent(seg)
}

// quoteLeafIdent renders the final segment of a column reference, which is
// always quoted so that reserved words ("user", "order", "date") keep working
// as column names. Already-quoted input is left alone to stay idempotent.
func quoteLeafIdent(seg string) string {
	if quotedIdentRe.MatchString(seg) {
		return seg
	}
	return QuoteIdent(seg)
}

func escapeColumn(col string) string {
	if len(col) == 0 {
		return col
	}
	part := strings.Split(col, ".")
	if len(part) == 1 {
		return quoteLeafIdent(strings.TrimSpace(col))
	}
	// Every segment is checked, not just the last one: the prefix used to be
	// copied through verbatim, so a column like "x; DROP TABLE y; --.z" ended
	// up emitting its prefix as raw SQL.
	last := len(part) - 1
	for i, seg := range part {
		trimmed := strings.TrimSpace(seg)
		if i == last {
			part[i] = quoteLeafIdent(trimmed)
			continue
		}
		// Leading whitespace is significant to EscapeColumnStmt, which splits
		// on "," and relies on the ", " spacing surviving the round trip.
		part[i] = strings.TrimSuffix(seg, trimmed) + quoteIdentSegment(trimmed)
	}
	return strings.Join(part, ".")
}

func EscapeColumnStmt(stmt string) string {
	cols := []string{}
	for _, v := range strings.Split(stmt, ",") {
		cols = append(cols, escapeColumn(v))
	}
	return strings.Join(cols, ",")
}

func EscapeColumns(cols []string) string {
	res := []string{}
	for _, v := range cols {
		res = append(res, escapeColumn(v))
	}
	return strings.Join(res, ",")
}

func EqNullOrEmptyStr(key string, value *string) string {
	if value == nil || (value != nil && *value == "") {
		return fmt.Sprintf(`(%[1]s is null or %[1]s = '')`, key)
	}
	return FormatSQL(fmt.Sprintf(`%s = ?`, key), value)
}

func EqNullOrEmptyDate(key string, value *time.Time) string {
	if value == nil {
		return fmt.Sprintf(`(%[1]s is null)`, key)
	}
	return FormatSQL(fmt.Sprintf(`%s = ?`, key), value)
}

// SQLFragmentHasStatementBreak reports whether a generated SQL fragment (a
// WHERE or ORDER BY clause that gets interpolated into a query template)
// contains a statement terminator, a comment introducer, or an unterminated
// string literal -- none of which a legitimate fragment ever needs.
//
// This is the last line of defence for the count query in api.RowIndex.List,
// which is executed without bind arguments. lib/pq sends argument-less queries
// over the simple protocol (see conn.go: `if len(args) == 0 { simpleQuery }`),
// and the simple protocol happily executes several statements in one round
// trip -- so a `;` reaching that query means arbitrary DDL/DML, not just a
// leaked row.
//
// It is deliberately structural rather than a keyword blocklist: occurrences
// inside quoted literals are skipped, so a genuine filter such as
// name ILIKE '%a--b%' or note = 'x; y' still passes.
func SQLFragmentHasStatementBreak(fragment string) bool {
	const (
		plain = iota
		inSingleQuote
		inDoubleQuote
	)

	state := plain
	for i := 0; i < len(fragment); i++ {
		switch c := fragment[i]; state {
		case inSingleQuote:
			if c == '\'' {
				if i+1 < len(fragment) && fragment[i+1] == '\'' {
					i++ // '' is an escaped quote, the literal continues
					continue
				}
				state = plain
			}
		case inDoubleQuote:
			if c == '"' {
				if i+1 < len(fragment) && fragment[i+1] == '"' {
					i++ // "" is an escaped quote, the identifier continues
					continue
				}
				state = plain
			}
		default:
			switch {
			case c == '\'':
				state = inSingleQuote
			case c == '"':
				state = inDoubleQuote
			case c == ';':
				return true
			case c == '-' && i+1 < len(fragment) && fragment[i+1] == '-':
				return true
			case c == '/' && i+1 < len(fragment) && fragment[i+1] == '*':
				return true
			}
		}
	}

	// An unclosed literal would swallow the remainder of the query template.
	return state != plain
}

func ExprContainsMaliciousSqlKeyword(str string) bool {
	keywords := [55]string{
		"union", "drop", "table", "database", "--", "/*", "#", "shutdown", "exec", ";", "select", "* ", " *", "having", "1=1", "'a'='a'", "'x'='x'",
		"''=''", `""=""`, "'1'='1'", "1 = 1", "'a' = 'a'", "'x' = 'x'", "'' = ''", `"" = ""`, "'1' = '1'", "update", "delete", "insert", "set", "into",
		"from", "mid", "version", "order by", "group by", `\\`, `//`, `' or "`, `' or '1`, `'like'`, `'=0`, `%00`, `1-false`,
		`1-true`, `1*56`, "`", "where", "case", "delay", "wait", "sleep", "md5", "benchmark", "convert",
	}
	input := strings.ToLower(str)
	for _, v := range keywords {
		if strings.Contains(input, v) {
			return true
		}
	}
	return false
}

// PgDump executes the pg_dump command with dbName as a parameter.
func PgDump(ctx context.Context, dbName string) error {
	if !IsCliExists("pg_dump") {
		return errors.New("pg_dump is not installed")
	}
	if !IsCliExists("gzip") {
		return errors.New("gzip is not installed")
	}

	// Load required environment variables
	dbPwd, dbHost, dbPort, dbUser := os.Getenv("DB_PWD"), os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), os.Getenv("DB_USER")
	if dbPwd == "" || dbHost == "" || dbPort == "" || dbUser == "" || dbName == "" {
		return fmt.Errorf("missing required environment variables or parameters")
	}

	// Allow configurable dump directory
	dumpDir := os.Getenv("DUMP_DIR")
	if dumpDir == "" {
		dumpDir = "dump"
	}

	if err := os.MkdirAll(dumpDir, 0755); err != nil {
		return fmt.Errorf("failed to create dump directory: %v", err)
	}

	// Generate timestamped file path
	timestamp := time.Now().Format("20060102_150405")
	filePath := filepath.Join(dumpDir, fmt.Sprintf("%s_dump_%s.sql.gz", dbName, timestamp))

	// Use direct execution instead of `bash -c`
	cmd := exec.Command("pg_dump", "-h", dbHost, "-p", dbPort, "-U", dbUser, "-d", dbName)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", dbPwd))

	// Pipe output to gzip
	gzipCmd := exec.Command("gzip")

	// Create output file
	outFile, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create dump file: %v", err)
	}
	//goland:noinspection GoUnhandledErrorResult
	defer outFile.Close()

	// Set up pipeline: pg_dump → gzip → file
	reader, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %v", err)
	}
	gzipCmd.Stdin = reader
	gzipCmd.Stdout = outFile

	// Start both processes
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error executing pg_dump: %v", err)
	}
	if err := gzipCmd.Start(); err != nil {
		return fmt.Errorf("error executing gzip: %v", err)
	}

	// Wait for completion
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("pg_dump failed: %v", err)
	}
	if err := gzipCmd.Wait(); err != nil {
		return fmt.Errorf("gzip failed: %v", err)
	}

	logger.Infof("Dump of %s saved at %s", dbName, filePath)

	TraceUnique(ctx, dbName, filePath)

	// Allow configurable retention policy
	keepN := 7
	if envKeepN := os.Getenv("DUMP_KEEP_N"); envKeepN != "" {
		if parsedKeepN, err := strconv.Atoi(envKeepN); err == nil && parsedKeepN > 0 {
			keepN = parsedKeepN
		}
	}

	return CleanOldDumps(keepN, dumpDir, dbName)
}

// CleanOldDumps keeps only the latest N dump files and deletes older ones.
func CleanOldDumps(keepN int, dumpDir, dbName string) error {
	files, err := os.ReadDir(dumpDir)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	var dumpFiles []os.DirEntry
	for _, file := range files {
		if !file.IsDir() && strings.HasPrefix(file.Name(), fmt.Sprintf("%s_dump_", dbName)) && strings.HasSuffix(file.Name(), ".sql.gz") {
			dumpFiles = append(dumpFiles, file)
		}
	}

	// Sort files by modification time (the newest first)
	sort.Slice(dumpFiles, func(i, j int) bool {
		iInfo, _ := dumpFiles[i].Info()
		jInfo, _ := dumpFiles[j].Info()
		return iInfo.ModTime().After(jInfo.ModTime())
	})

	// Keep latest keepN, delete older ones
	if len(dumpFiles) > keepN {
		for _, file := range dumpFiles[keepN:] {
			fullPath := filepath.Join(dumpDir, file.Name())
			if err := os.Remove(fullPath); err != nil {
				logger.Infof("Failed to delete %s: %v", fullPath, err)
			} else {
				logger.Infof("Deleted old dump: %s", fullPath)
			}
		}
	}

	return nil
}
