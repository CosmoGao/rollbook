package main

import (
	"bytes"
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
	"github.com/CosmoGao/rollbook/internal/mcp"
)

func TestDuplicateIPAcrossTransports(t *testing.T) {
	ctx := context.Background()
	database, err := db.Open(ctx, filepath.Join(t.TempDir(), "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	store := catalog.Store{DB: database}
	seed := func(id, typ, name string, fields map[string]any) {
		t.Helper()
		if _, e := store.Create(ctx, catalog.Object{ID: id, Type: typ, Name: name, Fields: fields}, "seed"); e != nil {
			t.Fatal(e)
		}
	}
	seed("location:site", "location", "站点", map[string]any{"kind": "站点"})
	seed("network:net", "network", "网段", map[string]any{"cidr": "192.0.2.0/24", "location_id": "location:site"})
	seed("host:owner", "host", "已占用主机", map[string]any{"kind": "物理机", "location_id": "location:site"})
	seed("host:other", "host", "新主机", map[string]any{"kind": "物理机", "location_id": "location:site"})
	seed("ip:existing", "ip", "原有 IP", map[string]any{"address": "192.0.2.5", "host_id": "host:owner"})
	seed("ip:other", "ip", "另一 IP", map[string]any{"address": "192.0.2.6", "host_id": "host:other"})
	authService := auth.Service{DB: database, Pepper: []byte("0123456789abcdef0123456789abcdef")}
	uid, err := authService.BootstrapUser(ctx, "owner", "strong password 12345")
	if err != nil {
		t.Fatal(err)
	}
	_, key, err := authService.IssueKey(ctx, auth.Principal{UserID: uid, Login: "owner"}, "write", auth.Write, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	engine := httpapi.Engine{Auth: authService, Catalog: catalog.Service{Store: store}}
	mux := http.NewServeMux()
	mux.Handle("/api/", engine.Handler())
	mux.Handle("/mcp", mcp.Server{Engine: engine})
	server := httptest.NewServer(mux)
	defer server.Close()
	request := func(path, method string, body any) map[string]any {
		t.Helper()
		data, _ := json.Marshal(body)
		req, _ := http.NewRequest(method, server.URL+path, bytes.NewReader(data))
		req.Header.Set("Authorization", "Bearer "+key)
		req.Header.Set("Content-Type", "application/json")
		res, e := server.Client().Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		var result map[string]any
		if e = json.NewDecoder(res.Body).Decode(&result); e != nil {
			t.Fatal(e)
		}
		return result
	}
	object := map[string]any{"id": "ip:duplicate", "type": "ip", "name": "重复 IP", "fields": map[string]any{"address": "192.0.2.5", "host_id": "host:other"}}
	args := map[string]any{"object": object, "base_rev": 0, "reason": "重复登记"}
	rest := request("/api/assets", "POST", args)
	assertIPOwner(t, rest)
	if rest["code"] != "validation" {
		t.Fatalf("REST: %v", rest)
	}
	updated := request("/api/assets/ip:other", "PATCH", map[string]any{"rev": 1, "patch": map[string]any{"fields": map[string]any{"address": "192.0.2.5"}}, "reason": "修改地址"})
	assertIPOwner(t, updated)
	mcpResult := request("/mcp", "POST", map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "create", "arguments": args}})
	result, ok := mcpResult["result"].(map[string]any)
	if !ok || result["isError"] != true {
		t.Fatalf("MCP: %v", mcpResult)
	}
	structured, ok := result["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("MCP missing structured error: %v", result)
	}
	assertIPOwner(t, structured)
	t.Setenv("ROLLBOOK_URL", server.URL)
	t.Setenv("ROLLBOOK_KEY", key)
	encoded, _ := json.Marshal(object)
	runner := cliRunner{client: server.Client(), out: &bytes.Buffer{}, configPath: filepath.Join(t.TempDir(), "config.toml")}
	err = runner.run([]string{"create", string(encoded), "重复登记"})
	if err == nil || !strings.Contains(err.Error(), "已占用主机") || !strings.Contains(err.Error(), "192.0.2.5") {
		t.Fatalf("CLI: %v", err)
	}
}

func assertIPOwner(t *testing.T, response map[string]any) {
	t.Helper()
	fields, ok := response["fields"].([]any)
	if !ok || len(fields) != 1 {
		t.Fatalf("missing field error: %v", response)
	}
	field := fields[0].(map[string]any)
	if field["field"] != "fields.address" || !strings.Contains(field["reason"].(string), "占用") {
		t.Fatalf("wrong field error: %v", field)
	}
	refs, ok := field["references"].([]any)
	if !ok || len(refs) != 1 || !strings.Contains(refs[0].(string), "已占用主机") || !strings.Contains(refs[0].(string), "192.0.2.5") {
		t.Fatalf("missing owner: %v", field)
	}
}
