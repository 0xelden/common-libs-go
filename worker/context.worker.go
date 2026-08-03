package worker

import (
	"context"

	"github.com/0xelden/common-libs-go/db"
	"github.com/0xelden/common-libs-go/helper"
	"github.com/0xelden/common-libs-go/serror"
	"github.com/0xelden/common-libs-go/shared"
	"github.com/hibiken/asynq"
)

func setContext(ctx context.Context, kv ...string) context.Context {
	def := []string{}
	pair := append(def[:], kv...)
	for i := 0; i < len(pair); i += 2 {
		ctx = context.WithValue(ctx, pair[i], pair[i+1])
	}
	return ctx
}

func SetCtx(ctx context.Context, kv ...string) context.Context {
	return setContext(ctx, kv...)
}

func SetCtxAndTrx(ctx context.Context, kv ...string) (context.Context, db.TrxDriver) {
	ctx = setContext(ctx, kv...)
	if tx := ctx.Value(shared.TrxInstance); tx != nil {
		switch v := tx.(type) {
		case db.TrxDriver:
			return ctx, v
		}
	}
	return ctx, nil
}

func Success(ctx context.Context, data any) {
	if data == nil {
		return
	}

	if task := ctx.Value(shared.TaskInstance); task != nil {
		switch v := task.(type) {
		case *asynq.Task:
			if _, err := v.ResultWriter().Write([]byte(helper.ToJsonIndent(data))); err != nil {
				helper.RecordErrorTrace(ctx, serror.Newf("task write result error, got:%v", err))
				return
			}
		}
	}
}
