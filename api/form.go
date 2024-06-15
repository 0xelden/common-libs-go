package api

import (
	"github.com/gin-gonic/gin"
	"github.com/go-playground/form"
	"github.com/0xelden/common-libs-go/helper"
	"github.com/0xelden/common-libs-go/shared"
	"github.com/0xelden/common-libs-go/types"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var (
	fdec    = form.NewDecoder()
	fenc    = form.NewEncoder()
	maxForm = int64(32 << 20) // 32MB
)

func init() {
	fdec.RegisterCustomTypeFunc((&types.RupiahCent{}).RegisterCustomTypeFunc(), types.RupiahCent{})
	fdec.RegisterCustomTypeFunc((&types.NativeDate{}).RegisterCustomTypeFunc(), types.NativeDate{})
	fdec.RegisterCustomTypeFunc((&types.NativeTime{}).RegisterCustomTypeFunc(), types.NativeTime{})
	fdec.RegisterCustomTypeFunc((&types.JsonObject{}).RegisterCustomTypeFunc(), types.JsonObject{})
	fdec.RegisterCustomTypeFunc((&types.JsonArray{}).RegisterCustomTypeFunc(), types.JsonArray{})
	fenc.RegisterCustomTypeFunc((&types.JsonObject{}).EncodeCustomTypeFunc(), types.JsonObject{})
	fenc.RegisterCustomTypeFunc((&types.JsonArray{}).EncodeCustomTypeFunc(), types.JsonArray{})
}

func BindValidate(c *gin.Context, data any, scene ...string) error {
	if err := FormUnmarshal(c, data); err != nil {
		return err
	}

	var sc string
	if len(scene) > 0 {
		sc = scene[len(scene)-1]
	}

	return shared.ValidateStruct(sc, data)
}

func FormUnmarshal(c *gin.Context, data any) error {
	var (
		span    = trace.SpanFromContext(c.Request.Context())
		record  = span.IsRecording()
		onError = func() {
			if record {
				span.SetAttributes(attribute.String("http.json", helper.ToJsonIndent(data)))
			}
		}
	)

	if err := c.Request.ParseMultipartForm(maxForm); err != nil {
		onError()
		return err
	}

	if err := fdec.Decode(data, c.Request.Form); err != nil {
		onError()
		return err
	}

	if record {
		func() {
			defer func() {
				if r := recover(); r != nil {
					onError()
				}
			}()
			span.SetAttributes(attribute.String("http.form", ToFormData(data)))
		}()
	}

	return nil
}

func ToFormData(data any) (res string) {
	for k, v := range helper.First(fenc.Encode(data)) {
		res += k + ":" + helper.If(len(v) > 0, v[0]+"\n", "\n")
	}
	return
}
