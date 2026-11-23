package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPatchServiceHTTPRoutesSkipsGuardedRoutes(t *testing.T) {
	tmpDir := t.TempDir()
	routeDir := filepath.Join(tmpDir, "service", "http")
	if err := os.MkdirAll(routeDir, 0o755); err != nil {
		t.Fatalf("mkdir route dir: %v", err)
	}

	existing := `package http

import (
	"gitlab.com/example/project/modules/demo_table"
)

func (ox gatewayHandler) initRoute() {
	var (
		demoTableCtrl = demo_table.NewDemoTableUsecase(ox.driver)
	)
	{
		ctrl := "/demo-table"
		router := ox.service.Group(ctrl)
		router.GET("/index", ox.MiddlewareGuard([]string{"list_demo"}), demo_table.IndexDemoTable(demoTableCtrl))
		router.POST("/add", ox.MiddlewareGuard([]string{"create_demo"}), demo_table.SaveDemoTable(ox.trxBuilder))
		router.PUT("/edit", ox.MiddlewareGuard([]string{"edit_demo"}), demo_table.CustomEditDemoTable(ox.trxBuilder))
	}
}
`
	routePath := filepath.Join(routeDir, "route.go")
	if err := os.WriteFile(routePath, []byte(existing), 0o644); err != nil {
		t.Fatalf("write route.go: %v", err)
	}

	generatedHTTP := `package demo_table

func IndexDemoTable(ctrl *DemoTableCtrl) gin.HandlerFunc { return nil }
func SaveDemoTable(txb db.TrxBuilder) gin.HandlerFunc { return nil }
func ViewDemoTable(ctrl *DemoTableCtrl) gin.HandlerFunc { return nil }
func EditDemoTable(txb db.TrxBuilder) gin.HandlerFunc { return nil }
func DeleteDemoTable(txb db.TrxBuilder) gin.HandlerFunc { return nil }
`

	content, _, err := patchServiceHTTPRoutes(routePath, "gitlab.com/example/project", generatedHTTP)
	if err != nil {
		t.Fatalf("patchServiceHTTPRoutes returned error: %v", err)
	}

	for _, route := range []string{`router.GET("/index"`, `router.POST("/add"`, `router.PUT("/edit"`} {
		if n := strings.Count(content, route); n != 1 {
			t.Fatalf("expected %s registered once, got %d:\n%s", route, n, content)
		}
	}
	if !strings.Contains(content, `ox.MiddlewareGuard([]string{"create_demo"}), demo_table.SaveDemoTable(ox.trxBuilder)`) {
		t.Fatalf("expected guarded route to be preserved:\n%s", content)
	}
	if !strings.Contains(content, `router.DELETE("/delete/:id", demo_table.DeleteDemoTable(ox.trxBuilder))`) {
		t.Fatalf("expected missing delete route to be added:\n%s", content)
	}
}
