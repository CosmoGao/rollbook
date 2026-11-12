package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIProfilesAndOutput(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer second-secret" {
			t.Errorf("wrong profile key")
		}
		paths = append(paths, r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"name":"测试","token":"hidden-secret","rev":2}]`))
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg := cliConfig{Default: "first", Profiles: map[string]cliProfile{"first": {URL: server.URL, Key: "first-secret"}, "家庭": {URL: server.URL, Key: "second-secret"}}}
	if err := writeCLIConfig(path, cfg); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("mode: %v %v", info, err)
	}
	t.Setenv("ROLLBOOK_KEY", "")
	t.Setenv("ROLLBOOK_URL", "")
	for _, format := range []string{"json", "yaml", "table"} {
		var out bytes.Buffer
		r := cliRunner{out: &out, configPath: path}
		if err := r.run([]string{"--profile", "家庭", "--output", format, "search", "测试"}); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out.String(), "hidden-secret") || !strings.Contains(out.String(), "测试") {
			t.Fatalf("%s output: %s", format, out.String())
		}
	}
	if len(paths) != 3 || paths[0] != "/api/search?q=%E6%B5%8B%E8%AF%95" {
		t.Fatal(paths)
	}
}
func TestCLIRequestRoutes(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		if r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/api/assets/") {
			w.Write([]byte(`{"rev":3}`))
			return
		}
		if r.Method != "GET" {
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if r.Method == "PATCH" && body["rev"] != float64(3) {
				t.Error(body)
			}
		}
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	t.Setenv("ROLLBOOK_URL", server.URL)
	t.Setenv("ROLLBOOK_KEY", "secret")
	r := cliRunner{out: &bytes.Buffer{}, configPath: filepath.Join(t.TempDir(), "missing")}
	for _, args := range [][]string{
		{"create", `{"id":"host:x"}`, "新增"},
		{"propose", "create", `{"id":"host:y"}`, "新增"},
		{"update", "host:x", "--set", "name=新名", "--reason", "更正"},
		{"propose", "host:x", "--set", "fields.kind=实体", "--reason", "建议"},
		{"note", "host:x", "--body", "记录", "--reason", "巡检"},
		{"verify", "host:x", "--reason", "巡检"},
		{"changes", "--pending"}, {"topology", "service:x"}, {"site", "location:x"},
		{"versions", "--outdated"}, {"versions", "--check"}, {"context", "get"},
		{"context", "put", `{"id":"context:x","name":"提示"}`},
	} {
		if err := r.run(args); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	want := []string{"POST /api/assets", "POST /api/changes/proposals", "GET /api/assets/host:x", "PATCH /api/assets/host:x", "GET /api/assets/host:x", "POST /api/changes/proposals", "GET /api/assets/host:x", "POST /api/notes", "GET /api/assets/host:x", "POST /api/assets/host:x/verify", "GET /api/changes", "GET /api/services/service:x/topology", "GET /api/sites/location:x", "GET /api/versions/outdated", "POST /api/versions/check", "GET /api/contexts", "PUT /api/contexts/context:x"}
	if strings.Join(requests, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v\nwant %v", requests, want)
	}
}
func TestCLIHelpErrorsAndCompletion(t *testing.T) {
	r := cliRunner{out: &bytes.Buffer{}, configPath: filepath.Join(t.TempDir(), "missing")}
	for _, args := range [][]string{{"--help"}, {"version"}, {"completion", "bash"}, {"completion", "zsh"}, {"completion", "fish"}} {
		if err := r.run(args); err != nil {
			t.Fatal(args, err)
		}
	}
	for status, word := range map[int]string{401: "认证失败", 403: "权限不足", 409: "重新 get", 422: "字段"} {
		got := cliHTTPError(status, []byte(`{"fields":[{"field":"name","reason":"字段错误"}]}`)).Error()
		if !strings.Contains(got, word) {
			t.Fatal(status, got)
		}
	}
}

func TestCLILoginSavesSecretWithoutEcho(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rollbook", "config.toml")
	var out, prompt bytes.Buffer
	r := cliRunner{in: strings.NewReader("https://example.invalid\nprivate-key\n"), out: &out, err: &prompt, configPath: path}
	if err := r.run([]string{"login", "--profile", "家庭"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String()+prompt.String(), "private-key") {
		t.Fatal("key echoed")
	}
	cfg, err := readCLIConfig(path)
	if err != nil || cfg.Profiles["家庭"].Key != "private-key" || cfg.Default != "家庭" {
		t.Fatalf("config: %v %v", cfg, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("mode: %v %v", info, err)
	}
}
