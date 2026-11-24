package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/CosmoGao/rollbook/internal/auth"
	"github.com/CosmoGao/rollbook/internal/backup"
)

func TestProviderSettingsEncryptionAndOwnership(t *testing.T) {
	e := testEngine(t)
	ctx := context.Background()
	pepper := e.Auth.Pepper
	p, err := NewProviderRegistry(ctx, e.Auth.DB, pepper, ProviderConfig{}, map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	config := map[string]string{"issuer": "https://issuer.example", "client_id": "client", "redirect_url": "https://rollbook.example/api/auth/oidc/callback"}
	if err = p.save(ctx, "oidc", providerUpdate{Enabled: true, Config: config, SecretAction: "replace", Secret: "fake-secret-DO-NOT-LEAK"}); err != nil {
		t.Fatal(err)
	}
	if !p.Enabled("oidc", "https://issuer.example") {
		t.Fatal("provider did not activate")
	}
	var encoded, raw string
	if err = e.Auth.DB.QueryRowContext(ctx, "SELECT secret,config FROM auth_provider_settings WHERE kind='oidc'").Scan(&encoded, &raw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(encoded, "fake-secret") || strings.Contains(raw, "fake-secret") || !strings.HasPrefix(encoded, "v1:") {
		t.Fatal("plaintext persisted")
	}
	items, err := p.list(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.Kind == "oidc" && (!item.SecretConfigured || strings.Contains(item.Config["issuer"], "fake-secret")) {
			t.Fatal(item)
		}
	}
	exported, err := (backup.Service{DB: e.Auth.DB}).Export(ctx, "full")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(exported), "fake-secret") || strings.Contains(string(exported), encoded) || strings.Contains(string(exported), "auth_provider_settings") {
		t.Fatal("credential in YAML")
	}
	if err = p.save(ctx, "oidc", providerUpdate{Enabled: false, Config: config, SecretAction: "keep"}); err != nil {
		t.Fatal(err)
	}
	if p.Enabled("oidc", "https://issuer.example") {
		t.Fatal("provider did not deactivate")
	}
	wrong, err := NewProviderRegistry(ctx, e.Auth.DB, []byte("abcdef0123456789abcdef0123456789"), ProviderConfig{}, map[string]bool{})
	if err != nil {
		t.Fatal(err)
	} // 已关闭的源可用原密钥恢复，也可覆盖凭据。
	if err = wrong.save(ctx, "oidc", providerUpdate{Enabled: true, Config: config, SecretAction: "keep"}); err != errCredential {
		t.Fatal("wrong pepper accepted", err)
	}
	if err = wrong.save(ctx, "oidc", providerUpdate{Enabled: true, Config: config, SecretAction: "replace", Secret: "new-fake-secret"}); err != nil {
		t.Fatal(err)
	}
	lost, err := NewProviderRegistry(ctx, e.Auth.DB, pepper, ProviderConfig{}, map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	if lost.Enabled("oidc", "https://issuer.example") {
		t.Fatal("undecryptable source activated")
	}
	lostItems, err := lost.list(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range lostItems {
		if item.Kind == "oidc" && (item.Enabled || item.Error == "") {
			t.Fatal("missing recovery warning", item)
		}
	}
	env := ProviderConfig{OIDC: OAuthProviderConfig{Issuer: "https://env.example", ClientID: "env", ClientSecret: "env-secret", RedirectURL: "https://rollbook.example/api/auth/oidc/callback"}}
	owned, err := NewProviderRegistry(ctx, e.Auth.DB, []byte("abcdef0123456789abcdef0123456789"), env, map[string]bool{"oidc": true})
	if err != nil {
		t.Fatal(err)
	}
	if !owned.Enabled("oidc", "https://env.example") {
		t.Fatal("environment lost priority")
	}
	items, err = owned.list(ctx)
	if err != nil {
		t.Fatal(err)
	}
	serialized, _ := json.Marshal(items)
	if strings.Contains(string(serialized), "env-secret") || strings.Contains(string(serialized), "new-fake-secret") {
		t.Fatal("secret in management response")
	}
	for _, item := range items {
		if item.Kind == "oidc" {
			if item.Source != "environment" || !item.Shadowed || !item.SecretConfigured || strings.Contains(item.Config["issuer"], "fake-secret") {
				t.Fatal(item)
			}
		}
	}
}

func TestProviderMutationCannotRemoveLastPath(t *testing.T) {
	e := testEngine(t)
	ctx := context.Background()
	p, err := NewProviderRegistry(ctx, e.Auth.DB, e.Auth.Pepper, ProviderConfig{}, map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	config := map[string]string{"client_id": "app", "redirect_url": "https://rollbook.example/api/auth/feishu/callback"}
	if err = p.save(ctx, "feishu", providerUpdate{Enabled: true, Config: config, SecretAction: "replace", Secret: "fake-secret"}); err != nil {
		t.Fatal(err)
	}
	if err = e.Auth.ManageBinding(ctx, "feishu", "app", "subject", "owner", true); err != nil {
		t.Fatal(err)
	}
	if err = e.Auth.SetLocalEnabled(ctx, false); err == nil {
		t.Fatal("disabled without real login")
	}
	if _, _, _, err = e.Auth.ExternalSession(ctx, "feishu", "app", "subject"); err != nil {
		t.Fatal(err)
	}
	e.Auth.EnabledProvider = p.Enabled
	if err = e.Auth.SetLocalEnabled(ctx, false); err != nil {
		t.Fatal(err)
	}
	if err = p.save(ctx, "feishu", providerUpdate{Enabled: false, Config: config, SecretAction: "keep"}); err == nil {
		t.Fatal("removed last path")
	}
	if !p.Enabled("feishu", "app") {
		t.Fatal("unsafe setting committed")
	}
}

func TestProviderSettingsBrowserBoundaryAndHotConfig(t *testing.T) {
	e := testEngine(t)
	ctx := context.Background()
	registry, err := NewProviderRegistry(ctx, e.Auth.DB, e.Auth.Pepper, ProviderConfig{}, map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	e.ProviderRegistry = registry
	e.Auth.EnabledProvider = registry.Enabled
	h := e.Handler()
	call := func(method, path, body, csrf, bearer string, cookies []*http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Host = "rollbook.example"
		r.Header.Set("Origin", "https://rollbook.example")
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
	if w := call("GET", "/api/auth/settings", "", "", "", nil); w.Code != 401 {
		t.Fatal(w.Code)
	}
	login := call("POST", "/api/auth/login", `{"login":"owner","password":"initial password 123"}`, "", "", nil)
	if login.Code != 200 {
		t.Fatal(login.Code, login.Body.String())
	}
	cookies := login.Result().Cookies()
	csrf := ""
	for _, c := range cookies {
		if c.Name == "rollbook_csrf" {
			csrf = c.Value
		}
	}
	if w := call("PUT", "/api/auth/settings/feishu", `{}`, "", "", cookies); w.Code != 403 {
		t.Fatal("CSRF accepted", w.Code)
	}
	var id string
	if err = e.Auth.DB.QueryRowContext(ctx, "SELECT id FROM users WHERE login='owner'").Scan(&id); err != nil {
		t.Fatal(err)
	}
	_, token, err := e.Auth.IssueKey(ctx, auth.Principal{UserID: id, Login: "owner"}, "writer", auth.Write, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if w := call("GET", "/api/auth/settings", "", "", token, nil); w.Code != 403 {
		t.Fatal("key accepted", w.Code)
	}
	if err = e.Auth.CreateExternalUser(ctx, "external"); err != nil {
		t.Fatal(err)
	}
	identities := call("GET", "/api/auth/settings/identities", "", "", "", cookies)
	if identities.Code != 200 {
		t.Fatal(identities.Code, identities.Body.String())
	}
	var listed struct {
		Users []struct {
			Login            string `json:"login"`
			HasLocalPassword bool   `json:"has_local_password"`
		} `json:"users"`
	}
	if err = json.Unmarshal(identities.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Users) != 2 || listed.Users[0].Login != "external" || listed.Users[0].HasLocalPassword || listed.Users[1].Login != "owner" || !listed.Users[1].HasLocalPassword {
		t.Fatal("incorrect password presence", listed.Users)
	}
	var hash string
	if err = e.Auth.DB.QueryRowContext(ctx, "SELECT password_hash FROM users WHERE login='owner'").Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(identities.Body.String(), hash) {
		t.Fatal("password hash exposed")
	}
	secret := "SENSITIVE-FAKE-SECRET-123"
	body := `{"enabled":true,"config":{"client_id":"app","redirect_url":"https://rollbook.example/api/auth/feishu/callback"},"secret_action":"replace","secret":"` + secret + `"}`
	if w := call("PUT", "/api/auth/settings/feishu", body, csrf, "", cookies); w.Code != 200 || strings.Contains(w.Body.String(), secret) {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, path := range []string{"/api/auth/settings", "/api/config"} {
		w := call("GET", path, "", "", "", cookies)
		if w.Code != 200 || strings.Contains(w.Body.String(), secret) {
			t.Fatal(path, w.Code, w.Body.String())
		}
	}
	if w := call("GET", "/api/config", "", "", "", nil); !strings.Contains(w.Body.String(), `"feishu":true`) {
		t.Fatal("not hot", w.Body.String())
	}
}

func TestOAuthPendingFailsAfterProviderChange(t *testing.T) {
	e := testEngine(t)
	ctx := context.Background()
	p, err := NewProviderRegistry(ctx, e.Auth.DB, e.Auth.Pepper, ProviderConfig{}, map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	e.ProviderRegistry = p
	config := map[string]string{"client_id": "app", "redirect_url": "https://rollbook.example/api/auth/feishu/callback"}
	if err = p.save(ctx, "feishu", providerUpdate{Enabled: true, Config: config, SecretAction: "replace", Secret: "old-fake-secret"}); err != nil {
		t.Fatal(err)
	}
	start := httptest.NewRecorder()
	e.Handler().ServeHTTP(start, reqWithCookie("/api/auth/feishu/start", nil))
	state := stateCookie(start)
	if state == nil || start.Code != 302 {
		t.Fatal(start.Code)
	}
	if err = p.save(ctx, "feishu", providerUpdate{Enabled: true, Config: config, SecretAction: "replace", Secret: "new-fake-secret"}); err != nil {
		t.Fatal(err)
	}
	callback := httptest.NewRecorder()
	e.Handler().ServeHTTP(callback, reqWithCookie("/api/auth/feishu/callback?state="+state.Value+"&code=fake", state))
	if callback.Code != 303 || !strings.Contains(callback.Header().Get("Location"), "auth_error") {
		t.Fatal("old flow accepted", callback.Code, callback.Body.String())
	}
}
