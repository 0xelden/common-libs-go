package api

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/ganigeorgiev/fexpr"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-errors/errors"
	"github.com/0xelden/common-libs-go/helper"
	"github.com/0xelden/common-libs-go/shared"
)

const (
	maxPageSize       = 5_000
	maxPageSizeExport = 100_000
)

//goland:noinspection GoVetStructTag
type IndexParam struct {
	Page     int      `query:"page" form:"page" json:"page"`
	Size     int      `query:"size" form:"size" json:"size"`
	Filter   string   `query:"filter" form:"filter" json:"filter"`
	Sort     []string `query:"sort" form:"sort" json:"sort"`
	Column   []string `query:"column" form:"column" json:"column"`
	SortStmt string   `query:"-" form:"-" json:"-"`
	Filters  []string `query:"-" form:"-" json:"-"`
	FileName string   `query:"-" form:"-" json:"-"`

	// used on tabular output filename
	FileType    string   `query:"file_type" form:"file_type" json:"-"`
	ColumnLabel []string `query:"-" form:"-" json:"-"`

	FilterMap map[string]string `query:"-" form:"-" json:"-"`
}

func NewIndexParam(ctx *gin.Context, binding binding.Binding) (*IndexParam, error) {
	param := &IndexParam{
		Page:      1,
		Size:      0,
		Sort:      []string{},
		Filters:   []string{},
		FileType:  "xlsx",
		FilterMap: map[string]string{},
	}
	_ = ctx.MustBindWith(param, binding) // ignore error here
	// omit_total dibaca langsung dari query (fallback ke context): handler umumnya baru memanggil
	// SetContext SETELAH NewIndexParam (lihat template cmd/gen-repo), sehingga nilai di context
	// belum terisi di titik ini dan pembesaran size ke maxPageSize tidak pernah aktif.
	omitTotal := ctx.Query(shared.OmitTotal) == "1" ||
		helper.ContextGetString(ctx.Request.Context(), shared.OmitTotal) == "1"
	if omitTotal {
		param.Page = 1
		param.Size = helper.If(param.Size > 0, param.Size, maxPageSize)
	}
	// set default size
	if param.Size < 1 {
		param.Size = 10
	}
	if err := shared.Validate.Struct(param); err != nil {
		return nil, err
	}
	// handle larger data for export
	allData := helper.ContextGetString(ctx.Request.Context(), shared.AllData) == "1"
	if allData {
		param.Page = 1
		param.Size = maxPageSizeExport
	}
	if len(param.Filter) > 0 {
		filter, err := param.parseFilter(param.Filter)
		if err != nil {
			return nil, err
		}
		param.Filters = append(param.Filters, filter)
	}
	if len(param.Sort) > 0 {
		sortStmt, err := param.generateSortStmt()
		if err != nil {
			return nil, err
		}
		param.SortStmt = sortStmt
	}
	return param, nil
}

// NewIndexParamWithContext constructor digunakan untuk internal use di dalam service 
func NewIndexParamWithContext(ctx context.Context, param *IndexParam) (*IndexParam, error) {
	param.FilterMap = map[string]string{}
	omitTotal := helper.ContextGetString(ctx, shared.OmitTotal) == "1"
	if omitTotal {
		param.Page = 1
		param.Size = helper.If(param.Size > 0, param.Size, maxPageSize)
	}
	// set default size
	if param.Size < 1 {
		param.Size = 10
	}
	if param.FileType == "" {
		param.FileType = "xlsx"
	}
	if err := shared.Validate.Struct(param); err != nil {
		return nil, err
	}
	// handle larger data for export
	allData := helper.ContextGetString(ctx, shared.AllData) == "1"
	if allData {
		param.Page = 1
		param.Size = maxPageSizeExport
	}

	if len(param.Filter) > 0 {
		filter, err := param.parseFilter(param.Filter)
		if err != nil {
			return nil, err
		}
		param.Filters = append(param.Filters, filter)
	}
	if len(param.Sort) > 0 {
		sortStmt, err := param.generateSortStmt()
		if err != nil {
			return nil, err
		}
		param.SortStmt = sortStmt
	}
	return param, nil
}

func (i *IndexParam) generateSortStmt() (string, error) {
	parts := make([]string, 0, len(i.Sort))
	for _, val := range i.Sort {
		split := strings.Split(val, ":")
		if len(split) != 2 {
			return "", errors.New("invalid sort parameter")
		}
		col := strings.TrimSpace(split[0])
		if len(col) == 0 {
			return "", errors.New("invalid sort column name")
		}
		order := "ASC"
		if strings.ToLower(strings.TrimSpace(split[1])) == "desc" {
			order = "DESC"
		}
		// jsonb path support: sort runs on the outer rows CTE (no table
		// aliases there), so a dotted name means column.key[...] — e.g.
		// "content.name:desc" -> content->>'name' DESC. Segments are
		// whitelisted by jsonbSegRe, so nothing unescaped reaches the SQL.
		jsonbCol, isJsonb, err := buildJsonbPath(col, 1, false)
		if err != nil {
			return "", err
		}
		if isJsonb {
			col = jsonbCol
		} else {
			// escape quote chars on column name
			col = strconv.Quote(col)
		}
		parts = append(parts, fmt.Sprintf(`%s %s`, col, order))
	}
	if len(parts) == 0 {
		return "", nil
	}
	return "ORDER BY " + strings.Join(parts, ", "), nil
}

func (i *IndexParam) GenerateSortStmt() string {
	if i.SortStmt != "" {
		return i.SortStmt
	}
	stmt, _ := i.generateSortStmt()
	return stmt
}

