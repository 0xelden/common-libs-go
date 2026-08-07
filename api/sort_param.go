package api

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-errors/errors"
	"github.com/0xelden/common-libs-go/helper"
)

//go:generate easytags $GOFILE query,form,json
type SortParam struct {
	Sort     []string `query:"sort" form:"sort" json:"sort"`
	SortStmt string   `query:"-" form:"-" json:"-"`
}

func NewSortParam(ctx *gin.Context, queryKey string) (SortParam, error) {
	param := SortParam{
		Sort: ctx.QueryArray(queryKey),
	}
	if len(param.Sort) == 0 {
		return param, nil
	}
	sortStmt, err := param.generateSortStmt()
	if err != nil {
		return SortParam{}, err
	}
	param.SortStmt = sortStmt
	return param, nil
}

func NewSortParamBinding(ctx *gin.Context, binding binding.Binding) (SortParam, error) {
	param := SortParam{
		Sort: []string{},
	}
	_ = ctx.MustBindWith(param, binding) // ignore error here
	if len(param.Sort) == 0 {
		return param, nil
	}
	sortStmt, err := param.generateSortStmt()
	if err != nil {
		return SortParam{}, err
	}
	param.SortStmt = sortStmt
	return param, nil
}

func (s *SortParam) generateSortStmt() (string, error) {
	parts := make([]string, 0, len(s.Sort))
	for _, val := range s.Sort {
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
		// quote the column name as a SQL identifier -- strconv.Quote would emit
		// Go escaping (\"), which PostgreSQL does not honour inside a quoted
		// identifier, letting a crafted sort value escape it
		col = helper.QuoteIdent(col)
		parts = append(parts, fmt.Sprintf(`%s %s`, col, order))
	}
	if len(parts) == 0 {
		return "", nil
	}
	return "ORDER BY " + strings.Join(parts, ", "), nil
}

func (s *SortParam) GenerateSortStmt() string {
	if s.SortStmt != "" {
		return s.SortStmt
	}
	stmt, _ := s.generateSortStmt()
	return stmt
}
