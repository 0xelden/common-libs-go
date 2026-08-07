package api

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"strings"

	"github.com/blockloop/scan/v2"
	"github.com/gin-gonic/gin"
	"github.com/tealeg/xlsx"
	"github.com/0xelden/common-libs-go/db"
	"github.com/0xelden/common-libs-go/helper"
	"github.com/0xelden/common-libs-go/shared"
	"github.com/0xelden/common-libs-go/serror"
)

type RowIndex[Row any] struct {
	RowsTotal  int   `json:"rows_total"`
	RowsLength int   `json:"rows_length"`
	PageNumber int   `json:"page_number"`
	PageSize   int   `json:"page_size"`
	Rows       []Row `json:"rows"`
	// qrep lazily normalizes optional SQL placeholders embedded in comments.
	qrep *strings.Replacer
}

func (r *RowIndex[Row]) replaceHandle(inp string) string {
	if r.qrep == nil {
		// Some query templates hide optional format verbs behind SQL comments
		// (for example "---%s") so callers can toggle fragments without changing
		// placeholder counts. Strip the comment markers before fmt.Sprintf runs.
		r.qrep = strings.NewReplacer(`---%s`, `%s`, `----%s`, `%s`)
	}
	return r.qrep.Replace(inp)
}

func (r *RowIndex[Row]) DownloadCsv(c *gin.Context, param *IndexParam, rowGetter func(index int, row Row) (line []any)) {
	var (
		buf    bytes.Buffer
		writer = csv.NewWriter(&buf)
	)

	err := writer.Write(param.ColumnLabel)
	if err != nil {
		Error(c, fmt.Errorf("error creating column header: %v", err), http.StatusInternalServerError)
		return
	}

	for i, v := range r.Rows {
		rows := rowGetter(i, v)
		line := make([]string, len(rows))
		for idx, val := range rows {
			line[idx] = helper.ToString(val)
		}
		err := writer.Write(line)
		if err != nil {
			Error(c, fmt.Errorf("error creating a row: %v", err), http.StatusInternalServerError)
			return
		}
	}

	writer.Flush()

	c.Header("Content-Disposition", "attachment; filename="+param.FileName+".csv")
	c.Header("Content-Type", "text/csv")
	if _, err := buf.WriteTo(c.Writer); err != nil {
		helper.RecordErrorTrace(c, serror.NewFromError(err))
		c.Abort()
		return
	}
}

func (r *RowIndex[Row]) DownloadXlsx(c *gin.Context, param *IndexParam, rowGetter func(index int, row Row) (line []any)) {
	file := xlsx.NewFile()
	sheet, err := file.AddSheet("Sheet1")
	if err != nil {
		Error(c, fmt.Errorf("error creating sheet: %v", err), http.StatusInternalServerError)
		return
	}

	col := sheet.AddRow()
	for _, v := range param.ColumnLabel {
		col.AddCell().Value = v
	}

	for i, v := range r.Rows {
		row := sheet.AddRow()
		for _, val := range rowGetter(i, v) {
			row.AddCell().Value = helper.ToString(val)
		}
	}

	c.Header("Content-Disposition", "attachment; filename="+param.FileName+".xlsx")
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	if err := file.Write(c.Writer); err != nil {
		helper.RecordErrorTrace(c, serror.NewFromError(err))
		c.Abort()
		return
	}
}

func (r *RowIndex[Row]) Transform(ctx context.Context, param IndexParam) (res any, err error) {
	if len(r.Rows) == 0 {
		return r, nil
	}
	if _, ok := any(r.Rows[0]).(Transformer[IndexParam]); !ok {
		return r, nil
	}
	tf := RowIndex[any]{
		RowsTotal:  r.RowsTotal,
		RowsLength: r.RowsLength,
		PageNumber: r.PageNumber,
		PageSize:   r.PageSize,
		Rows:       make([]any, len(r.Rows)),
	}
	for i, item := range r.Rows {
		tf.Rows[i], err = any(item).(Transformer[IndexParam]).Transform(ctx, param)
		if err != nil {
			return nil, err
		}
	}
	return tf, nil
}

func (r *RowIndex[Row]) List(ctx context.Context, db db.Driver, param IndexParam, countQuery, listQuery string, useAndClause ...bool) serror.SError {
	var (
		total     int
		withTotal = helper.ContextGetString(ctx, shared.OmitTotal) != "1"
		filters   = param.GenerateFilterStmt(useAndClause...)
	)

	// Both clauses are interpolated into the query template as raw SQL, so they
	// are checked before any of it reaches the driver. This matters most for the
	// count query below: it runs without bind arguments, which makes lib/pq use
	// the simple protocol, where a `;` would let injected DDL/DML execute.
	// IndexParam.Filters is a plain []string that callers append to directly,
	// so a fragment can arrive here without ever passing through parseFilter.
	if helper.SQLFragmentHasStatementBreak(filters) {
		return serror.New("invalid filter: statement terminator or comment not allowed")
	}

	if withTotal {
		// Normalize commented placeholders before injecting generated filters.
		countQuery = r.replaceHandle(countQuery)
		stmt := fmt.Sprintf(countQuery, filters)
		err := db.QueryRowContext(ctx, stmt).Scan(&total)
		if err != nil {
			return serror.NewFromError(err)
		}
	}

	offset := (param.Page - 1) * param.Size
	if offset <= total {
		var (
			stmt      string
			sort      = param.GenerateSortStmt()
			selective = strings.Count(listQuery, "%s") == 3 && len(param.Column) > 0
		)

		// SortStmt can also be assigned directly by callers, bypassing
		// generateSortStmt's quoting.
		if helper.SQLFragmentHasStatementBreak(sort) {
			return serror.New("invalid sort: statement terminator or comment not allowed")
		}

		// Apply the same normalization for list queries that use optional clauses.
		listQuery = r.replaceHandle(listQuery)

		// handle selective column
		if selective {
			fields := helper.EscapeColumns(param.Column)
			stmt = fmt.Sprintf(listQuery, filters, fields, sort)
		} else {
			stmt = fmt.Sprintf(listQuery, filters, sort)
		}

		rows, err := db.QueryContext(ctx, stmt, param.Size, offset)
		if err != nil {
			return serror.NewFromError(err)
		}

		//goland:noinspection GoUnhandledErrorResult
		defer rows.Close()
		if err := scan.Rows(&r.Rows, rows); err != nil {
			return serror.NewFromError(err)
		}
	}

	r.RowsTotal = total
	r.PageNumber = param.Page
	r.PageSize = param.Size
	r.RowsLength = len(r.Rows)

	return nil
}
