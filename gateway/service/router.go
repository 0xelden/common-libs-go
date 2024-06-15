package service

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/0xelden/common-libs-go/gateway/models"
	"github.com/0xelden/common-libs-go/serror"
)

type HandlerFunc func(ctx *Context) Result

type Middleware func(HandlerFunc) HandlerFunc

type protectedRoute struct {
	pattern *regexp.Regexp
	method  string
}

type router struct {
	routes          map[string][]Route
	protectedRoutes map[string][]protectedRoute
}

type Route struct {
	method  string
	params  []string
	handler HandlerFunc
	rule    *regexp.Regexp
}

func (svc *Service) GET(path string, handler HandlerFunc) Route {
	return svc.router.register(http.MethodGet, path, handler)
}

func (svc *Service) POST(path string, handler HandlerFunc) Route {
	return svc.router.register(http.MethodPost, path, handler)
}

func (svc *Service) PUT(path string, handler HandlerFunc) Route {
	return svc.router.register(http.MethodPut, path, handler)
}

func (svc *Service) DELETE(path string, handler HandlerFunc) Route {
	return svc.router.register(http.MethodDelete, path, handler)
}

func (svc *Service) Protect(routes ...Route) {
	for _, each := range routes {
		svc.router.protectedRoutes[each.method] = append(
			svc.router.protectedRoutes[each.method], protectedRoute{
				method:  "protect",
				pattern: each.rule,
			},
		)
	}
}

func (svc *Service) Strict(routes ...Route) {
	for _, each := range routes {
		svc.router.protectedRoutes[each.method] = append(
			svc.router.protectedRoutes[each.method], protectedRoute{
				method:  "strict",
				pattern: each.rule,
			},
		)
	}
}

func (svc *Service) Private(routes ...Route) {
	for _, each := range routes {
		svc.router.protectedRoutes[each.method] = append(
			svc.router.protectedRoutes[each.method], protectedRoute{
				method:  "private",
				pattern: each.rule,
			},
		)
	}
}

func (svc *Service) Docs(path string) serror.SError {
	var byt []byte

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		var err error

		byt, err = ioutil.ReadFile(path)
		if err != nil {
			return serror.NewFromError(err)
		}
	}

	handler := func(ctx *Context) Result {
		return ctx.RawResponse(http.StatusOK, byt)
	}

	svc.router.register(http.MethodGet, "/api-docs.yaml", handler)
	return nil
}

func (svc *Service) WithMiddleware(handler HandlerFunc, mws ...Middleware) HandlerFunc {
	if len(mws) > 0 {
		handler = mws[len(mws)-1](handler)
		for i := len(mws) - 2; i >= 0; i-- {
			handler = mws[i](handler)
		}
	}

	return handler
}

func (r *router) register(method string, pattern string, handlerFn HandlerFunc) Route {
	var params []string

	tpl := strings.Trim(pattern, "/")
	for opener := strings.Index(tpl, "{"); opener != -1; opener = strings.Index(tpl, "{") {
		closer := strings.Index(tpl, "}")
		if closer == -1 {
			panic(errors.New("} doesn't exist"))
		}

		params = append(params, tpl[opener+1:closer])
		tpl = strings.Replace(tpl, tpl[opener:closer+1], `([\w-_]+)`, 1)
	}

	tpl = strings.Replace(tpl, "/", "/{1}", strings.Count(tpl, "/"))
	tpl = fmt.Sprintf("^/?%s/??$", tpl)
	rule, err := regexp.Compile(tpl)
	if err != nil {
		panic(fmt.Errorf("while compiling regex rule: %v", err))
	}

	for _, route := range r.routes[method] {
		if route.rule.String() == rule.String() {
			panic(errors.New("path already registered"))
		}
	}

	route := Route{
		method:  method,
		params:  params,
		handler: handlerFn,
		rule:    rule,
	}
	r.routes[method] = append(r.routes[method], route)

	return route
}

func (r router) route(ctx *Context) Result {
	for _, each := range r.routes[ctx.requestContext.method] {
		if each.rule.Match([]byte(ctx.requestContext.path)) {
			ss := each.rule.FindStringSubmatch(ctx.requestContext.path)
			for index, key := range each.params {
				ctx.params[key] = ss[index+1]
			}

			return each.handler(ctx)
		}
	}

	return r.notFound(ctx)
}

func (r router) notFound(ctx *Context) Result {
	return ctx.JSONResponse(http.StatusNotFound, models.ResponseBody{
		Error:      "Jalur tidak ditemukan",
		Controller: ctx.path,
		Action:     ctx.method,
	})
}
