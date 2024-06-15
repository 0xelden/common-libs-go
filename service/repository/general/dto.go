package general

import (
	"context"
	"reflect"
	"sort"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/blockloop/scan/v2"
	"github.com/google/uuid"
	"github.com/lann/builder"
	"github.com/lib/pq"
	"github.com/0xelden/common-libs-go/helper"
	"github.com/0xelden/common-libs-go/service"
	"github.com/0xelden/common-libs-go/shared"
	"github.com/0xelden/common-libs-go/serror"
)

type DTO[T any] struct {
	table string
	repo  service.GeneralRepo
	tstr  string
}

func NewDTO[T any](tableName string, generalRepo service.GeneralRepo) service.DTO[T] {
	var t T
	return &DTO[T]{
		table: tableName,
		repo:  generalRepo,
		tstr:  reflect.TypeOf(t).String(),
	}
}

// SelectRow return error on not found
func (r *DTO[T]) SelectRow(ctx context.Context, stmt sq.SelectBuilder) (result T, serr serror.SError) {
	if _, ok := builder.Get(stmt, "From"); !ok {
		stmt = stmt.From(r.table)
	}
	serr = r.repo.SelectScanRow(ctx, &result, stmt)
	return result, serr
}

func (r *DTO[T]) SelectRows(ctx context.Context, stmt sq.SelectBuilder) (result []T, serr serror.SError) {
	if _, ok := builder.Get(stmt, "From"); !ok {
		stmt = stmt.From(r.table)
	}
	serr = r.repo.SelectScanRows(ctx, &result, stmt)
	return result, serr
}

func (r *DTO[T]) ColValues(ctx context.Context, t any) (columns []string, values []any, serr serror.SError) {
	var (
		err  error
		now  = time.Now()
		by   = helper.ContextGetAny(ctx, shared.X_UserId, nil)
		excl = []string{"status", "created_by", "created_at", "updated_by", "updated_at"}
	)
	columns, err = scan.Columns(t, excl...)
	if err != nil {
		return columns, values, serror.NewFromError(err)
	}
	vals, err := scan.Values(columns, t)
	if err != nil {
		return columns, values, serror.NewFromError(err)
	}
	columns = append(columns, excl...)
	values = append(vals, 1, by, now, by, now)
	return columns, values, nil
}

func (r *DTO[T]) PrepStructInsert(ctx context.Context, Struct any) (maps map[string]any, serr serror.SError) {
	// Type assertion to check if Struct is a pointer
	if reflect.TypeOf(Struct).Kind() != reflect.Ptr {
		return maps, serror.New("Struct must be a pointer")
	}
	var (
		now = time.Now()
		by  = helper.ContextGetAny(ctx, shared.X_UserId, nil)
	)
	maps = helper.StructToMapDB(Struct)
	id := helper.ToString(maps["id"])
	maps["id"] = helper.If(helper.IsValidUUID(id), id, uuid.NewString())
	maps["status"] = helper.If(!helper.IsNil(maps["status"]), maps["status"], 1)
	maps["created_by"] = by
	maps["created_at"] = now
	maps["updated_by"] = by
	maps["updated_at"] = now
	return maps, nil
}

func (r *DTO[T]) PrepStructEdit(ctx context.Context, Struct any, excludeColumns []string) (maps map[string]any, serr serror.SError) {
	// Type assertion to check if Struct is a pointer
	if reflect.TypeOf(Struct).Kind() != reflect.Ptr {
		return maps, serror.New("Struct must be a pointer")
	}
	var (
		now     = time.Now()
		by      = helper.ContextGetAny(ctx, shared.X_UserId, nil)
		withId  = helper.ContextGetString(ctx, "with_id") == "1"
		defExcl = helper.If(withId, []string{"created_by", "created_at"}, []string{"id", "created_by", "created_at"})
	)
	maps = helper.StructToMapDB(Struct)
	maps["updated_by"] = by
	maps["updated_at"] = now
	if len(excludeColumns) == 0 {
		excludeColumns = make([]string, 0, len(defExcl))
	}
	excludeColumns = append(excludeColumns, defExcl...)
	for _, v := range excludeColumns {
		delete(maps, v)
	}
	if helper.IsNil(maps["status"]) {
		delete(maps, "status")
	}
	return maps, nil
}

func (r *DTO[T]) InsertRows(ctx context.Context, form []T, itemCallback func(index int, item map[string]any)) (result []T, serr serror.SError) {
	const chunkSize = 1000

	if len(form) == 0 {
		return result, serror.New("empty rows insert is not allowed")
	}

	if len(form) <= chunkSize {
		return r.insertRows(ctx, form, itemCallback)
	}

	for start := 0; start < len(form); start += chunkSize {
		end := start + chunkSize
		if end > len(form) {
			end = len(form)
		}

		part := form[start:end]

		var cb func(index int, item map[string]any)
		if itemCallback != nil {
			offset := start
			cb = func(index int, item map[string]any) {
				itemCallback(offset+index, item)
			}
		}

		if _, err := r.insertRows(ctx, part, cb); err != nil {
			return result, err
		}
	}

	return form, nil
}

