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

	// Filters holds raw SQL fragments and is deliberately NOT bindable from the
	// request (query:"-" form:"-"), mirroring IndexParam.Filters.
	//
	// It used to carry `query:"filters" form:"filters"`, which let a client send
	// ?filters=... straight into the WHERE clause. Worse than adding a
	// predicate: gin's form mapping assigns the field whenever the key is
	// present, so a client-supplied ?filters= REPLACED the scoping fragments a
	// caller passed to NewViewParam — turning "fetch row X within this tenant"
	// into "fetch any row matching whatever the client asked for".
	//
	// Build entries with AddFilter, not fmt.Sprintf.
	Filters []string `query:"-" form:"-" json:"-" validate:"required_without=Id,dive,gt=0"`
}

// AddFilter appends a WHERE fragment built from a developer-written template
// and user-supplied values, e.g.
//
//	param.AddFilter("result.company_id = ?", companyID)
//
// The template is trusted, every arg is escaped by helper.FormatSQL, and the
// result is rejected if it would break out of the clause. Prefer this over
// appending to Filters directly — that field is raw SQL, so a value pasted in
// with fmt.Sprintf is an injection point.
func (v *ViewParam) AddFilter(stmt string, args ...any) error {
	fragment, err := buildFilterFragment(stmt, args...)
	if err != nil {
		return err
	}
	v.Filters = append(v.Filters, fragment)
	return nil
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
