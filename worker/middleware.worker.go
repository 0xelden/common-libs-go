package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/0xelden/common-libs-go/helper"
	"github.com/0xelden/common-libs-go/logger"
	"github.com/0xelden/common-libs-go/serror"
	"github.com/0xelden/common-libs-go/shared"
	"github.com/go-errors/errors"
	"github.com/hibiken/asynq"
	"github.com/uptrace/uptrace-go/uptrace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

func (cfg *Config) TracingMiddleware(h asynq.Handler) asynq.Handler {
	var (
		// Create a tracer. Usually, tracer is a global variable.
		tracer = otel.Tracer("job-worker")
	)
	return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
		start := time.Now()
		id := helper.RandStringUnsafe(7)
		logger.Infof("Start processing %q id:%s", t.Type(), id)

		ctx, span := tracer.Start(ctx, t.Type())
		trace := uptrace.TraceURL(span)
		defer logger.Infof("trace url for %s: %s", t.Type(), trace)
		defer func() {
			span.SetAttributes(attribute.Int("job.duration."+id, int(time.Since(start))))
		}()
		defer span.End()

		onErr := func(e error) error {
			serr := serror.NewFromErrors(1, e)
			sf := helper.Reduce(serr.StackFrames(), []map[string]any{}, func(res []map[string]any, val errors.StackFrame, i int) []map[string]any {
				return append(res, map[string]any{
					"file": val.File,
					"line": val.LineNumber,
					"pkg":  val.Package,
				})
			})
			if length := len(sf); length > 4 {
				sf = sf[:4]
			}
			span.RecordError(serr)
			span.SetAttributes(attribute.String(shared.ExceptionSource, fmt.Sprintf("%s:%d %s", serr.File(), serr.Line(), serr.FN())))
			span.SetAttributes(attribute.String(shared.ExceptionStackFrames, helper.ToJsonIndent(sf)))
			logger.Infof("Failed task %q id:%s Elapsed Time = %v", t.Type(), id, time.Since(start))
			return e
		}

		span.SetAttributes(
			attribute.String("job.payload."+id, string(t.Payload())),
			attribute.String("job.type."+id, t.Type()),
		)

		trx, err := cfg.Trxb.NewTransaction(ctx)
		if err != nil {
			return onErr(fmt.Errorf("Trxb.NewTransaction error: %v", err))
		}

		//goland:noinspection GoUnhandledErrorResult
		defer trx.Rollback()

		ctx = context.WithValue(ctx, shared.TaskInstance, t)
		ctx = context.WithValue(ctx, shared.TrxInstance, trx)
		ctx = context.WithValue(ctx, "trace_url", trace)
		if err := h.ProcessTask(ctx, t); err != nil {
			return onErr(err)
		}

		if err := trx.Commit(); err != nil {
			return onErr(err)
		}

		logger.Infof("Finished processing %q id:%s Elapsed Time = %v", t.Type(), id, time.Since(start))
		return nil
	})
}
