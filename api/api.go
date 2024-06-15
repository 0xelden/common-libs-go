package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-errors/errors"
	_ "github.com/joho/godotenv/autoload"
	"github.com/0xelden/common-libs-go/db"
	"github.com/0xelden/common-libs-go/helper"
	"github.com/0xelden/common-libs-go/shared"
	"github.com/0xelden/common-libs-go/serror"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type response struct {
	Code       int              `json:"code"`
	Error      string           `json:"error"`
	Portal     string           `json:"portal,omitempty"`
	Service    string           `json:"service"`
	Controller string           `json:"controller"`
	Action     string           `json:"action"`
	Result     any              `json:"result"`
	Trace      []map[string]any `json:"trace,omitempty"`
	Ctx        map[string]any   `json:"ctx,omitempty"`
}

//goland:noinspection GoUnusedExportedFunction
func Transform[T Selector](c *gin.Context, data Transformer[T], selector T) {
	res, err := data.Transform(c.Request.Context(), selector)
	if err != nil {
		Error(c, err, http.StatusInternalServerError)
		return
	}
	Response(c, http.StatusOK, nil, res)
}

func Success(c *gin.Context, data any) {
	if tx, ok := c.Get(shared.TrxInstance); ok && tx != nil {
		switch v := tx.(type) {
		case db.Committer:
			if helper.IsNil(v) {
				Error(c, fmt.Errorf("transaction instance is nil"))
				return
			}
			if err := v.Commit(); err != nil {
				Error(c, err)
				return
			}
		default:
			Error(c, fmt.Errorf("unknown transaction instance, got:%T", tx))
			return
		}
	}
	// Has anything (header or body) already been written?
	if c.Writer.Written() {
		// yes—don’t try to write again
		return
	}
	Response(c, http.StatusOK, nil, data)
}

func Error(c *gin.Context, err error, code ...int) {
	httpCode := http.StatusBadRequest
	if len(code) > 0 {
		httpCode = code[0]
	}
	if tx, ok := c.Get(shared.TrxInstance); ok && tx != nil {
		switch v := tx.(type) {
		case db.Rollbacker:
			if helper.IsNil(v) {
				break
			}
			if err := v.Rollback(); err != nil {
				helper.Trace(c, "api.Error.rollback", err)
			}
		default:
			helper.Trace(c, "api.Error.rollback", serror.Newf("unknown transaction instance, got:%v", v))
		}
	}
	Response(c, httpCode, serror.NewFromErrors(1, err), nil)
}

func Response(c *gin.Context, code int, error serror.SError, data interface{}) {
	var (
		failed = code >= 400
		ctx    = c.Request.Context()
		span   = trace.SpanFromContext(ctx)
	)

	if failed && error == nil {
		err := errors.New("HTTP response is not ok but no error provided")
		span.RecordError(err)
		span.SetStatus(codes.Code(code), err.Error())
		panic(err) // this condition should not be allowed, so we crash here
	}

	var (
		err       string
		svc       = c.GetString(shared.X_Service)
		ctrl      = c.GetString(shared.X_Controller)
		action    = c.GetString(shared.X_Action)
		recording = span.IsRecording()
		verbose   = shared.Staging || c.Query(shared.X_Trace) == "1"
	)

	if error != nil {
		err = error.Error()
	}

	res := response{
		Code:       code,
		Error:      err,
		Service:    svc,
		Controller: ctrl,
		Action:     action,
		Result:     data,
		Ctx: map[string]any{
			"url": helper.TraceURL(span),
		},
	}

	if verbose {
		res.Ctx[shared.X_UserId] = c.GetString(shared.X_UserId)
		res.Ctx[shared.X_RoleId] = c.GetString(shared.X_RoleId)
		res.Ctx[shared.X_PortalId] = c.GetString(shared.X_PortalId)
		res.Ctx[shared.X_PortalDomain] = c.GetString(shared.X_PortalDomain)
	}

	// monitor error
	if failed && error != nil {
		sf := helper.Reduce(error.StackFrames(), []map[string]any{}, func(res []map[string]any, val errors.StackFrame, i int) []map[string]any {
			return append(res, map[string]any{
				"file": val.File,
				"line": val.LineNumber,
				"pkg":  val.Package,
			})
		})
		if length := len(sf); length > 4 {
			sf = sf[:4]
		}
		if recording {
			span.RecordError(error)
			span.SetAttributes(attribute.String(shared.ExceptionSource, fmt.Sprintf("%s:%d %s", error.File(), error.Line(), error.FN())))
			span.SetAttributes(attribute.String(shared.ExceptionStackFrames, helper.ToJsonIndent(sf)))
			span.SetStatus(codes.Code(code), http.StatusText(code))
		}
		if verbose {
			res.Trace = sf
		}
	}

	c.JSON(code, res)
}
