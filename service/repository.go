package service

import (
	"context"

	sq "github.com/Masterminds/squirrel"
	"github.com/0xelden/common-libs-go/api"
	"github.com/0xelden/common-libs-go/db"
	"github.com/0xelden/common-libs-go/serror"
)

type (
	Inserter[T any] interface {
		Insert(ctx context.Context, form T) (result T, serr serror.SError)
	}
	Updater[T any] interface {
		Update(ctx context.Context, form T, where ...sq.Sqlizer) (affected int64, serr serror.SError)
	}
	Remover interface {
		Remove(ctx context.Context, id string, where ...sq.Sqlizer) (affected int64, serr serror.SError)
	}
	Getter[T any] interface {
		Get(ctx context.Context, param api.ViewParam) (result T, serr serror.SError)
	}
	Lister[T any] interface {
		List(ctx context.Context, param api.IndexParam) (result api.RowIndex[T], serr serror.SError)
	}
)

type GeneralRepo interface {
	SelectScanRow(ctx context.Context, result interface{}, stmt sq.SelectBuilder) serror.SError
	SelectScanRows(ctx context.Context, result interface{}, stmt sq.SelectBuilder) serror.SError
	InsertScanRow(ctx context.Context, result interface{}, stmt sq.InsertBuilder) serror.SError
	UpdateScanRow(ctx context.Context, result interface{}, stmt sq.UpdateBuilder) serror.SError
	InsertRows(ctx context.Context, stmt sq.InsertBuilder) (affected int64, serr serror.SError)
	DeleteRows(ctx context.Context, stmt sq.DeleteBuilder) (affected int64, serr serror.SError)
	UpdateRows(ctx context.Context, stmt sq.UpdateBuilder) (affected int64, serr serror.SError)
	CountRows(ctx context.Context, stmt sq.SelectBuilder) (total int64, serr serror.SError)
	RawSelectOne(ctx context.Context, result any, query string, args ...any) serror.SError
	RawSelectRows(ctx context.Context, result any, query string, args ...any) serror.SError
	Driver() db.Driver
	TableName() string
}

type DTO[T any] interface {
	SelectRow(ctx context.Context, stmt sq.SelectBuilder) (result T, serr serror.SError)
	SelectRows(ctx context.Context, stmt sq.SelectBuilder) (result []T, serr serror.SError)
	InsertRows(ctx context.Context, form []T, itemCallback func(index int, item map[string]any)) (result []T, serr serror.SError)
	PatchRows(ctx context.Context, form []T, itemCallback func(index int, item map[string]any)) (result []T, serr serror.SError)
	PatchRow(ctx context.Context, form T, itemCallback func(item map[string]any)) (result T, serr serror.SError)
	PrepStructInsert(ctx context.Context, Struct any) (maps map[string]any, serr serror.SError)
	PrepStructEdit(ctx context.Context, Struct any, excludes []string) (maps map[string]any, serr serror.SError)
}

type Repository[dto, model any] interface {
	DTO() DTO[dto]
	GeneralRepo() GeneralRepo
	ToDto(m model) dto
	ToDtoList(src []model, fn ...func(model) dto) []dto
	Insert(ctx context.Context, form dto) (result dto, serr serror.SError)
	Update(ctx context.Context, form dto, where ...sq.Sqlizer) (affected int64, serr serror.SError)
	Remove(ctx context.Context, id string, where ...sq.Sqlizer) (affected int64, serr serror.SError)
	Get(ctx context.Context, param api.ViewParam) (result model, serr serror.SError)
	List(ctx context.Context, param api.IndexParam) (result api.RowIndex[model], serr serror.SError)
}
