package general

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/blockloop/scan/v2"
	"github.com/lann/builder"
	"github.com/lib/pq"
	"github.com/0xelden/common-libs-go/db"
	"github.com/0xelden/common-libs-go/helper"
	"github.com/0xelden/common-libs-go/models"
	"github.com/0xelden/common-libs-go/service"
	"github.com/0xelden/common-libs-go/shared"
	"github.com/0xelden/common-libs-go/serror"
)

var (
	ErrGeneralRepoNotFound  = errors.New("GeneralRepo: id not found")
	ErrGeneralRepoInvalidId = errors.New("GeneralRepo: invalid param.Id uuid type, exepected as: string, []string")
)

type generalRepo struct {
	db    db.Driver
	table *string // fully qualified table name for this repo
	pfmt  sq.PlaceholderFormat
}

func NewGeneralRepo(db db.Driver, tableName string, placeholderFmt ...sq.PlaceholderFormat) service.GeneralRepo {
	var pfmt sq.PlaceholderFormat = sq.Dollar
	if len(placeholderFmt) > 0 {
		pfmt = placeholderFmt[0]
	}
	var table *string
	if len(tableName) > 0 {
		table = &tableName
	}
	return &generalRepo{db: db, table: table, pfmt: pfmt}
}

func (g generalRepo) incdec(ctx context.Context, param models.FieldParam, query string) serror.SError {
	id := []string{}
	switch param.Id.(type) {
	case string:
		id = []string{param.Id.(string)}
	case []string:
		id = param.Id.([]string)
	default:
		return serror.NewFromError(ErrGeneralRepoInvalidId)
	}

	var (
		qfield = helper.QuoteIdent(param.Field)
		by     = helper.ContextGetString(ctx, shared.X_UserId)
		stmt   = fmt.Sprintf(query, param.Table, qfield, qfield)
		named  = []sql.NamedArg{
			{Name: "id", Value: pq.Array(id)},
			{Name: "by", Value: by},
			{Name: "value", Value: param.Value},
		}
	)

	stmt, args, err := helper.NamedToNumberedSQL(stmt, named...)
	if err != nil {
		return serror.NewFromError(err)
	}
	row, err := g.db.ExecContext(ctx, stmt, args...)
	if err != nil {
		return serror.NewFromError(err)
	}
	if row == nil {
		return serror.NewFromError(errors.New("GeneralRepo: ExecContext returned nil result"))
	}
	n, err := row.RowsAffected()
	if err != nil {
		return serror.NewFromError(err)
	}
	if n == 0 {
		return serror.NewFromError(ErrGeneralRepoNotFound)
	}
	return nil
}

func (g generalRepo) Driver() db.Driver {
	return g.db
}

func (g generalRepo) TableName() string {
	return helper.Deref(g.table)
}

func (g generalRepo) IncrementFields(ctx context.Context, param models.FieldParam) serror.SError {
	return g.incdec(ctx, param, IncrementField)
}

func (g generalRepo) DecrementFields(ctx context.Context, param models.FieldParam) serror.SError {
	return g.incdec(ctx, param, DecrementField)
}

func (g generalRepo) setDefaultFromClause(stmt sq.SelectBuilder) sq.SelectBuilder {
	if v, ok := builder.Get(stmt, "From"); !ok || v == "" {
		// nosemgrep
		return stmt.From(*g.table)
	}
	return stmt
}

func (g generalRepo) SelectScanRow(ctx context.Context, result interface{}, stmt sq.SelectBuilder) serror.SError {
	if g.table != nil {
		stmt = g.setDefaultFromClause(stmt)
	}
	stmt = stmt.Limit(1).PlaceholderFormat(g.pfmt)
	query, args, err := stmt.ToSql()
	if err != nil {
		return serror.NewFromError(err)
	}
	row, err := g.db.QueryContext(ctx, query, args...)
	if err != nil {
		return serror.NewFromError(err)
	}
	//goland:noinspection GoUnhandledErrorResult
	defer row.Close()
	if err := scan.Row(result, row); err != nil {
		return serror.NewFromError(err)
	}
	return nil
}

func (g generalRepo) SelectScanRows(ctx context.Context, result interface{}, stmt sq.SelectBuilder) serror.SError {
	if g.table != nil {
		stmt = g.setDefaultFromClause(stmt)
	}
	stmt = stmt.PlaceholderFormat(g.pfmt)
	query, args, err := stmt.ToSql()
	if err != nil {
		return serror.NewFromError(err)
	}
	rows, err := g.db.QueryContext(ctx, query, args...)
	if err != nil {
		return serror.NewFromError(err)
	}
	//goland:noinspection GoUnhandledErrorResult
	defer rows.Close()
	if err := scan.Rows(result, rows); err != nil {
		return serror.NewFromError(err)
	}
	return nil
}

func (g generalRepo) CountRows(ctx context.Context, stmt sq.SelectBuilder) (total int64, serr serror.SError) {
	if g.table != nil {
		stmt = g.setDefaultFromClause(stmt)
	}
	stmt = stmt.PlaceholderFormat(g.pfmt)
	query, args, err := stmt.ToSql()
	if err != nil {
		return 0, serror.NewFromError(err)
	}
	rows, err := g.db.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, serror.NewFromError(err)
	}
	//goland:noinspection GoUnhandledErrorResult
	defer rows.Close()
	if err := scan.Row(&total, rows); err != nil {
		return total, serror.NewFromError(err)
	}
	return total, nil
}

func (g generalRepo) RawSelectOne(ctx context.Context, result any, query string, args ...any) serror.SError {
	rows, err := g.db.QueryContext(ctx, query, args...)
	if err != nil {
		return serror.NewFromError(err)
	}
	//goland:noinspection GoUnhandledErrorResult
	defer rows.Close()
	if err := scan.Row(result, rows); err != nil {
		return serror.NewFromError(err)
	}
	return nil
}

