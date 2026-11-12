package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestMCPStdioRoundTrip(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mcp" || r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("请求路径或鉴权错误: %s", r.URL.Path)
		}
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":{"method":%q}}`, req.ID, req.Method)
	}))
	defer server.Close()
	var out, stderr bytes.Buffer
	input := strings.NewReader("" +
		`{"jsonrpc":"2.0","id":1,"method":"initialize"}` + "\n" +
		`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n" +
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"search","arguments":{"q":"test"}}}` + "\n")
	r := cliRunner{client: server.Client(), in: input, out: &out, err: &stderr}
	if err := r.mcp(server.URL, "secret"); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 || stderr.Len() != 0 {
		t.Fatalf("stdio 响应=%q stderr=%q", out.String(), stderr.String())
	}
	seen := map[string]string{}
	for _, line := range lines {
		var reply struct {
			ID     json.Number `json:"id"`
			Result struct {
				Method string `json:"method"`
			} `json:"result"`
		}
		if err := json.Unmarshal([]byte(line), &reply); err != nil {
			t.Fatal(err)
		}
		seen[reply.ID.String()] = reply.Result.Method
	}
	if seen["1"] != "initialize" || seen["2"] != "tools/list" || seen["3"] != "tools/call" {
		t.Fatalf("收到错误响应: %v", seen)
	}
}

func TestMCPReadsProfile(t *testing.T) {
	t.Setenv("ROLLBOOK_URL", "")
	t.Setenv("ROLLBOOK_KEY", "")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer profile-key" {
			t.Error("未使用所选 profile 的 Key")
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"result":{}}`)
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := writeCLIConfig(path, cliConfig{Default: "unused", Profiles: map[string]cliProfile{
		"office": {URL: server.URL, Key: "profile-key"},
	}}); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	r := cliRunner{client: server.Client(), in: strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n"), out: &out, err: &stderr, configPath: path}
	if err := r.run([]string{"--profile", "office", "mcp"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"result"`) || stderr.Len() != 0 {
		t.Fatalf("stdout=%q stderr=%q", out.String(), stderr.String())
	}
}
