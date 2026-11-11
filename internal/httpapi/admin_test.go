package httpapi_test

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
	"github.com/CosmoGao/rollbook/internal/httpapi"
)

func TestBrowserAdminAndKeyBoundary(t *testing.T) {
	ctx := context.Background()
	database, err := db.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	a := auth.Service{DB: database, Pepper: []byte("0123456789abcdef0123456789abcdef")}
	userID, err := a.BootstrapUser(ctx, "owner", "strong password 12345")
	if err != nil {
		t.Fatal(err)
	}
	h := (httpapi.Engine{Auth: a, Catalog: catalog.Service{Store: catalog.Store{DB: database}}}).Handler()
	call := func(method, path, body, csrf string, cookies []*http.Cookie, bearer string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Host = "rollbook.test"
		r.Header.Set("Origin", "https://rollbook.test")
		if csrf != "" {
			r.Header.Set("X-CSRF-Token", csrf)
		}
		if bearer != "" {
			r.Header.Set("Authorization", "Bearer "+bearer)
		}
		for _, c := range cookies {
			r.AddCookie(c)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	login := call("POST", "/api/auth/login", `{"login":"owner","password":"strong password 12345"}`, "", nil, "")
	if login.Code != 200 {
		t.Fatal(login.Code, login.Body)
	}
	cookies := login.Result().Cookies()
	var csrf string
	for _, c := range cookies {
		if c.Name == "rollbook_csrf" {
			csrf = c.Value
		}
	}
	if csrf == "" {
		t.Fatal("no CSRF cookie")
	}
	if w := call("POST", "/api/keys", `{"name":"agent","level":"read"}`, "", cookies, ""); w.Code != 403 {
		t.Fatal(w.Code)
	}
	created := call("POST", "/api/keys", `{"name":"agent","level":"read"}`, csrf, cookies, "")
	if created.Code != 201 {
		t.Fatal(created.Code, created.Body)
	}
	var result struct {
		Key   auth.Key `json:"key"`
		Token string   `json:"token"`
	}
	if err = json.Unmarshal(created.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(result.Token, "rbk_") {
		t.Fatal("missing token")
	}
	if w := call("GET", "/api/keys", "", "", nil, result.Token); w.Code != 403 {
		t.Fatal(w.Code)
	}
	if w := call("POST", "/api/changes/change:missing/approve", "", "", nil, result.Token); w.Code != 403 {
		t.Fatal(w.Code)
	}
	if w := call("PATCH", "/api/keys/"+result.Key.ID+"/level", `{"level":"write"}`, csrf, cookies, ""); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if p, err := a.Authenticate(ctx, result.Token); err != nil || p.Level != auth.Write {
		t.Fatal(p, err)
	}
	if w := call("POST", "/api/keys/"+result.Key.ID+"/revoke", "", csrf, cookies, ""); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if w := call("GET", "/api/search?q=test", "", "", nil, result.Token); w.Code != 401 {
		t.Fatal(w.Code)
	}
	store := catalog.Store{DB: database}
	if _, err := store.Create(ctx, catalog.Object{ID: "location:old", Type: "location", Name: "旧站点", Fields: map[string]any{"kind": "站点"}}, "owner"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(ctx, catalog.Object{ID: "location:new", Type: "location", Name: "新站点", Fields: map[string]any{"kind": "站点"}}, "owner"); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, "UPDATE objects SET verified_at=? WHERE id=?", time.Now().UTC().Format(time.RFC3339Nano), "location:new"); err != nil {
		t.Fatal(err)
	}
	listed := call("GET", "/api/assets/unverified", "", "", cookies, "")
	if listed.Code != 200 || !strings.Contains(listed.Body.String(), "location:old") || strings.Contains(listed.Body.String(), "location:new") {
		t.Fatal(listed.Code, listed.Body)
	}
	service := catalog.Service{Store: store}
	name := "已批准"
	changeID, err := service.Propose(ctx, auth.Principal{UserID: userID, Login: "owner"}, "location:old", 1, catalog.Patch{Name: &name}, "修正站点", "test")
	if err != nil {
		t.Fatal(err)
	}
	approved := call("POST", "/api/changes/"+changeID+"/approve", "", csrf, cookies, "")
	if approved.Code != 200 {
		t.Fatal(approved.Code, approved.Body)
	}
	updated, err := store.Get(ctx, "location:old")
	if err != nil || updated.Name != "已批准" {
		t.Fatal(updated, err)
	}
	name = "不应生效"
	changeID, err = service.Propose(ctx, auth.Principal{UserID: userID, Login: "owner"}, "location:old", updated.Rev, catalog.Patch{Name: &name}, "再次修正", "test")
	if err != nil {
		t.Fatal(err)
	}
	rejected := call("POST", "/api/changes/"+changeID+"/reject", "", csrf, cookies, "")
	if rejected.Code != 200 {
		t.Fatal(rejected.Code, rejected.Body)
	}
	updated, err = store.Get(ctx, "location:old")
	if err != nil || updated.Name != "已批准" {
		t.Fatal(updated, err)
	}
}
