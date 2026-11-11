package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCLIUsesBearerAndREST(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("missing Bearer: %q", r.Header.Get("Authorization"))
		}
		switch calls {
		case 1:
			if r.Method != "GET" || r.URL.Query().Get("q") != "Overleaf" {
				t.Errorf("search: %s %s", r.Method, r.URL)
			}
		case 2:
			if r.Method != "GET" || r.URL.Path != "/api/assets/service:overleaf" {
				t.Errorf("get: %s %s", r.Method, r.URL)
			}
		case 3:
			if r.Method != "PATCH" || r.URL.Path != "/api/assets/service:overleaf" {
				t.Errorf("update: %s %s", r.Method, r.URL)
			}
			var v map[string]any
			if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
				t.Error(err)
			}
			if v["rev"] != float64(2) || v["reason"] != "修正" {
				t.Error(v)
			}
		case 4:
			if r.Method != "POST" || r.URL.Path != "/api/assets" {
				t.Errorf("create: %s %s", r.Method, r.URL)
			}
			var v map[string]any
			if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
				t.Error(err)
			}
			if v["base_rev"] != float64(0) || v["reason"] != "新建" {
				t.Error(v)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{}\n"))
	}))
	defer server.Close()
	t.Setenv("ROLLBOOK_URL", server.URL)
	t.Setenv("ROLLBOOK_KEY", "test-key")
	for _, args := range [][]string{{"search", "Overleaf"}, {"get", "service:overleaf"}, {"update", "service:overleaf", "2", `{"name":"新名称"}`, "修正"}, {"create", `{"id":"location:test","type":"location","name":"站点","fields":{"kind":"站点"}}`, "新建"}} {
		if err := runCLI(args); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 4 {
		t.Fatal(calls)
	}
}
