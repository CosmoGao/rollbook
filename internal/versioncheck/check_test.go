package versioncheck_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/CosmoGao/rollbook/internal/auth"
	"github.com/CosmoGao/rollbook/internal/catalog"
	"github.com/CosmoGao/rollbook/internal/db"
	"github.com/CosmoGao/rollbook/internal/versioncheck"
)

func TestProvidersPreserveInventoryAndFailure(t *testing.T) {
	ctx := context.Background()
	database, err := db.Open(ctx, filepath.Join(t.TempDir(), "checks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	store := catalog.Store{DB: database}
	add := func(id, typ string, fields map[string]any) {
		t.Helper()
		if _, err := store.Create(ctx, catalog.Object{ID: id, Type: typ, Name: id, Fields: fields}, "seed"); err != nil {
			t.Fatal(err)
		}
	}
	add("location:site", "location", map[string]any{"kind": "站点"})
	add("host:machine", "host", map[string]any{"kind": "物理机", "location_id": "location:site"})
	configs := []struct {
		id  string
		cfg versioncheck.Config
		old string
	}{
		{"github", versioncheck.Config{Provider: "github_releases", Repository: "org/repo"}, "v1.2.0"},
		{"docker", versioncheck.Config{Provider: "docker_hub", Image: "org/image"}, "1.3"},
		{"eol", versioncheck.Config{Provider: "endoflife_date", Product: "go"}, "stable"},
	}
	for _, x := range configs {
		b, _ := json.Marshal(x.cfg)
		add("service:"+x.id, "service", map[string]any{"host_id": "host:machine", "upstream_json": string(b)})
		add("version:"+x.id, "version", map[string]any{"service_id": "service:" + x.id, "current_version": x.old, "latest_version": "0.1", "pinned": 1, "pinned_reason": "compatibility"})
	}
	githubMode := "ok"
	dockerMode := "ok"
	eolMode := "ok"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/repos/"):
			if githubMode == "limit" {
				w.WriteHeader(429)
				return
			}
			if githubMode == "timeout" {
				<-r.Context().Done()
				return
			}
			w.Write([]byte(`{"tag_name":"v1.5.0","html_url":"https://github.com/org/repo/releases/tag/v1.5.0"}`))
		case strings.HasPrefix(r.URL.Path, "/v2/"):
			if dockerMode == "limit" {
				w.WriteHeader(429)
				return
			}
			w.Write([]byte(`{"results":[{"name":"latest"},{"name":"1.8.0"},{"name":"1.7.0"}]}`))
		case strings.HasPrefix(r.URL.Path, "/api/v1/"):
			if eolMode == "limit" {
				w.WriteHeader(429)
				return
			}
			w.Write([]byte(`{"result":{"releases":[{"latest":{"name":"1.6.2","link":"https://go.dev/doc"},"eolFrom":"2027-01-01"},{"latest":{"name":"1.5.9"},"eolFrom":"2026-01-01"}]}}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	checker := versioncheck.Checker{DB: database, GitHubURL: srv.URL, DockerURL: srv.URL, EOLURL: srv.URL, Client: &http.Client{Timeout: 50 * time.Millisecond}}
	result, err := checker.Run(ctx, false)
	if err != nil || result.Checked != 3 || result.Failed != 0 {
		t.Fatalf("first run: %+v %v", result, err)
	}
	expected := map[string]string{"github": "v1.5.0", "docker": "1.8.0", "eol": "1.6.2"}
	for id, want := range expected {
		o, err := store.Get(ctx, "version:"+id)
		if err != nil {
			t.Fatal(err)
		}
		if o.Fields["latest_version"] != want || o.Fields["pinned_reason"] != "compatibility" || o.Rev != 1 || o.Fields["checked_at"] == nil {
			t.Fatalf("changed inventory %s: %+v", id, o)
		}
	}
	q := catalog.Query{Service: catalog.Service{Store: store}}
	list, err := q.Outdated(ctx, auth.Principal{UserID: "owner"})
	if err != nil || len(list) != 3 {
		t.Fatalf("outdated list: %+v %v", list, err)
	}
	states := map[string]string{}
	for _, v := range list {
		states[v.ID] = v.Fields["version_state"].(string)
	}
	if states["version:eol"] != "待人工确认" || states["version:docker"] != "有新版本" {
		t.Fatal(states)
	}
	githubMode = "timeout"
	dockerMode = "limit"
	eolMode = "limit"
	result, err = checker.Run(ctx, false)
	if err != nil || result.Failed != 3 {
		t.Fatalf("failure run: %+v %v", result, err)
	}
	for id, want := range expected {
		o, err := store.Get(ctx, "version:"+id)
		if err != nil {
			t.Fatal(err)
		}
		if o.Fields["latest_version"] != want {
			t.Fatalf("failed upstream overwrote %s: %+v", id, o)
		}
		var msg string
		if err := database.QueryRow(`SELECT last_error FROM version_checks WHERE service_id=?`, "service:"+id).Scan(&msg); err != nil || msg == "" {
			t.Fatalf("missing error %s: %s %v", id, msg, err)
		}
	}
}
func TestCompareConservative(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		cmp  int
		ok   bool
	}{{"v1.2", "1.3.0", -1, true}, {"1.3", "v1.2", 1, true}, {"1.2.0", "v1.2", 0, true}, {"stable", "v2", 0, false}, {"1.2-rc1", "1.2", 0, false}} {
		got, ok := versioncheck.Compare(tc.a, tc.b)
		if got != tc.cmp || ok != tc.ok {
			t.Fatalf("compare %s %s: %d %v", tc.a, tc.b, got, ok)
		}
	}
}

func TestScheduledCreatesVersionAndSkipsFresh(t *testing.T) {
	ctx := context.Background()
	database, err := db.Open(ctx, filepath.Join(t.TempDir(), "scheduled.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	store := catalog.Store{DB: database}
	for _, o := range []catalog.Object{
		{ID: "location:site", Type: "location", Name: "site", Fields: map[string]any{"kind": "站点"}},
		{ID: "host:machine", Type: "host", Name: "machine", Fields: map[string]any{"kind": "物理机", "location_id": "location:site"}},
		{ID: "service:app", Type: "service", Name: "app", Fields: map[string]any{"host_id": "host:machine", "upstream_json": `{"provider":"github_releases","repository":"org/repo"}`}},
	} {
		if _, err := store.Create(ctx, o, "seed"); err != nil {
			t.Fatal(err)
		}
	}
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++; w.Write([]byte(`{"tag_name":"v2.0.0"}`)) }))
	defer srv.Close()
	current := time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)
	checker := versioncheck.Checker{DB: database, GitHubURL: srv.URL, Now: func() time.Time { return current }}
	first, err := checker.Run(ctx, true)
	if err != nil || first.Checked != 1 {
		t.Fatalf("first %+v %v", first, err)
	}
	detail, err := catalog.Query{Service: catalog.Service{Store: store}}.ServiceDetail(ctx, auth.Principal{UserID: "owner"}, "service:app")
	if err != nil || len(detail.Versions) != 1 || detail.Versions[0].Fields["latest_version"] != "v2.0.0" {
		t.Fatalf("created version %+v %v", detail.Versions, err)
	}
	second, err := checker.Run(ctx, true)
	if err != nil || second.Skipped != 1 || hits != 1 {
		t.Fatalf("fresh %+v %v hits=%d", second, err, hits)
	}
	current = current.Add(25 * time.Hour)
	third, err := checker.Run(ctx, true)
	if err != nil || third.Checked != 1 || hits != 2 {
		t.Fatalf("due %+v %v hits=%d", third, err, hits)
	}
}
