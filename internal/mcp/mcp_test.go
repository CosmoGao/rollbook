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
	_, initialization := request("POST", "/mcp", keys[auth.Read], map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize"})
	initResult := initialization["result"].(map[string]any)
	if initResult["instructions"] == "" || initResult["serverInfo"].(map[string]any)["version"] == "1.0.0" {
		t.Fatalf("MCP initialize incomplete: %v", initResult)
	}
	capabilities := initResult["capabilities"].(map[string]any)
	if capabilities["resources"] == nil || capabilities["prompts"] == nil || capabilities["tools"] == nil {
		t.Fatalf("missing capabilities: %v", capabilities)
	}
	rpc := func(key, method string, params map[string]any) (int, map[string]any) {
		t.Helper()
		return request("POST", "/mcp", key, map[string]any{"jsonrpc": "2.0", "id": 7, "method": method, "params": params})
	}
	_, resourceList := rpc(keys[auth.Read], "resources/list", nil)
	resources := resourceList["result"].(map[string]any)["resources"].([]any)
	if len(resources) != 1 || resources[0].(map[string]any)["uri"] != "rollbook://rules" {
		t.Fatalf("resources: %v", resourceList)
	}
	_, templateList := rpc(keys[auth.Read], "resources/templates/list", nil)
	if len(templateList["result"].(map[string]any)["resourceTemplates"].([]any)) != 2 {
		t.Fatalf("templates: %v", templateList)
	}
	for uri, wanted := range map[string]string{"rollbook://rules": "只读 key", "rollbook://sites/location:bj": "北京", "rollbook://services/service:overleaf": "Overleaf"} {
		_, result := rpc(keys[auth.Read], "resources/read", map[string]any{"uri": uri})
		contents, ok := result["result"].(map[string]any)["contents"].([]any)
		if !ok || len(contents) != 1 {
			t.Fatalf("read %s: %v", uri, result)
		}
		content := contents[0].(map[string]any)
		if content["uri"] != uri || !bytes.Contains([]byte(content["text"].(string)), []byte(wanted)) {
			t.Fatalf("read %s: %v", uri, content)
		}
		if content["mimeType"] == "application/json" {
			var parsed any
			if err := json.Unmarshal([]byte(content["text"].(string)), &parsed); err != nil {
				t.Fatalf("invalid resource JSON: %v", err)
			}
		}
	}
	_, promptListing := rpc(keys[auth.Read], "prompts/list", nil)
	if len(promptListing["result"].(map[string]any)["prompts"].([]any)) != 3 {
		t.Fatalf("prompts: %v", promptListing)
	}
	for name, args := range map[string]map[string]any{"register_service": {"name": "新服务"}, "retire_host": {"host_id": "host:overleaf"}, "handoff_context": {"topic": "迁移"}} {
		_, result := rpc(keys[auth.Read], "prompts/get", map[string]any{"name": name, "arguments": args})
		messages, ok := result["result"].(map[string]any)["messages"].([]any)
		if !ok || len(messages) != 1 || messages[0].(map[string]any)["role"] != "user" {
			t.Fatalf("prompt %s: %v", name, result)
		}
	}
	for _, tc := range []struct {
		method string
		params map[string]any
	}{
		{"resources/read", map[string]any{"uri": "rollbook://services/a/b"}},
		{"resources/read", map[string]any{"uri": "rollbook://services/%2e%2e"}},
		{"resources/read", map[string]any{"uri": "rollbook://services/service:overleaf?x=1"}},
		{"resources/read", map[string]any{"uri": "rollbook://services/missing"}},
		{"prompts/get", map[string]any{"name": "register_service", "arguments": map[string]any{}}},
		{"prompts/get", map[string]any{"name": "register_service", "arguments": map[string]any{"name": 123}}},
		{"prompts/get", map[string]any{"name": "unknown", "arguments": map[string]any{"name": "x"}}},
	} {
		_, result := rpc(keys[auth.Read], tc.method, tc.params)
		if result["error"] == nil || result["result"] != nil {
			t.Fatalf("expected protocol error: %v", result)
		}
	}
	for _, method := range []string{"resources/list", "resources/templates/list", "resources/read", "prompts/list", "prompts/get"} {
		status, result := rpc("invalid", method, map[string]any{"uri": "rollbook://services/service:overleaf", "name": "retire_host", "arguments": map[string]any{"host_id": "host:overleaf"}})
		encoded, _ := json.Marshal(result)
		if status != 401 || result["result"] != nil || bytes.Contains(encoded, []byte("Overleaf")) {
			t.Fatalf("unauthenticated %s: %d %v", method, status, result)
		}
	}
	_, listing := request("POST", "/mcp", keys[auth.Read], map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/list"})
	listed := listing["result"].(map[string]any)["tools"].([]any)
	for _, item := range listed {
		tool := item.(map[string]any)
		if len(tool["description"].(string)) < 20 {
			t.Fatalf("missing tool description: %v", tool)
		}
		schema := tool["inputSchema"].(map[string]any)
		if schema["type"] != "object" {
			t.Fatalf("invalid schema: %v", tool)
		}
		if tool["name"] == "update" || tool["name"] == "propose_change" {
			patch := schema["properties"].(map[string]any)["patch"].(map[string]any)
			fields := patch["properties"].(map[string]any)["fields"].(map[string]any)["properties"].(map[string]any)
			for _, name := range []string{"network_id", "host_id", "protocol", "port", "url"} {
				if fields[name] == nil {
					t.Fatalf("patch lacks %s", name)
				}
			}
		}
		if tool["name"] == "create" {
			object := schema["properties"].(map[string]any)["object"].(map[string]any)
			if len(object["oneOf"].([]any)) < 10 {
				t.Fatal("create object variants missing")
			}
		}
	}
	if len(listed) != 17 {
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
	create := map[string]any{"object": map[string]any{"id": "location:new", "type": "location", "name": "新站点", "fields": map[string]any{"kind": "站点"}}, "base_rev": 0, "reason": "新建站点"}
	status, response := request("POST", "/api/assets", keys[auth.Read], create)
	if status != 403 {
		t.Fatal(status, response)
	}
	status, response = request("POST", "/api/assets", keys[auth.Write], create)
	if status != 200 || response["id"] != "location:new" {
		t.Fatal(status, response)
	}
	create["object"].(map[string]any)["id"] = "location:proposed"
	structured, failed := call("create", keys[auth.Propose], create)
	if failed {
		t.Fatal(structured)
	}
	change := structured.(map[string]any)["id"].(string)
	if _, err := store.Get(ctx, "location:proposed"); err == nil {
		t.Fatal("proposal became visible")
	}
	if _, err := e.Catalog.Review(ctx, user, change, true); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(ctx, "location:proposed"); err != nil {
		t.Fatal(err)
	}
	create["object"].(map[string]any)["id"] = "host:invalid"
	create["object"].(map[string]any)["type"] = "host"
	create["object"].(map[string]any)["fields"] = map[string]any{"kind": "物理机"}
	status, response = request("POST", "/api/assets", keys[auth.Write], create)
	if status != 422 || response["code"] != "validation" || response["fields"].([]any)[0].(map[string]any)["field"] != "fields.location_id" {
		t.Fatal(status, response)
	}
	structured, failed = call("create", keys[auth.Write], create)
	if !failed || structured.(map[string]any)["code"] != "validation" {
		t.Fatal(structured, failed)
	}
	create["object"].(map[string]any)["id"] = "ip:outside"
	create["object"].(map[string]any)["type"] = "ip"
	create["object"].(map[string]any)["fields"] = map[string]any{"address": "198.51.100.5", "host_id": "host:overleaf", "network_id": "network:bj"}
	status, response = request("POST", "/api/assets", keys[auth.Write], create)
	if status != 422 || response["fields"].([]any)[0].(map[string]any)["field"] != "fields.address" {
		t.Fatal(status, response)
	}
	create["object"].(map[string]any)["name"] = "password=hidden-value"
	status, response = request("POST", "/api/assets", keys[auth.Write], create)
	encoded, _ := json.Marshal(response)
	if status != 422 || bytes.Contains(encoded, []byte("hidden-value")) || response["fields"].([]any)[0].(map[string]any)["field"] != "object.name" {
		t.Fatal(status, response)
	}
	for _, tc := range []struct {
		object map[string]any
		field  string
	}{
		{map[string]any{"id": "dependency:self", "type": "dependency", "name": "自依赖", "fields": map[string]any{"from_id": "service:overleaf", "to_id": "service:overleaf", "kind": "手工"}}, "fields.to_id"},
		{map[string]any{"id": "endpoint:repeat", "type": "endpoint", "name": "重复反代", "fields": map[string]any{"service_id": "service:overleaf", "kind": "反向代理", "url": "https://repeat.invalid", "via_json": []string{"service:overleaf", "service:overleaf"}}}, "fields.via_json"},
	} {
		input := map[string]any{"object": tc.object, "base_rev": 0, "reason": "校验"}
		status, response = request("POST", "/api/assets", keys[auth.Write], input)
		if status != 422 || response["code"] != "validation" || response["fields"].([]any)[0].(map[string]any)["field"] != tc.field {
			t.Fatal(status, response)
		}
		structured, failed = call("create", keys[auth.Write], input)
		if !failed || structured.(map[string]any)["code"] != "validation" {
			t.Fatal(structured, failed)
		}
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
