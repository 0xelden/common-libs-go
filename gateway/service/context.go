package service

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strings"

	qs "github.com/derekstavis/go-qs"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/0xelden/common-libs-go/gateway/models"
	"github.com/0xelden/common-libs-go/gateway/packets"
	"github.com/0xelden/common-libs-go/helper"
	"github.com/0xelden/common-libs-go/serror"
)

type Result interface {
	buildResponse(context.Context) (*packets.Response, error)
}

type Context struct {
	requestContext
	responseContext
}

type responseContext struct {
	status   int
	body     models.Response
	bodyType string
	header   map[string]string
}

type requestContext struct {
	path       string
	method     string
	body       []byte
	params     map[string]string
	forms      string
	query      map[string]string
	header     map[string]string
	authInfo   models.AuthorizationInfo
	clientInfo models.ClientInfo
}

func (rc requestContext) Path() string {
	return rc.path
}

func (rc requestContext) Method() string {
	return rc.method
}

func (rc requestContext) RawBody() []byte {
	return rc.body
}

func (rc requestContext) BodyBind(v interface{}) serror.SError {
	ctype := helper.CleanSpit(rc.Header(models.ContentTypeHeaderKey), ";")
	switch ctype[0] {
	case "text/json", "application/json":
		err := rc.BodyJSONBind(v)
		if err != nil {
			return serror.NewFromErrorc(err, "Failed to reading json")
		}

	case "text/xml", "application/xml":
		errx := rc.BodyXMLBind(v)
		if errx != nil {
			errx.AddComments("while body xml bind")
			return errx
		}

	case "text/plain", "multipart/form-data", "application/x-www-form-urlencoded":
		res, err := qs.Unmarshal(string(rc.body))
		if err != nil {
			return serror.NewFromErrorc(err, "Failed to unmarshal query")
		}

		byt, err := json.Marshal(res)
		if err != nil {
			return serror.NewFromErrorc(err, "Failed to marshal json")
		}

		err = json.Unmarshal(byt, v)
		if err != nil {
			return serror.NewFromErrorc(err, "Failed to unmarshal json")
		}

	default:
		return serror.Newc("Unknown request body", "@")
	}

	return nil
}

func (rc requestContext) BodyJSONBind(v interface{}) error {
	err := json.Unmarshal(rc.body, v)
	if err != nil {
		return err
	}

	return nil
}

func (rc requestContext) BodyXMLBind(v interface{}) serror.SError {
	err := xml.Unmarshal(rc.body, v)
	if err != nil {
		return serror.NewFromErrorc(err, "Failed to unmarshaling xml")
	}

	return nil
}

func (rc requestContext) Parameter(key string) string {
	return rc.params[key]
}

func (rc requestContext) Parameterd(key string, def string) string {
	val, ok := rc.params[key]
	if ok {
		return val
	}
	return def
}

func (rc requestContext) Parameters() map[string]string {
	return rc.params
}

func (rc requestContext) PostForm(key string) string {
	stringReader := strings.NewReader(string(rc.RawBody()))
	stringReadCloser := ioutil.NopCloser(stringReader)

	contentType := rc.Header("X-Content-Type")
	if contentType == "" {
		contentType = rc.Header("Content-Type")
	}

	req := &http.Request{
		Method: rc.Method(),
		Header: map[string][]string{
			"Content-Type": {contentType},
		},
		Body: stringReadCloser,
	}

	req.ParseMultipartForm(0)
	for k, v := range req.Form {
		re := regexp.MustCompile("([a-z_]+)" + "(\\[([a-z0-9_]+)\\])?")
		matches := re.FindStringSubmatch(k)

		if len(matches) >= 4 {
			if matches[2] == "" {
				if k == key {
					return v[0]
				}
				continue
			}
		}
	}
	return ""
}

func (rc requestContext) DefaultPostForm(key string, def string) string {
	stringReader := strings.NewReader(string(rc.RawBody()))
	stringReadCloser := ioutil.NopCloser(stringReader)

	contentType := rc.Header("X-Content-Type")
	if contentType == "" {
		contentType = rc.Header("Content-Type")
	}

	req := &http.Request{
		Method: rc.Method(),
		Header: map[string][]string{
			"Content-Type": {contentType},
		},
		Body: stringReadCloser,
	}

	req.ParseMultipartForm(0)
	for k, v := range req.Form {
		re := regexp.MustCompile("([a-z_]+)" + "(\\[([a-z0-9_]+)\\])?")
		matches := re.FindStringSubmatch(k)

		if len(matches) >= 4 {
			if matches[2] == "" {
				if k == key {
					return v[0]
				}
				continue
			}
		}
	}
	return def
}

func (rc requestContext) Query(key string) string {
	return rc.query[key]
}

func (rc requestContext) DefaultQuery(key string, def string) string {
	val, ok := rc.query[key]
	if ok {
		return val
	}
	return def
}

func (rc requestContext) Queries() map[string]string {
	return rc.query
}

func (rc requestContext) ContentType() string {
	return rc.XHeader(models.BvContentTypeHeaderKey)
}