func (g generalRepo) RawSelectRows(ctx context.Context, result any, query string, args ...any) serror.SError {
	rows, err := g.db.QueryContext(ctx, query, args...)
	if err != nil {
		return serror.NewFromError(err)
	}
	//goland:noinspection GoUnhandledErrorResult
	defer rows.Close()
	if err := scan.Rows(result, rows); err != nil {
		return serror.NewFromError(err)
	}
	return nil
}

func (g generalRepo) DeleteRows(ctx context.Context, stmt sq.DeleteBuilder) (affected int64, serr serror.SError) {
	if g.table != nil {
		if v, ok := builder.Get(stmt, "From"); !ok || v == "" {
			stmt = stmt.From(*g.table)
		}
	}
	stmt = stmt.PlaceholderFormat(g.pfmt)
	query, args, err := stmt.ToSql()
	if err != nil {
		return 0, serror.NewFromError(err)
	}
	rows, err := g.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, serror.NewFromError(err)
	}
	if rows == nil {
		return 0, serror.NewFromError(errors.New("GeneralRepo: ExecContext returned nil result"))
	}
	n, err := rows.RowsAffected()
	if err != nil {
		return 0, serror.NewFromError(err)
	}
	return n, nil
}

func (g generalRepo) UpdateRows(ctx context.Context, stmt sq.UpdateBuilder) (affected int64, serr serror.SError) {
	if g.table != nil {
		if v, ok := builder.Get(stmt, "Table"); !ok || v == "" {
			stmt = stmt.Table(*g.table)
		}
	}
	stmt = stmt.PlaceholderFormat(g.pfmt)
	query, args, err := stmt.ToSql()
	if err != nil {
		return 0, serror.NewFromError(err)
	}
	rows, err := g.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, serror.NewFromError(err)
	}
	if rows == nil {
		return 0, serror.NewFromError(errors.New("GeneralRepo: ExecContext returned nil result"))
	}
	n, err := rows.RowsAffected()
	if err != nil {
		return 0, serror.NewFromError(err)
	}
	return n, nil
}

func (g generalRepo) InsertScanRow(ctx context.Context, result interface{}, stmt sq.InsertBuilder) serror.SError {
	if g.table != nil {
		if v, ok := builder.Get(stmt, "Into"); !ok || v == "" {
			stmt = stmt.Into(*g.table)
		}
	}
	query, args, err := stmt.PlaceholderFormat(g.pfmt).Suffix("returning *").ToSql()
	if err != nil {
		return serror.NewFromError(err)
	}
	row, err := g.db.QueryContext(ctx, query, args...)
	if err != nil {
		return serror.NewFromError(err)
	}
	//goland:noinspection GoUnhandledErrorResult
	defer row.Close()
	if err := scan.Row(result, row); err != nil {
		return serror.NewFromError(err)
	}
	return nil
}

func (g generalRepo) UpdateScanRow(ctx context.Context, result interface{}, stmt sq.UpdateBuilder) serror.SError {
	if g.table != nil {
		if v, ok := builder.Get(stmt, "Table"); !ok || v == "" {
			stmt = stmt.Table(*g.table)
		}
	}
	query, args, err := stmt.PlaceholderFormat(g.pfmt).Suffix("returning *").ToSql()
	if err != nil {
		return serror.NewFromError(err)
	}
	row, err := g.db.QueryContext(ctx, query, args...)
	if err != nil {
		return serror.NewFromError(err)
	}
	//goland:noinspection GoUnhandledErrorResult
	defer row.Close()
	if err := scan.Row(result, row); err != nil {
		return serror.NewFromError(err)
	}
	return nil
}

// VerifyIdsExist checks that every id in ids is present in this repo's bound
// table's id column (optionally narrowed by extra conditions, e.g.
// sq.Eq{"status": 1} to require the row be active). Returns nil if ids is
// empty, or a descriptive error naming whichever ids were not found.
func (g generalRepo) VerifyIdsExist(ctx context.Context, ids []string, where ...sq.Sqlizer) serror.SError {
	if len(ids) == 0 {
		return nil
	}
	if g.table == nil {
		return serror.New("VerifyIdsExist: repo has no bound table")
	}

	stmt := sq.Select("id").From(*g.table).Where("id = ANY(?::uuid[])", pq.Array(ids))
	for _, cond := range where {
		stmt = stmt.Where(cond)
	}

	var found []string
	if serr := g.SelectScanRows(ctx, &found, stmt); serr != nil {
		return serr
	}

	foundSet := make(map[string]struct{}, len(found))
	for _, id := range found {
		foundSet[id] = struct{}{}
	}
	var missing []string
	for _, id := range ids {
		if _, ok := foundSet[id]; !ok {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		return serror.Newf("unknown id(s) in %s: %v", *g.table, missing)
	}
	return nil
}

func (g generalRepo) InsertRows(ctx context.Context, stmt sq.InsertBuilder) (affected int64, serr serror.SError) {
	if g.table != nil {
		if v, ok := builder.Get(stmt, "Into"); !ok || v == "" {
			stmt = stmt.Into(*g.table)
		}
	}
	query, args, err := stmt.PlaceholderFormat(g.pfmt).ToSql()
	if err != nil {
		return affected, serror.NewFromError(err)
	}
	row, err := g.db.ExecContext(ctx, query, args...)
	if err != nil {
		return affected, serror.NewFromError(err)
	}
	if row == nil {
		return affected, serror.NewFromError(errors.New("GeneralRepo: ExecContext returned nil result"))
	}
	affected, err = row.RowsAffected()
	if err != nil {
		return affected, serror.NewFromError(err)
	}
	return affected, nil
}
