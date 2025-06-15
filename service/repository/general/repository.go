package general

import (
	"context"
	"database/sql"
	"errors"
	"reflect"

	sq "github.com/Masterminds/squirrel"
	"github.com/0xelden/common-libs-go/api"
	"github.com/0xelden/common-libs-go/db"
	"github.com/0xelden/common-libs-go/helper"
	"github.com/0xelden/common-libs-go/service"
	"github.com/0xelden/common-libs-go/shared"
	"github.com/0xelden/common-libs-go/serror"
)

type RepositoryConfig[dto, model any] struct {
	Driver            db.Driver
	Dto               service.DTO[dto]
	Repo              service.GeneralRepo
	TableName         string
	CountQuery        string
	IndexQuery        string
	DtoIdField        string
	IndexFilterUseAnd bool
	Overrider         any
}

type Repository[dto, model any] struct {
	config *RepositoryConfig[dto, model]
}

type RepositoryBuilder[dto, model any] struct {
	config *RepositoryConfig[dto, model]
}

func NewRepositoryBuilder[dto, model any](driver db.Driver, tableName string) *RepositoryBuilder[dto, model] {
	if len(tableName) == 0 {
		panic("table name must not be empty on NewRepositoryBuilder")
	}
	return &RepositoryBuilder[dto, model]{config: &RepositoryConfig[dto, model]{
		Driver:     driver,
		TableName:  tableName,
		DtoIdField: "id",
	}}
}

func (r *Repository[dto, model]) DTO() service.DTO[dto] {
	return r.config.Dto
}

func (r *Repository[dto, model]) GeneralRepo() service.GeneralRepo {
	return r.config.Repo
}

func (r *Repository[dto, model]) ToDto(m model) dto {
	var zero dto
	if v, ok := any(m).(dto); ok {
		return v
	}

	var dtoPtr *dto
	dtoType := reflect.TypeOf(dtoPtr).Elem()
	val := reflect.ValueOf(m)
	if !val.IsValid() {
		return zero
	}

	if val.Type().AssignableTo(dtoType) {
		return val.Interface().(dto)
	}
	if val.Type().ConvertibleTo(dtoType) {
		return val.Convert(dtoType).Interface().(dto)
	}

	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return zero
		}
		val = val.Elem()
		if !val.IsValid() {
			return zero
		}
		if val.Type().AssignableTo(dtoType) {
			return val.Interface().(dto)
		}
		if val.Type().ConvertibleTo(dtoType) {
			return val.Convert(dtoType).Interface().(dto)
		}
	}

	if val.Kind() != reflect.Struct {
		return zero
	}

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := field.Type()
		if fieldType.AssignableTo(dtoType) && field.CanInterface() {
			return field.Interface().(dto)
		}
		if fieldType.ConvertibleTo(dtoType) && field.CanInterface() {
			return field.Convert(dtoType).Interface().(dto)
		}
		if fieldType.Kind() != reflect.Ptr || field.IsNil() {
			continue
		}
		elem := field.Elem()
		if elem.Type().AssignableTo(dtoType) && elem.CanInterface() {
			return elem.Interface().(dto)
		}
		if elem.Type().ConvertibleTo(dtoType) && elem.CanInterface() {
			return elem.Convert(dtoType).Interface().(dto)
		}
	}

	return zero
}

func (r *Repository[dto, model]) ToDtoList(src []model, fn ...func(model) dto) []dto {
	res := make([]dto, len(src))

	converter := r.ToDto
	if len(fn) > 0 && fn[0] != nil {
		converter = fn[0]
	}

	for i, v := range src {
		res[i] = converter(v)
	}
	return res
}

func (r *Repository[dto, model]) Insert(ctx context.Context, form dto) (result dto, serr serror.SError) {
	if iface, ok := r.config.Overrider.(service.Inserter[dto]); ok {
		return iface.Insert(ctx, form)
	}
	helper.TraceUnique(ctx, "Repository.Create", form)
	maps, serr := r.config.Dto.PrepStructInsert(ctx, &form)
	if serr != nil {
		return result, serr
	}
	serr = r.config.Repo.InsertScanRow(ctx, &result, sq.Insert(r.config.TableName).SetMap(maps))
	return result, serr
}

func (r *Repository[dto, model]) Update(ctx context.Context, form dto, where ...sq.Sqlizer) (affected int64, serr serror.SError) {
	if iface, ok := r.config.Overrider.(service.Updater[dto]); ok {
		return iface.Update(ctx, form, where...)
	}

	helper.TraceUnique(ctx, "Repository.Edit", form)
	ctx = helper.ContextSetString(ctx, "with_id", "1")
	maps, serr := r.config.Dto.PrepStructEdit(ctx, &form, nil)
	if serr != nil {
		return affected, serr
	}

	var cond sq.Sqlizer
	key := helper.If(len(r.config.DtoIdField) == 0, "id", r.config.DtoIdField)
	id, ok := maps[key]
	if !ok && len(where) == 0 {
		return affected, serror.New("id column not provided for Update")
	}
	if ok {
		cond = sq.Eq{key: id}
	}

	if len(where) > 0 {
		combined := sq.And{}
		if !helper.IsNil(cond) {
			combined = append(combined, cond)
		}
		for _, v := range where {
			if !helper.IsNil(v) {
				combined = append(combined, v)
			}
		}
		cond = combined
	}

	if helper.IsNil(cond) {
		return affected, serror.New("Update got invalid args")
	}

	return r.config.Repo.UpdateRows(ctx, sq.Update(r.config.TableName).Where(cond).SetMap(maps))
}