func (rc requestContext) ClientIP() string {
	return rc.XHeader(models.BvRealIPTypeHeaderKey)
}

func (rc requestContext) ClientIPProof() bool {
	return helper.ToBool(rc.XHeader(models.BvRealIPProofTypeHeaderKey), false)
}

func (rc requestContext) Header(key string) string {
	return rc.header[http.CanonicalHeaderKey(fmt.Sprintf("bv-%s", key))]
}

func (rc requestContext) Headerd(key string, def string) string {
	if val, ok := rc.header[http.CanonicalHeaderKey(fmt.Sprintf("bv-%s", key))]; ok {
		return val
	}
	return def
}

func (rc requestContext) Headers() (res map[string]string) {
	res = make(map[string]string)
	for k, v := range rc.header {
		if strings.HasPrefix(strings.ToLower(k), "bv-") {
			res[helper.Sub(k, 3, 0)] = v
		}
	}

	return res
}

func (rc requestContext) XHeader(key string) string {
	return rc.header[http.CanonicalHeaderKey(key)]
}

func (rc requestContext) XHeaderd(key string, def string) string {
	if val, ok := rc.header[http.CanonicalHeaderKey(key)]; ok {
		return val
	}
	return def
}

func (rc requestContext) XHeaders() map[string]string {
	return rc.header
}

func (rc requestContext) AuthorizationInfo() models.AuthorizationInfo {
	return rc.authInfo
}

func (rc requestContext) ClientInfo() models.ClientInfo {
	return rc.clientInfo
}

func (ctx *responseContext) SetHeader(key, value string) {
	ctx.header[http.CanonicalHeaderKey(key)] = value
}

func (ctx responseContext) SetContentType(mime string) {
	ctx.SetHeader(models.BvContentTypeHeaderKey, mime)
}

func (ctx responseContext) XMLResponse(status int, body models.ResponseBody) Result {
	if body.Result == nil {
		switch {
		default:
			body.Result = models.DefaultMessage
		case status >= 400:
			body.Result = ""
		}
	}

	switch {
	case body.Action == "POST":
		ctx.body.Action = "Add"
	case body.Action == "PUT":
		ctx.body.Action = "Edit"
	case body.Action == "DELETE":
		ctx.body.Action = "Delete"
	case body.Action == "GET":
		ctx.body.Action = "Get"
	default:
		ctx.body.Action = ""
	}

	ctx.status = status
	ctx.bodyType = models.BodyTypeJSON
	ctx.body.Response = status
	ctx.body.Error = body.Error
	ctx.body.Appid = body.Appid
	ctx.body.Svcid = body.Svcid
	ctx.body.Controller = body.Controller
	ctx.body.Action = body.Action
	ctx.body.Result = body.Result

	return ctx
}

func (ctx responseContext) JSONResponse(status int, body models.ResponseBody) Result {
	if body.Result == nil {
		switch {
		default:
			body.Result = models.DefaultMessage
		case status >= 400:
			body.Result = ""
		}
	}

	switch {
	case body.Action == "POST":
		ctx.body.Action = "Add"
	case body.Action == "PUT":
		ctx.body.Action = "Edit"
	case body.Action == "DELETE":
		ctx.body.Action = "Delete"
	case body.Action == "GET":
		ctx.body.Action = "Get"
	default:
		ctx.body.Action = ""
	}

	ctx.status = status
	ctx.bodyType = models.BodyTypeJSON
	ctx.body.Response = status
	ctx.body.Error = body.Error
	ctx.body.Appid = body.Appid
	ctx.body.Svcid = body.Svcid
	ctx.body.Controller = body.Controller
	ctx.body.Result = body.Result

	return ctx
}

func (ctx responseContext) RawResponse(status int, body []byte) Result {
	ctx.status = status
	ctx.bodyType = models.BodyTypeRaw

	ctx.body.Result = body
	ctx.body.Response = status

	return ctx
}

func (ctx responseContext) Redirect(url string) Result {
	ctx.SetHeader("location", url)
	return ctx.RawResponse(http.StatusMovedPermanently, []byte{})
}

func (ctx responseContext) buildResponse(gCtx context.Context) (*packets.Response, error) {
	var (
		err  error
		body []byte
	)

	switch ctx.bodyType {
	case models.BodyTypeRaw:
		body = ctx.body.Result.([]byte)

	case models.BodyTypeXML:
		ctx.SetContentType(models.BodyTypeXML)
		body, err = xml.Marshal(ctx.body)

		if err != nil {
			return nil, fmt.Errorf("while marshalling xml: %v", err)
		}

	default:
		ctx.SetContentType(models.BodyTypeJSON)
		body, err = json.Marshal(ctx.body)

		if err != nil {
			return nil, fmt.Errorf("while marshalling json: %v", err)
		}
	}

	err = grpc.SendHeader(gCtx, metadata.New(ctx.header))
	if err != nil {
		return nil, fmt.Errorf("while sending grpc header: %v", err)
	}

	host, err := os.Hostname()
	if err != nil {
		host = "?"
	}

	return &packets.Response{
		Server: host,
		Status: int32(ctx.status),
		Body:   body,
	}, nil
}
