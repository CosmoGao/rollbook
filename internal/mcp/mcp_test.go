package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/CosmoGao/rollbook/internal/auth"
	"github.com/CosmoGao/rollbook/internal/catalog"
	"github.com/CosmoGao/rollbook/internal/db"
	"github.com/CosmoGao/rollbook/internal/httpapi"
	"github.com/CosmoGao/rollbook/internal/mcp"
)

func TestToolsAndREST(t *testing.T) {
	ctx := context.Background()
	database, err := db.Open(ctx, filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	a := auth.Service{DB: database, Pepper: []byte("0123456789abcdef0123456789abcdef")}
	uid, err := a.BootstrapUser(ctx, "owner", "strong password 12345")
	if err != nil {
		t.Fatal(err)
	}
	user := auth.Principal{UserID: uid, Login: "owner"}
	keys := map[auth.Level]string{}
	for _, level := range []auth.Level{auth.Read, auth.Propose, auth.Write} {
		_, raw, e := a.IssueKey(ctx, user, string(level), level, nil, "")
		if e != nil {
			t.Fatal(e)
		}
		keys[level] = raw
	}
	store := catalog.Store{DB: database}
	add := func(id, typ, name string, fields map[string]any) {
		t.Helper()
		if _, e := store.Create(ctx, catalog.Object{ID: id, Type: typ, Name: name, Fields: fields}, "seed"); e != nil {
			t.Fatalf("%s: %v", id, e)
		}
	}
	add("location:bj", "location", "北京", map[string]any{"kind": "站点"})
	add("host:overleaf", "host", "Overleaf 宿主", map[string]any{"kind": "物理机", "location_id": "location:bj"})
	add("network:bj", "network", "北京网段", map[string]any{"cidr": "172.20.1.0/24", "location_id": "location:bj"})
	add("ip:overleaf", "ip", "Overleaf IP", map[string]any{"address": "172.20.1.122", "host_id": "host:overleaf"})
	add("service:overleaf", "service", "Overleaf", map[string]any{"host_id": "host:overleaf"})
	add("port:overleaf", "port", "8080", map[string]any{"service_id": "service:overleaf", "port": 8080, "protocol": "tcp"})
	add("endpoint:overleaf", "endpoint", "Overleaf 入口", map[string]any{"service_id": "service:overleaf", "kind": "反向代理", "url": "https://latex.app.v2rich.com:5443"})
	add("version:overleaf", "version", "Overleaf 版本", map[string]any{"service_id": "service:overleaf", "current_version": "5.0", "latest_version": "5.1"})
	e := httpapi.Engine{Auth: a, Catalog: catalog.Service{Store: store}}
	mux := http.NewServeMux()
	mux.Handle("/api/", e.Handler())
	mux.Handle("/mcp", mcp.Server{Engine: e})
	server := httptest.NewServer(mux)
	defer server.Close()
	request := func(method, path, key string, body any) (int, map[string]any) {
		t.Helper()
		var b []byte
		if body != nil {
			b, _ = json.Marshal(body)
		}
		req, _ := http.NewRequest(method, server.URL+path, bytes.NewReader(b))
		req.Header.Set("Authorization", "Bearer "+key)
		req.Header.Set("Content-Type", "application/json")
		res, er := http.DefaultClient.Do(req)
		if er != nil {
			t.Fatal(er)
		}
		defer res.Body.Close()
		var result map[string]any
		_ = json.NewDecoder(res.Body).Decode(&result)
		return res.StatusCode, result
	}
	readREST := func(path, key string) (int, any) {
		t.Helper()
		req, _ := http.NewRequest("GET", server.URL+path, nil)
		req.Header.Set("Authorization", "Bearer "+key)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var v any
		if err = json.NewDecoder(res.Body).Decode(&v); err != nil {
			t.Fatal(err)
		}
		return res.StatusCode, v
	}
	call := func(name, key string, args map[string]any) (any, bool) {
		t.Helper()
		_, v := request("POST", "/mcp", key, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": name, "arguments": args}})
		result, _ := v["result"].(map[string]any)
		if result == nil {
			t.Fatalf("%s response: %v", name, v)
		}
		failed, _ := result["isError"].(bool)
		return result["structuredContent"], failed
	}
	_, listing := request("POST", "/mcp", keys[auth.Read], map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/list"})
	listed := listing["result"].(map[string]any)["tools"].([]any)
	if len(listed) != 16 {
		t.Fatalf("tools=%d", len(listed))
	}
	reads := []struct{ name, path, id string }{
		{"search", "/api/search?q=Overleaf", ""}, {"get_asset", "/api/assets/host:overleaf", "host:overleaf"},
		{"get_service", "/api/services/Overleaf", "Overleaf"}, {"list_by_host", "/api/hosts/host:overleaf/inventory", "host:overleaf"},
		{"list_by_network", "/api/networks/network:bj/inventory", "network:bj"}, {"get_site", "/api/sites/北京", "北京"},
		{"topology", "/api/services/service:overleaf/topology", "service:overleaf"}, {"list_outdated", "/api/versions/outdated", ""},
		{"get_changes", "/api/changes", ""}, {"get_context", "/api/contexts", ""},
	}
	for _, tc := range reads {
		t.Run(tc.name, func(t *testing.T) {
			args := map[string]any{}
			if tc.id != "" {
				args["id"] = tc.id
			}
			if tc.name == "search" {
				args["q"] = "Overleaf"
			}
			mcpData, failed := call(tc.name, keys[auth.Read], args)
			if failed {
				t.Fatal("MCP failed")
			}
			status, restData := readREST(tc.path, keys[auth.Read])
			if status != 200 {
				t.Fatalf("REST %d: %v", status, restData)
			}
			if !reflect.DeepEqual(mcpData, restData) {
				t.Fatalf("MCP=%v REST=%v", mcpData, restData)
			}
		})
	}
	service, failed := call("get_service", keys[auth.Read], map[string]any{"id": "Overleaf"})
	if failed {
		t.Fatal(service)
	}
	data, _ := json.Marshal(service)
	for _, part := range []string{"172.20.1.122", "8080", "latex.app.v2rich.com", "5.0"} {
		if !bytes.Contains(data, []byte(part)) {
			t.Fatalf("missing %s in %s", part, data)
		}
	}
	writes := []struct {
		name, path, method string
		args               map[string]any
		level              auth.Level
	}{
		{"put_context", "/api/contexts/context:test", "PUT", map[string]any{"id": "context:test", "rev": 0, "reason": "test", "object": map[string]any{"id": "context:test", "type": "context", "name": "test", "fields": map[string]any{"scope": "会话", "key": "test", "body_md": "hello"}}}, auth.Read},
		{"record_note", "/api/notes", "POST", map[string]any{"id": "service:overleaf", "rev": 1, "kind": "运维笔记", "body_md": "hello", "reason": "test"}, auth.Propose},
		{"propose_change", "/api/changes/proposals", "POST", map[string]any{"id": "service:overleaf", "rev": 3, "patch": map[string]any{"name": "Overleaf new"}, "reason": "test"}, auth.Propose},
		{"update", "/api/assets/service:overleaf", "PATCH", map[string]any{"id": "service:overleaf", "rev": 3, "patch": map[string]any{"name": "Overleaf 2"}, "reason": "test"}, auth.Write},
		{"mark_verified", "/api/assets/service:overleaf/verify", "POST", map[string]any{"id": "service:overleaf", "rev": 5, "reason": "test"}, auth.Write},
		{"check_versions", "/api/versions/check", "POST", map[string]any{}, auth.Write},
	}
	for _, tc := range writes {
		t.Run(tc.name, func(t *testing.T) {
			_, failed := call(tc.name, keys[tc.level], tc.args)
			if failed {
				t.Fatal("MCP failed")
			}
			if tc.name == "put_context" {
				tc.args["id"] = "context:rest"
				tc.args["object"].(map[string]any)["id"] = "context:rest"
				tc.path = "/api/contexts/context:rest"
			}
			if tc.name == "record_note" {
				tc.args["rev"] = 2
			}
			if tc.name == "update" {
				tc.args["rev"] = 4
			}
			if tc.name == "mark_verified" {
				tc.args["rev"] = 6
			}
			status, v := request(tc.method, tc.path, keys[tc.level], tc.args)
			if status != 200 {
				t.Fatalf("REST %d: %v", status, v)
			}
		})
	}
	status, _ := request("PATCH", "/api/assets/service:overleaf", keys[auth.Read], map[string]any{"rev": 6, "patch": map[string]any{}, "reason": "test"})
	if status != 403 {
		t.Fatal(status)
	}
	status, _ = request("PATCH", "/api/assets/service:overleaf", keys[auth.Propose], map[string]any{"rev": 6, "patch": map[string]any{}, "reason": "test"})
	if status != 403 {
		t.Fatal(status)
	}
	status, _ = request("PATCH", "/api/assets/service:overleaf", keys[auth.Write], map[string]any{"rev": 1, "patch": map[string]any{}, "reason": "test"})
	if status != 409 {
		t.Fatal(status)
	}
	status, _ = request("PATCH", "/api/assets/service:overleaf", keys[auth.Write], map[string]any{"rev": 6, "patch": map[string]any{"name": "password=abc"}, "reason": "test"})
	if status != 422 {
		t.Fatal(status)
	}
	status, _ = request("GET", "/api/keys", keys[auth.Write], nil)
	if status != 403 {
		t.Fatal(status)
	}
	status, _ = request("POST", "/api/changes/anything/approve", keys[auth.Write], map[string]any{})
	if status != 403 {
		t.Fatal(status)
	}
	p, err := a.Authenticate(ctx, keys[auth.Read])
	if err != nil {
		t.Fatal(err)
	}
	if err = a.Revoke(ctx, user, p.KeyID); err != nil {
		t.Fatal(err)
	}
	status, _ = request("GET", "/api/search?q=test", keys[auth.Read], nil)
	if status != 401 {
		t.Fatal(status)
	}
}
