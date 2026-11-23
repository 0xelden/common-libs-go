package general

import (
	"context"
	"strings"
	"testing"

	"github.com/0xelden/common-libs-go/serror"
	"github.com/0xelden/common-libs-go/service"
	sq "github.com/Masterminds/squirrel"
)

type insertCapture struct {
	service.GeneralRepo
	sql string
}

func (c *insertCapture) InsertRows(_ context.Context, stmt sq.InsertBuilder) (int64, serror.SError) {
	c.sql, _, _ = stmt.ToSql()
	return 1, nil
}

func TestInsertRowsSqlSuffix(t *testing.T) {
	const suffix = "ON CONFLICT (id) DO NOTHING"
	rows := []noIdDto{{Name: "a"}}

	t.Run("applied via WithSqlSuffix", func(t *testing.T) {
		repo := &insertCapture{}
		if _, serr := NewDTO[noIdDto]("t", repo).InsertRows(WithSqlSuffix(context.Background(), suffix), rows, nil); serr != nil {
			t.Fatal(serr)
		}
		if !strings.HasSuffix(repo.sql, suffix) {
			t.Fatalf("suffix not applied: %s", repo.sql)
		}
	})

	t.Run("ignored from a plain string context key", func(t *testing.T) {
		repo := &insertCapture{}
		ctx := context.WithValue(context.Background(), "sql_suffix", suffix) //nolint:staticcheck
		if _, serr := NewDTO[noIdDto]("t", repo).InsertRows(ctx, rows, nil); serr != nil {
			t.Fatal(serr)
		}
		if strings.Contains(repo.sql, "ON CONFLICT") {
			t.Fatalf("string key injected suffix: %s", repo.sql)
		}
	})
}
