package httpapi_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CosmoGao/rollbook/internal/auth"
	"github.com/CosmoGao/rollbook/internal/db"
	"github.com/CosmoGao/rollbook/internal/httpapi"
	"github.com/CosmoGao/rollbook/skills"
)

func TestSkillZipSessionBoundaryAndContents(t *testing.T) {
	ctx := context.Background()
	database, err := db.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	a := auth.Service{DB: database, Pepper: []byte("0123456789abcdef0123456789abcdef")}
	if _, err := a.BootstrapUser(ctx, "owner", "strong password 12345"); err != nil {
		t.Fatal(err)
	}
	h := (httpapi.Engine{Auth: a, AllowInsecureHTTP: true}).Handler()
	call := func(method, path string, cookies []*http.Cookie, bearer string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, nil)
		r.Host = "rollbook.test"
		r.Header.Set("Origin", "https://rollbook.test")
		if bearer != "" {
			r.Header.Set("Authorization", "Bearer "+bearer)
		}
		for _, c := range cookies {
			r.AddCookie(c)
			if c.Name == "rollbook_csrf" && method != http.MethodGet {
				r.Header.Set("X-CSRF-Token", c.Value)
			}
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := call("GET", "/api/agent/skill.zip", nil, ""); w.Code != 401 {
		t.Fatalf("anonymous: %d", w.Code)
	}
	login := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{"login":"owner","password":"strong password 12345"}`))
	login.Host = "rollbook.test"
	login.Header.Set("Origin", "https://rollbook.test")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, login)
	if w.Code != 200 {
		t.Fatalf("login: %d %s", w.Code, w.Body.String())
	}
	cookies := w.Result().Cookies()
	if w := call("GET", "/api/agent/skill.zip", cookies, "invalid"); w.Code != 401 {
		t.Fatalf("invalid bearer: %d", w.Code)
	}
	created := httptest.NewRequest("POST", "/api/keys", strings.NewReader(`{"name":"agent","level":"read"}`))
	created.Host = "rollbook.test"
	created.Header.Set("Origin", "https://rollbook.test")
	for _, c := range cookies {
		created.AddCookie(c)
		if c.Name == "rollbook_csrf" {
			created.Header.Set("X-CSRF-Token", c.Value)
		}
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, created)
	if w.Code != 201 {
		t.Fatalf("create key: %d %s", w.Code, w.Body.String())
	}
	var token struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &token); err != nil {
		t.Fatal(err)
	}
	if w := call("GET", "/api/agent/skill.zip", cookies, token.Token); w.Code != 403 {
		t.Fatalf("bearer key: %d", w.Code)
	}
	for _, path := range []string{"/api/agent/skill.zip", "/api/agent/skill.zip?path=../../go.mod"} {
		w := call("GET", path, cookies, "")
		if w.Code != 200 || w.Header().Get("Content-Type") != "application/zip" || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Content-Disposition") != `attachment; filename="rollbook-skill.zip"` {
			t.Fatalf("download: %d %v", w.Code, w.Header())
		}
		z, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
		if err != nil {
			t.Fatal(err)
		}
		if len(z.File) != 1 || z.File[0].Name != "rollbook/SKILL.md" {
			t.Fatalf("unexpected zip members: %v", z.File)
		}
		file, err := z.File[0].Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(file)
		file.Close()
		if err != nil {
			t.Fatal(err)
		}
		expected, err := skills.Rollbook.ReadFile("rollbook/SKILL.md")
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(content, expected) {
			t.Fatal("Skill content differs from embedded file")
		}
	}
	if w := call("POST", "/api/agent/skill.zip", cookies, ""); w.Code != 405 {
		t.Fatalf("method: %d", w.Code)
	}
}