func (r *Repository[dto, model]) Remove(ctx context.Context, id string, where ...sq.Sqlizer) (affected int64, serr serror.SError) {
	if iface, ok := r.config.Overrider.(service.Remover); ok {
		return iface.Remove(ctx, id, where...)
	}
	if len(id) == 0 && len(where) == 0 {
		return affected, serror.New("Remove requires either a non-empty id or at least one condition in where")
	}
	var cond sq.Sqlizer = sq.Eq{r.config.DtoIdField: id}
	if len(where) > 0 {
		combined := sq.And{}
		if len(id) > 0 {
			combined = append(combined, cond)
		}
		for _, v := range where {
			if !helper.IsNil(v) {
				combined = append(combined, v)
			}
		}
		cond = combined
	}
	return r.config.Repo.DeleteRows(ctx, sq.Delete(r.config.TableName).Where(cond))
}

func (r *Repository[dto, model]) Get(ctx context.Context, param api.ViewParam) (result model, serr serror.SError) {
	if iface, ok := r.config.Overrider.(service.Getter[model]); ok {
		return iface.Get(ctx, param)
	}
	ctx = helper.ContextSetString(ctx, shared.OmitTotal, "1")
	// Id and Filters are combined (AND), so a scoped lookup like
	// {Id: x, Filters: ["result.company_id = 'y'"]} matches row x only when it
	// also passes the filters. Previously a non-empty Filters silently
	// REPLACED the id predicate, making Get return the first row matching the
	// filters regardless of Id — which broke every "fetch by id within tenant
	// scope" caller (and the ownership checks built on them).
	if param.Id == "" && len(param.Filters) == 0 {
		// an unconstrained Get would silently return an arbitrary row
		return result, serror.New("view param requires an id or filters")
	}
	var filters []string
	if param.Id != "" {
		filters = append(filters, helper.FormatSQL("result.id = ?", param.Id))
	}
	filters = append(filters, param.Filters...)
	idx := api.IndexParam{
		Page:    1,
		Size:    1,
		Filters: filters,
	}
	res, serr := r.List(ctx, idx)
	if len(res.Rows) == 0 {
		ident := "id:" + param.Id
		if len(param.Filters) > 0 {
			ident += " filter:" + helper.ToString(param.Filters)
		}
		return result, serror.Newf("row not found, %v", ident)
	}
	return res.Rows[0], nil
}

// VerifyIdsExist delegates to the underlying GeneralRepo — see its doc comment.
func (r *Repository[dto, model]) VerifyIdsExist(ctx context.Context, ids []string, where ...sq.Sqlizer) serror.SError {
	return r.config.Repo.VerifyIdsExist(ctx, ids, where...)
}

func (r *Repository[dto, model]) List(ctx context.Context, param api.IndexParam) (result api.RowIndex[model], serr serror.SError) {
	if iface, ok := r.config.Overrider.(service.Lister[model]); ok {
		return iface.List(ctx, param)
	}
	if len(r.config.IndexQuery) == 0 && len(r.config.CountQuery) == 0 {
		return result, serror.New("Index method not implemented")
	}
	result.Rows = []model{}
	helper.TraceUnique(ctx, "Repository.Index", param)
	serr = result.List(ctx, r.config.Driver, param, r.config.CountQuery, r.config.IndexQuery, r.config.IndexFilterUseAnd)
	if serr != nil && errors.Is(serr.Cause(), sql.ErrNoRows) {
		return result, nil
	}
	return result, serr
}

func (b *RepositoryBuilder[dto, model]) WithIndexQuery(countQuery, indexQuery string, indexFilterUseAnd ...bool) *RepositoryBuilder[dto, model] {
	b.config.CountQuery = countQuery
	b.config.IndexQuery = indexQuery
	if len(indexFilterUseAnd) > 0 {
		b.config.IndexFilterUseAnd = indexFilterUseAnd[0]
	}
	return b
}

func (b *RepositoryBuilder[dto, model]) WithOverride(overrider any) *RepositoryBuilder[dto, model] {
	b.config.Overrider = overrider
	return b
}

func (b *RepositoryBuilder[dto, model]) WithDtoInterface(Dto service.DTO[dto]) *RepositoryBuilder[dto, model] {
	b.config.Dto = Dto
	return b
}

func (b *RepositoryBuilder[dto, model]) WithGeneralRepoInterface(repo service.GeneralRepo) *RepositoryBuilder[dto, model] {
	b.config.Repo = repo
	return b
}

func (b *RepositoryBuilder[dto, model]) New() service.Repository[dto, model] {
	if b.config.Repo == nil {
		b.config.Repo = NewGeneralRepo(b.config.Driver, b.config.TableName)
	}
	if b.config.Dto == nil {
		b.config.Dto = NewDTO[dto](b.config.TableName, b.config.Repo)
	}
	if _, ok := b.config.Overrider.(service.Lister[model]); (!ok || b.config.Overrider == nil) && len(b.config.IndexQuery) == 0 && len(b.config.CountQuery) == 0 {
		panic("neither service.Lister or IndexQuery & CountQuery implemented")
	}
	return &Repository[dto, model]{config: b.config}
}
