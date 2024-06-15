package helper

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/0xelden/common-libs-go/types"
)

func TestNamedToNumberedSQL(t *testing.T) {
	type args struct {
		stmt string
		args []sql.NamedArg
	}
	tests := []struct {
		name        string
		args        args
		wantErr     bool
		wantNewStmt string
		wantNewArgs []interface{}
	}{
		{
			name: "correct usage",
			args: args{
				stmt: "SELECT * FROM test WHERE foo = @foo AND bar = @bar",
				args: []sql.NamedArg{
					{Name: "foo", Value: 1},
					{Name: "bar", Value: "foobar"},
				},
			},
			wantErr:     false,
			wantNewStmt: "SELECT * FROM test WHERE foo = $1 AND bar = $2",
			wantNewArgs: []interface{}{
				1,
				"foobar",
			},
		},
		{
			name: "correct usage, substring name",
			args: args{
				stmt: "SELECT * FROM test WHERE foo = @foo AND bar = @bar AND foobar = @foobar",
				args: []sql.NamedArg{
					{Name: "foo", Value: 1},
					{Name: "bar", Value: "bar"},
					{Name: "foobar", Value: "foobar"},
				},
			},
			wantErr:     false,
			wantNewStmt: "SELECT * FROM test WHERE foo = $2 AND bar = $3 AND foobar = $1",
			wantNewArgs: []interface{}{
				"foobar",
				1,
				"bar",
			},
		},
		{
			name: "correct usage",
			args: args{
				stmt: "SELECT * FROM test WHERE foo = @foo AND bar = @bar AND baz = @baz",
				args: []sql.NamedArg{
					{Name: "baz", Value: "x"},
					{Name: "bar", Value: "y"},
					{Name: "foo", Value: 1},
				},
			},
			wantErr:     false,
			wantNewStmt: "SELECT * FROM test WHERE foo = $3 AND bar = $2 AND baz = $1",
			wantNewArgs: []interface{}{
				"x",
				"y",
				1,
			},
		},
		{
			name: "error duplicate name",
			args: args{
				stmt: "SELECT * FROM test WHERE foo = @foo AND bar = @bar AND foo = @foo",
				args: []sql.NamedArg{
					{Name: "foo", Value: 1},
					{Name: "bar", Value: "foobar"},
					{Name: "foo", Value: 0.42},
				},
			},
			wantErr:     true,
			wantNewStmt: "",
			wantNewArgs: nil,
		},
		{
			name: "nil args",
			args: args{
				stmt: "SELECT * FROM test WHERE foo = @foo",
				args: nil,
			},
			wantErr:     true,
			wantNewStmt: "",
			wantNewArgs: nil,
		},
		{
			name: "empty args",
			args: args{
				stmt: "SELECT * FROM test WHERE foo = @foo",
				args: []sql.NamedArg{},
			},
			wantErr:     true,
			wantNewStmt: "",
			wantNewArgs: nil,
		},
		{
			name: "empty args",
			args: args{
				stmt: "SELECT * FROM test WHERE foo = 1",
				args: []sql.NamedArg{},
			},
			wantErr:     true,
			wantNewStmt: "",
			wantNewArgs: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotNewStmt, gotNewArgs, gotErr := NamedToNumberedSQL(tt.args.stmt, tt.args.args...)
			if tt.wantErr && gotErr == nil {
				t.Errorf("NamedToNumberedSQL() gotErr = %v, want %v", gotErr == nil, tt.wantErr)
			}
			if gotNewStmt != tt.wantNewStmt {
				t.Errorf("NamedToNumberedSQL() gotNewStmt = %v, want %v", gotNewStmt, tt.wantNewStmt)
			}
			if !reflect.DeepEqual(gotNewArgs, tt.wantNewArgs) {
				t.Errorf("NamedToNumberedSQL() gotNewArgs = %v, want %v", gotNewArgs, tt.wantNewArgs)
			}
		})
	}
}