func (r *DTO[T]) insertRows(ctx context.Context, form []T, itemCallback func(index int, item map[string]any)) (result []T, serr serror.SError) {
	if len(form) == 0 {
		return result, serror.New("empty rows insert is not allowed")
	}

	var (
		now   = time.Now()
		by    = helper.ContextGetAny(ctx, shared.X_UserId)
		stmt  = sq.Insert(r.table)
		rows  = make([]map[string]any, 0, len(form))
		hasCb = itemCallback != nil
	)

	for i, item := range form {
		row := helper.StructToMapDB(item)

		id := helper.ToString(row["id"])
		if !helper.IsValidUUID(id) {
			row["id"] = uuid.NewString()
		} else {
			row["id"] = id
		}

		if helper.IsNil(row["status"]) {
			row["status"] = 1
		}

		row["created_by"] = by
		row["created_at"] = now
		row["updated_by"] = by
		row["updated_at"] = now

		if hasCb {
			itemCallback(i, row)
		}

		rows = append(rows, row)
	}

	helper.TraceUnique(ctx, "*DTO["+r.tstr+"].InsertRows", rows)

	// ambil kolom dari row pertama, lalu sort supaya urutannya stabil
	cols := make([]string, 0, len(rows[0]))
	for k := range rows[0] {
		cols = append(cols, k)
	}
	sort.Strings(cols)

	stmt = stmt.Columns(cols...)

	for _, row := range rows {
		val := make([]any, 0, len(cols))
		for _, col := range cols {
			val = append(val, row[col])
		}
		stmt = stmt.Values(val...)
	}

	if v := helper.ContextGetString(ctx, shared.SqlSuffix); v != "" {
		stmt = stmt.Suffix(v)
	}

	_, serr = r.repo.InsertRows(ctx, stmt)
	if serr != nil {
		return result, serr
	}

	return form, nil
}

func (r *DTO[T]) PatchRows(ctx context.Context, form []T, itemCallback func(index int, item map[string]any)) (result []T, serr serror.SError) {
	chunkSize := 64
	if len(form) < chunkSize {
		return r.patchRows(ctx, form, itemCallback)
	}
	chunk := helper.SliceChunk(form, chunkSize)
	for _, v := range chunk {
		if _, err := r.patchRows(ctx, v, itemCallback); err != nil {
			return result, err
		}
	}
	return form, nil
}

func (r *DTO[T]) patchRows(ctx context.Context, form []T, itemCallback func(index int, item map[string]any)) (result []T, serr serror.SError) {
	if len(form) == 0 {
		return result, serror.New("empty rows patch is not allowed")
	}

	var (
		now   = time.Now()
		by    = helper.ContextGetAny(ctx, shared.X_UserId)
		stmt  = sq.Update(r.table)
		rows  = []map[string]any{}
		hasCb = itemCallback != nil
		ids   = make([]string, 0, len(form))
	)

	for i, item := range form {
		row := helper.StructToMapDB(item)
		row["updated_by"] = by
		row["updated_at"] = now
		if hasCb {
			itemCallback(i, row)
		}
		for k, v := range row {
			if helper.IsNil(v) {
				delete(row, k)
				continue
			}
		}
		id := helper.ToString(row["id"])
		if !helper.IsValidUUID(id) {
			return nil, serror.Newf("patch error, index no %d did not have an id", i)
		}

		row["id"] = id
		ids = append(ids, id)
		rows = append(rows, row)
	}

	helper.TraceUnique(ctx, "*DTO["+r.tstr+"].PatchRows", rows)

	caseExprs := make(map[string]sq.CaseBuilder)
	for _, row := range rows {
		for col, val := range row {
			if col == "id" {
				continue
			}
			caseExpr := caseExprs[col]
			caseExpr = caseExpr.When(sq.Expr("id = ?", row["id"]), sq.Expr("?", val))
			caseExprs[col] = caseExpr
		}
	}

	if len(caseExprs) == 0 {
		return result, serror.New("patch error, empty fields to update")
	}

	stmt = stmt.Where(sq.Eq{"id": ids})
	for col, caseExpr := range caseExprs {
		stmt = stmt.Set(col, caseExpr.Else(sq.Expr(pq.QuoteIdentifier(col))))
	}

	if v := helper.ContextGetString(ctx, shared.SqlSuffix); v != "" {
		stmt = stmt.Suffix(v)
	}

	_, serr = r.repo.UpdateRows(ctx, stmt)
	if serr != nil {
		return result, serr
	}
	return form, nil
}

func (r *DTO[T]) PatchRow(ctx context.Context, form T, itemCallback func(item map[string]any)) (result T, serr serror.SError) {
	var (
		now   = time.Now()
		by    = helper.ContextGetAny(ctx, shared.X_UserId)
		stmt  = sq.Update(r.table)
		hasCb = itemCallback != nil
	)

	row := helper.StructToMapDB(form)
	row["updated_by"] = by
	row["updated_at"] = now
	if hasCb {
		itemCallback(row)
	}

	id := helper.ToString(row["id"])
	if !helper.IsValidUUID(id) {
		return result, serror.Newf("patch error, dto did not have an id")
	}

	row["id"] = id
	helper.TraceUnique(ctx, "*DTO["+r.tstr+"].PatchRow", row)

	for k, v := range row {
		if k == "id" {
			continue
		}
		if helper.IsNil(v) {
			delete(row, k)
			continue
		}
		stmt = stmt.Set(k, v)
	}
	if v := helper.ContextGetString(ctx, shared.SqlSuffix); v != "" {
		stmt = stmt.Suffix(v)
	}

	serr = r.repo.UpdateScanRow(ctx, &form, stmt.Where(sq.Eq{"id": id}))
	if serr != nil {
		return result, serr
	}

	return form, nil
}
