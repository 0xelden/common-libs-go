package api

import (
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/0xelden/common-libs-go/shared"
)

//go:generate easytags $GOFILE query,form,json
type ViewParam struct {
	Id         string   `json:"id" validate:"required_without=Filters,omitempty,uuid"`
	Column     []string `query:"column" form:"column" json:"column"`
	ItemColumn []string `query:"item_column" form:"item_column" json:"item_column"`
	Filters    []string `query:"filters" form:"filters" json:"filters" validate:"required_without=Id,dive,gt=0"`
}

func NewViewParam(ctx *gin.Context, binding binding.Binding, id string, filter ...string) (*ViewParam, error) {
	param := &ViewParam{Filters: filter}
	if err := ctx.MustBindWith(param, binding); err != nil {
		return nil, err
	}
	param.Id = id
	if err := shared.Validate.Struct(param); err != nil {
		return nil, err
	}
	return param, nil
}
