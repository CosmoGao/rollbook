package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNetworkCommandUsesInventory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/api/networks/network:test/inventory" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"network:test","occupancy":{"cidr":"192.0.2.0/30","next_available":"192.0.2.1","used":[],"free":[]}}`))
	}))
	defer server.Close()
	t.Setenv("ROLLBOOK_URL", server.URL)
	t.Setenv("ROLLBOOK_KEY", "test-key")
	var out bytes.Buffer
	r := cliRunner{client: server.Client(), out: &out, configPath: t.TempDir() + "/missing.toml"}
	if err := r.run([]string{"--output", "json", "network", "network:test"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"next_available": "192.0.2.1"`) && !strings.Contains(out.String(), `"next_available":"192.0.2.1"`) {
		t.Fatal(out.String())
	}
}
