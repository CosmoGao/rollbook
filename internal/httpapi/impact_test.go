package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CosmoGao/rollbook/internal/auth"
	"github.com/CosmoGao/rollbook/internal/catalog"
	"github.com/CosmoGao/rollbook/internal/db"
	"github.com/CosmoGao/rollbook/internal/httpapi"
)

func TestImpactRESTAuthorizationAndPreview(t *testing.T) {
	ctx := context.Background()
	database, err := db.Open(ctx, filepath.Join(t.TempDir(), "impact.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	authService := auth.Service{DB: database, Pepper: []byte("0123456789abcdef0123456789abcdef")}
	if _, err = authService.BootstrapUser(ctx, "owner", "strong password 12345"); err != nil {
		t.Fatal(err)
	}
	service := catalog.Service{Store: catalog.Store{DB: database}}
	handler := (httpapi.Engine{Auth: authService, Catalog: service, AllowInsecureHTTP: true}).Handler()
	call := func(path string, cookies []*http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		for _, cookie := range cookies {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if w := call("/api/hosts/host:root/impact?recursive=1", nil); w.Code != 401 {
		t.Fatalf("unauthorized: %d", w.Code)
	}
	for _, object := range []catalog.Object{
		{ID: "location:site", Type: "location", Name: "站点", Fields: map[string]any{"kind": "站点"}},
		{ID: "host:root", Type: "host", Name: "Proxmox", Fields: map[string]any{"kind": "物理机", "location_id": "location:site"}},
		{ID: "host:vm", Type: "host", Name: "虚拟机", Fields: map[string]any{"kind": "虚拟机", "location_id": "location:site", "parent_host_id": "host:root"}},
		{ID: "service:app", Type: "service", Name: "应用", Fields: map[string]any{"host_id": "host:vm"}},
		{ID: "endpoint:web", Type: "endpoint", Name: "入口", Fields: map[string]any{"service_id": "service:app", "kind": "内网直连", "url": "https://example.invalid"}},
	} {
		if _, err = service.Store.Create(ctx, object, "owner"); err != nil {
			t.Fatal(object.ID, err)
		}
	}
	request := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{"login":"owner","password":"strong password 12345"}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatal(response.Body)
	}
	cookies := response.Result().Cookies()
	for _, path := range []string{"/api/hosts/host:root/impact?recursive=1", "/api/services/service:app/impact?recursive=1", "/api/hosts/host:root/offline"} {
		w := call(path, cookies)
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body)
		}
		var data map[string]json.RawMessage
		if err = json.Unmarshal(w.Body.Bytes(), &data); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(path, "offline") {
			if !strings.Contains(string(data["preview"]), "host:vm") {
				t.Fatal(w.Body)
			}
		} else if strings.Contains(path, "hosts") {
			if !strings.Contains(string(data["hosts"]), "host:vm") || !strings.Contains(string(data["endpoints"]), "endpoint:web") {
				t.Fatal(w.Body)
			}
		} else if !strings.Contains(string(data["endpoints"]), "endpoint:web") {
			t.Fatal(w.Body)
		}
	}
}