func TestFormatSQL(t *testing.T) {
	type args struct {
		stmt string
		args []interface{}
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			args: args{
				stmt: "? and ?",
				args: []interface{}{"a", "b"},
			},
			want: "'a' and 'b'",
		},
		{
			args: args{
				stmt: "foo IS NOT DISTINCT FROM ?",
				args: []interface{}{nil},
			},
			want: "foo IS NOT DISTINCT FROM NULL",
		},
		{
			args: args{
				stmt: "foo = ? and bar = ?",
				args: []interface{}{
					First(types.NewNativeDateFromString("2025-01-30")).String(),
					First(types.NewNativeTimeFromString("10:00:00")).String(),
				},
			},
			want: "foo = '2025-01-30' and bar = '10:00:00'",
		},
		{
			args: args{
				stmt: "a=? && b=? && c=?",
				args: []interface{}{"a", "b", "c"},
			},
			want: "a='a' && b='b' && c='c'",
		},
		{
			args: args{
				stmt: "a=any(array[?])",
				args: []interface{}{[]any{"a", "b", "c"}},
			},
			want: "a=any(array['a','b','c'])",
		},
		{
			args: args{
				stmt: "a in (?)",
				args: []interface{}{[]any{"a", "b", "c"}},
			},
			want: "a in ('a','b','c')",
		},
		{
			args: args{
				stmt: "a=any(array[?]::enum_foo[])",
				args: []interface{}{[]any{"a", "b", "c"}},
			},
			want: "a=any(array['a','b','c']::enum_foo[])",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatSQL(tt.args.stmt, tt.args.args...); got != tt.want {
				t.Errorf("FormatSQL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEscapeColumnStmt(t *testing.T) {
	type args struct {
		columns string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "00",
			args: args{"po.id, po.po_no, po.date"},
			want: `po."id", po."po_no", po."date"`,
		},
		{
			name: "01",
			args: args{`"order".po.id, "order".po.po_no, "order".po.date`},
			want: `"order".po."id", "order".po."po_no", "order".po."date"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EscapeColumnStmt(tt.args.columns); got != tt.want {
				t.Errorf("EscapeColumnStmt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_escapeColumn(t *testing.T) {
	type args struct {
		col string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "00",
			args: args{"my_db.public.user"},
			want: `my_db.public."user"`,
		},
		{
			name: "01",
			args: args{"public.user"},
			want: `public."user"`,
		},
		{
			name: "02",
			args: args{"user"},
			want: `"user"`,
		},
		{
			name: "03",
			args: args{""},
			want: "",
		},
		{
			name: "04",
			args: args{`"my_db".public.user`},
			want: `"my_db".public."user"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := escapeColumn(tt.args.col); got != tt.want {
				t.Errorf("escapeColumn() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNullOrEmptyStr(t *testing.T) {
	type S struct {
		F *string
		G *string
		T *time.Time
	}

	s1 := S{
		G: Ptr("bar"),
	}

	s2 := S{
		G: Ptr(""),
	}

	type args[T any] struct {
		key   string
		value *T
	}
	tests := []struct {
		name string
		args args[string]
		want string
	}{
		{
			name: "00",
			args: args[string]{
				key:   "test",
				value: s1.F,
			},
			want: "(test is null or test = '')",
		},
		{
			name: "01",
			args: args[string]{
				key:   "foo",
				value: s1.G,
			},
			want: "foo = 'bar'",
		},
		{
			name: "02",
			args: args[string]{
				key:   "test",
				value: s2.G,
			},
			want: "(test is null or test = '')",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EqNullOrEmptyStr(tt.args.key, tt.args.value); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("EqNullOrEmptyStr() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNullOrEmptyDate(t *testing.T) {
	type S struct {
		T *time.Time
	}

	tz, _ := time.LoadLocation(Env("APP_TIMEZONE", "Asia/Jakarta"))
	now := time.Now()
	s1 := S{
		T: Ptr(now),
	}
	s2 := S{}

	type args[T any] struct {
		key   string
		value *T
	}
	tests := []struct {
		name string
		args args[time.Time]
		want string
	}{
		{
			name: "00",
			args: args[time.Time]{
				key:   "test",
				value: s2.T,
			},
			want: "(test is null)",
		},
		{
			name: "01",
			args: args[time.Time]{
				key:   "foo",
				value: s1.T,
			},
			want: "foo = '" + now.In(tz).Format("2006-01-02 15:04:05.999") + "'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EqNullOrEmptyDate(tt.args.key, tt.args.value); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("EqNullOrEmptyDate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPgDump(t *testing.T) {
	if //goland:noinspection GoBoolExpressions
	1 == 1 {
		return
	}
	if err := PgDump(context.Background(), "dev_cm"); err != nil {
		t.Errorf("TestPgDump() error %v", err)
	}
}

// Test function
func TestCleanOldDumps(t *testing.T) {
	if //goland:noinspection GoBoolExpressions
	1 == 1 {
		return
	}
	// Helper function to create dummy dump files
	createDummyDumps := func(dumpDir, dbName string, count int) ([]string, error) {
		var filenames []string

		for i := 0; i < count; i++ {
			timestamp := time.Now().Unix() - int64(i*60) // Generate timestamps decreasing every minute
			filename := fmt.Sprintf("%s_dump_%d.sql.gz", dbName, timestamp)
			fullPath := filepath.Join(dumpDir, filename)

			// Create an empty file
			file, err := os.Create(fullPath)
			if err != nil {
				return nil, fmt.Errorf("failed to create file %s: %w", fullPath, err)
			}
			//goland:noinspection GoUnhandledErrorResult
			file.Close()
			filenames = append(filenames, fullPath)
		}

		// Sort filenames to ensure order
		sort.Slice(filenames, func(i, j int) bool {
			return filenames[i] > filenames[j] // Newest first
		})

		return filenames, nil
	}

	// Create a temporary directory
	dumpDir, err := os.MkdirTemp("", "test_dumps")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	//goland:noinspection GoUnhandledErrorResult
	defer os.RemoveAll(dumpDir) // Clean up after test

	dbName := "test_db"
	totalFiles := 5 // Create 5 dump files
	keepFiles := 3

	// Generate dummy dump files
	createdFiles, err := createDummyDumps(dumpDir, dbName, totalFiles)
	if err != nil {
		t.Fatalf("Failed to create dummy dump files: %v", err)
	}

	t.Log(createdFiles)

	// Run cleanup function
	if err := CleanOldDumps(keepFiles, dumpDir, dbName); err != nil {
		t.Fatalf("CleanOldDumps failed: %v", err)
	}

	// Check remaining files
	files, err := os.ReadDir(dumpDir)
	if err != nil {
		t.Fatalf("Failed to read dump directory: %v", err)
	}

	// Ensure only keepFiles files remain
	if len(files) != keepFiles {
		t.Errorf("Expected 5 files, but found %d", len(files))
	}
}
