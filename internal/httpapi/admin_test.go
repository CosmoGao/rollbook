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
	h := (httpapi.Engine{Auth: a, Catalog: catalog.Service{Store: catalog.Store{DB: database}}, AllowInsecureHTTP: true}).Handler()
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
	if w := call("GET", "/api/versions/alerts", "", "", nil, ""); w.Code != 401 {
		t.Fatalf("anonymous alert settings: %d", w.Code)
	}
	if w := call("PUT", "/api/versions/alerts", `{"enabled":true,"webhook_url":"javascript:bad","format":"feishu"}`, csrf, cookies, ""); w.Code != 422 {
		t.Fatalf("invalid webhook: %d %s", w.Code, w.Body.String())
	}
	if w := call("PUT", "/api/versions/alerts", `{"enabled":false,"webhook_url":"http://127.0.0.1:12345/hook","format":"wecom"}`, csrf, cookies, ""); w.Code != 200 {
		t.Fatalf("save webhook: %d %s", w.Code, w.Body.String())
	}
	if w := call("GET", "/api/versions/alerts", "", "", cookies, ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"format":"wecom"`) {
		t.Fatalf("read webhook: %d %s", w.Code, w.Body.String())
	}
	badURL := call("POST", "/api/assets", `{"object":{"id":"endpoint:bad","type":"endpoint","name":"入口","fields":{"url":"javascript:alert(1)"}},"base_rev":0,"reason":"测试"}`, csrf, cookies, "")
	if badURL.Code != 422 || !strings.Contains(badURL.Body.String(), "fields.url") {
		t.Fatal("URL validation", badURL.Code, badURL.Body.String())
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
	metadataPath := "/api/keys/" + result.Key.ID + "/metadata"
	if w := call("PATCH", metadataPath, `{"name":"hijack"}`, "", nil, result.Token); w.Code != 403 {
		t.Fatal("key edited metadata", w.Code, w.Body)
	}
	if w := call("PATCH", metadataPath, `{"name":"renamed"}`, "", cookies, ""); w.Code != 403 {
		t.Fatal("missing csrf", w.Code)
	}
	for _, body := range []string{`{"name":"","note":"ok"}`, `{"name":"bad","expires_at":"yesterday"}`, `{"name":"bad","expires_at":"2020-01-01T00:00:00Z"}`, `{"name":"bad","note":"password: secret"}`} {
		if w := call("PATCH", metadataPath, body, csrf, cookies, ""); w.Code != 422 {
			t.Fatal("invalid metadata", w.Code, w.Body)
		}
	}
	expiry := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	body, _ := json.Marshal(map[string]string{"name": "renamed", "note": "test note", "expires_at": expiry})
	if w := call("PATCH", metadataPath, string(body), csrf, cookies, ""); w.Code != 200 {
		t.Fatal("update metadata", w.Code, w.Body)
	}
	var keys []auth.Key
	if w := call("GET", "/api/keys", "", "", cookies, ""); w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &keys) != nil || len(keys) != 1 || keys[0].Name != "renamed" || keys[0].Note != "test note" || keys[0].ExpiresAt == "" || keys[0].CreatedAt == "" {
		t.Fatal("key list metadata", w.Code, w.Body)
	}
	if p, err := a.Authenticate(ctx, result.Token); err != nil || p.KeyName != "renamed" {
		t.Fatal("key metadata did not take effect", p, err)
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
	if w := call("PATCH", metadataPath, `{"name":"resurrect"}`, csrf, cookies, ""); w.Code != 404 {
		t.Fatal("revoked key edited", w.Code, w.Body)
	}
	var auditCount int
	if err := database.QueryRowContext(ctx, "SELECT count(*) FROM changes WHERE object_type='api_key' AND object_id=? AND action='revoke_key'", result.Key.ID).Scan(&auditCount); err != nil || auditCount != 1 {
		t.Fatal("missing revocation audit", auditCount, err)
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
	name = "批量批准"
	first, err := service.Propose(ctx, auth.Principal{UserID: userID, Login: "owner"}, "location:old", updated.Rev, catalog.Patch{Name: &name}, "批准", "test")
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Propose(ctx, auth.Principal{UserID: userID, Login: "owner"}, "location:new", 1, catalog.Patch{Name: &name}, "批准", "test")
	if err != nil {
		t.Fatal(err)
	}
	batch := call("POST", "/api/changes/batch", `{"ids":["`+first+`","`+second+`","change:missing"],"action":"approve"}`, csrf, cookies, "")
	if batch.Code != 200 || !strings.Contains(batch.Body.String(), `"ok":true`) || !strings.Contains(batch.Body.String(), `"ok":false`) {
		t.Fatal(batch.Code, batch.Body)
	}
	verified := call("POST", "/api/assets/verify-batch", `{"ids":["location:old","location:missing"],"reason":"现场核实"}`, csrf, cookies, "")
	if verified.Code != 200 || !strings.Contains(verified.Body.String(), `"ok":true`) || !strings.Contains(verified.Body.String(), `"ok":false`) {
		t.Fatal(verified.Code, verified.Body)
	}
	wrong := call("POST", "/api/auth/change-password", `{"old_password":"wrong","new_password":"new password 12345"}`, csrf, cookies, "")
	if wrong.Code != 422 || !strings.Contains(wrong.Body.String(), "old_password") {
		t.Fatal(wrong.Code, wrong.Body)
	}
	for _, tc := range []struct{ password, reason string }{
		{"short", "密码至少 8 个字符"},
		{"abcdef12", "密码须包含大写、小写、数字、符号中的至少三类"},
		{"Password1", "密码过于常见"},
		{"xxOwNeR1!", "密码不能包含用户名"},
	} {
		body, _ := json.Marshal(map[string]string{"old_password": "strong password 12345", "new_password": tc.password})
		result := call("POST", "/api/auth/change-password", string(body), csrf, cookies, "")
		if result.Code != 422 || !strings.Contains(result.Body.String(), "new_password") || !strings.Contains(result.Body.String(), tc.reason) {
			t.Fatal(result.Code, result.Body)
		}
	}
	changed := call("POST", "/api/auth/change-password", `{"old_password":"strong password 12345","new_password":"new password 12345"}`, csrf, cookies, "")
	if changed.Code != 200 {
		t.Fatal(changed.Code, changed.Body)
	}
	if w := call("GET", "/api/auth/session", "", "", cookies, ""); w.Code != 401 {
		t.Fatal("old session", w.Code)
	}
	login = call("POST", "/api/auth/login", `{"login":"owner","password":"new password 12345"}`, "", nil, "")
	if login.Code != 200 {
		t.Fatal(login.Code, login.Body)
	}
	cookies = login.Result().Cookies()
	for _, c := range cookies {
		if c.Name == "rollbook_csrf" {
			csrf = c.Value
		}
	}
	if w := call("POST", "/api/auth/logout-all", "", csrf, cookies, ""); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if w := call("GET", "/api/auth/session", "", "", cookies, ""); w.Code != 401 {
		t.Fatal("logout all", w.Code)
	}
}