// jsonbSegRe validates every dot-segment of a jsonb path identifier before
// it is single-quoted into SQL. Strict identifier shape (no quotes, spaces,
// or operators) is the injection barrier here: identifiers are the only part
// of a filter that reach the SQL string without going through EscapeSQL, so
// a segment that fails this whitelist is rejected instead of being emitted.
var jsonbSegRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// buildJsonbPath translates a dotted identifier into a jsonb access
// expression, treating the first rootSegs segments as the SQL column
// reference and the rest as JSON keys:
//
//	rootSegs=2 (filter): "result.content.name" -> "result.content->>'name'"
//	rootSegs=1 (sort):   "content.name"        -> "content->>'name'"
//	nested:              "result.content.a.b"  -> "result.content->'a'->>'b'"
//
// Identifiers with <= rootSegs segments are returned unchanged (ok=false),
// preserving the existing alias.column / column::cast behavior. The final
// key uses ->> (text); castNumeric wraps the whole path in (...)::numeric so
// comparisons against number literals aren't lexicographic ("9" > "100").
func buildJsonbPath(ident string, rootSegs int, castNumeric bool) (string, bool, error) {
	parts := strings.Split(ident, ".")
	if len(parts) <= rootSegs {
		return ident, false, nil
	}
	for _, part := range parts {
		if !jsonbSegRe.MatchString(part) {
			return "", false, errors.New("invalid filter identifier: " + ident)
		}
	}
	var b strings.Builder
	b.WriteString(strings.Join(parts[:rootSegs], "."))
	last := len(parts) - 1
	for idx := rootSegs; idx <= last; idx++ {
		b.WriteString(helper.If(idx == last, "->>", "->"))
		b.WriteString("'")
		b.WriteString(parts[idx])
		b.WriteString("'")
	}
	expr := b.String()
	if castNumeric {
		expr = "(" + expr + ")::numeric"
	}
	return expr, true, nil
}

func (i *IndexParam) parseFilterAst(exp any) (res []string, err error) {
	switch exp := exp.(type) {
	case []fexpr.ExprGroup:
		part := []string{}
		for idx, x := range exp {
			if idx > 0 && idx <= len(exp)-1 {
				part = append(part, helper.If(x.Join == fexpr.JoinOr, " OR ", " AND "))
			}
			filter, err := i.parseFilterAst(x)
			if err != nil {
				return nil, err
			}
			part = append(part, strings.Join(filter, ""))
		}
		return append(res, strings.Join(part, "")), nil

	case fexpr.ExprGroup:
		switch item := exp.Item.(type) {
		case []fexpr.ExprGroup:
			filter, err := i.parseFilterAst(item)
			if err != nil {
				return nil, err
			}
			return append(res, "(", strings.Join(filter, ""), ")"), nil

		default:
			filter, err := i.parseFilterAst(item)
			if err != nil {
				return nil, err
			}
			return append(res, strings.Join(filter, "")), nil
		}

	case fexpr.Expr:
		left := helper.If(exp.Left.Type == fexpr.TokenText, helper.EscapeSQL(exp.Left.Literal), exp.Left.Literal)
		if exp.Left.Type == fexpr.TokenIdentifier {
			// jsonb path support: alias.column.key[...] — cast to numeric only
			// when compared against a number literal (null stays text/IS NULL)
			castNumeric := exp.Right.Type == fexpr.TokenNumber
			jsonbLeft, isJsonb, err := buildJsonbPath(left, 2, castNumeric)
			if err != nil {
				return nil, err
			}
			if isJsonb {
				left = jsonbLeft
			}
		}
		if strings.ToLower(exp.Right.Literal) == "null" {
			return append(res, fmt.Sprintf("%s %s", left, helper.If(exp.Op == fexpr.SignEq, "IS NULL", "IS NOT NULL"))), nil
		}

		op := string(exp.Op)
		if op == "~" {
			op = " ILIKE " // case-insensitive like
		}

		right := helper.If(exp.Right.Type == fexpr.TokenText, helper.EscapeSQL(exp.Right.Literal), exp.Right.Literal)

		// detect malicious filter like 1=1 or 99=99
		if strings.TrimSpace(strings.ToLower(left)) == strings.TrimSpace(strings.ToLower(right)) {
			return nil, errors.New("invalid filter")
		}

		// find escaped quotes and convert it to sql format
		left = helper.EscapeSingleQuoteSql(left)
		right = helper.EscapeSingleQuoteSql(right)
		join := fmt.Sprintf("%s%s%s", left, op, right)

		i.FilterMap[left] = op + right
		i.FilterMap[right] = op + left
		return append(res, join), nil
	}

	return res, nil
}

func (i *IndexParam) parseFilter(str string) (res string, err error) {
	exp, err := fexpr.Parse(str)
	if err != nil {
		return "", err
	}
	filter, err := i.parseFilterAst(exp)
	if err != nil {
		return "", err
	}
	return strings.Join(filter, " "), nil
}

func (i *IndexParam) GenerateFilterStmt(useAndClause ...bool) string {
	if len(i.Filters) == 0 {
		return ""
	}

	prefix := "WHERE "
	if len(useAndClause) > 0 && useAndClause[0] {
		prefix = "AND "
	}

	filters := make([]string, len(i.Filters))
	for i, v := range i.Filters {
		filters[i] = "(" + v + ")"
	}

	return prefix + strings.Join(filters, " AND ")
}

func (i *IndexParam) Select(_ context.Context) []string {
	return i.Column
}
