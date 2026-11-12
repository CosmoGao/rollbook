package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
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

func TestBrowserMaintenanceAPI(t *testing.T) {
	ctx := context.Background()
	database, err := db.Open(ctx, filepath.Join(t.TempDir(), "web.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	a := auth.Service{DB: database, Pepper: []byte("0123456789abcdef0123456789abcdef")}
	if _, err = a.BootstrapUser(ctx, "owner", "strong password 12345"); err != nil {
		t.Fatal(err)
	}
	h := (httpapi.Engine{Auth: a, Catalog: catalog.Service{Store: catalog.Store{DB: database}}, AllowInsecureHTTP: true}).Handler()
	var cookies []*http.Cookie
	csrf := ""
	call := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Host = "rollbook.test"
		r.Header.Set("Origin", "https://rollbook.test")
		if csrf != "" {
			r.Header.Set("X-CSRF-Token", csrf)
		}
		for _, c := range cookies {
			r.AddCookie(c)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	login := call("POST", "/api/auth/login", `{"login":"owner","password":"strong password 12345"}`)
	if login.Code != 200 {
		t.Fatal(login.Code, login.Body)
	}
	cookies = login.Result().Cookies()
	for _, c := range cookies {
		if c.Name == "rollbook_csrf" {
			csrf = c.Value
		}
	}
	for _, item := range []struct{ id, typ, name, fields string }{
		{"location:site", "location", "站点", `{"kind":"站点"}`},
		{"network:net", "network", "网段", `{"cidr":"192.0.2.0/24","location_id":"location:site"}`},
		{"host:box", "host", "主机", `{"kind":"物理机","location_id":"location:site"}`},
		{"ip:one", "ip", "192.0.2.5", `{"address":"192.0.2.5","host_id":"host:box"}`},
		{"service:app", "service", "应用", `{"host_id":"host:box"}`},
	} {
		body := fmt.Sprintf(`{"object":{"id":%q,"type":%q,"name":%q,"fields":%s},"base_rev":0,"reason":"网页录入"}`, item.id, item.typ, item.name, item.fields)
		w := call("POST", "/api/assets", body)
		if w.Code != 200 {
			t.Fatal(item.id, w.Code, w.Body)
		}
	}
	w := call("POST", "/api/assets", `{"object":{"id":"location:source","type":"location","name":"来源枚举","source":"自动发现","fields":{"kind":"站点"}},"base_rev":0,"reason":"验证来源"}`)
	if w.Code != 200 {
		t.Fatal("来源枚举被拒绝", w.Code, w.Body)
	}
	w = call("POST", "/api/assets/host:box/verify", `{"rev":1,"reason":"现场核实"}`)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	w = call("POST", "/api/notes", `{"id":"host:box","rev":2,"kind":"运维笔记","body_md":"已检查","reason":"记录检查"}`)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	var note struct {
		ID string `json:"id"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &note); err != nil {
		t.Fatal(err)
	}
	w = call("PATCH", "/api/assets/"+note.ID, `{"rev":1,"patch":{"fields":{"body_md":"覆盖"}},"reason":"修改笔记"}`)
	if w.Code != 422 || !strings.Contains(w.Body.String(), "fields.body_md") {
		t.Fatal("笔记编辑未返回字段错误", w.Code, w.Body)
	}
	w = call("GET", "/api/hosts/host:box/offline", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "ip:one") {
		t.Fatal(w.Code, w.Body)
	}
	w = call("POST", "/api/hosts/host:box/offline", `{"rev":3,"reason":"下线","options":{"services":true,"ips":true}}`)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	var op struct {
		OperationID string `json:"operation_id"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &op); err != nil || op.OperationID == "" {
		t.Fatal(w.Body, err)
	}
	w = call("GET", "/api/search?q=主机", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"status":"停用"`) {
		t.Fatal(w.Code, w.Body)
	}
	w = call("POST", "/api/hosts/host:box/restore", fmt.Sprintf(`{"operation_id":%q,"reason":"恢复"}`, op.OperationID))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	w = call("DELETE", "/api/assets/host:box", `{"rev":5,"reason":"清理"}`)
	if w.Code != 422 || !strings.Contains(w.Body.String(), "service:app") {
		t.Fatal(w.Code, w.Body)
	}
	w = call("GET", "/api/changes?object_id=host:box", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "host_restore") || !strings.Contains(w.Body.String(), "record_note") {
		t.Fatal(w.Code, w.Body)
	}
	w = call("GET", "/api/assets/host:box/detail", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), note.ID) || !strings.Contains(w.Body.String(), "service:app") {
		t.Fatal("统一详情缺少笔记或反向引用", w.Code, w.Body)
	}
	w = call("GET", "/api/changes?object_type=host&since=2020-01-01T00:00:00Z", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "host_restore") || strings.Contains(w.Body.String(), `"object_type":"service"`) {
		t.Fatal("变更筛选失败", w.Code, w.Body)
	}
}
