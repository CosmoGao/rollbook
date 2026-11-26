package httpapi

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/CosmoGao/rollbook/internal/auth"
	"github.com/CosmoGao/rollbook/internal/db"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}
func testEngine(t *testing.T) Engine {
	t.Helper()
	d, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	s := auth.Service{DB: d, Pepper: []byte("12345678901234567890123456789012")}
	if _, err = s.BootstrapUser(context.Background(), "owner", "initial password 123"); err != nil {
		t.Fatal(err)
	}
	return Engine{Auth: s, AllowInsecureHTTP: true}
}
func reqWithCookie(path string, c *http.Cookie) *http.Request {
	r := httptest.NewRequest("GET", path, nil)
	r.Host = "rollbook.example"
	r.TLS = &tls.ConnectionState{}
	if c != nil {
		r.AddCookie(c)
	}
	return r
}
func stateCookie(w *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range w.Result().Cookies() {
		if c.Name == "rollbook_auth_state" && c.Value != "" {
			return c
		}
	}
	return nil
}

func TestFeishuCallbackAndFailures(t *testing.T) {
	e := testEngine(t)
	e.Providers.Feishu = OAuthProviderConfig{ClientID: "cli_test", ClientSecret: "example-secret", RedirectURL: "https://rollbook.example/api/auth/feishu/callback"}
	e.Providers.HTTPClient = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/oauth/v3/token":
			return response(200, `{"code":0,"access_token":"user-token"}`), nil
		case "/open-apis/authen/v1/user_info":
			if r.Header.Get("Authorization") != "Bearer user-token" {
				t.Error("wrong bearer")
			}
			return response(200, `{"code":0,"data":{"open_id":"ou_subject","email":"owner@example.com"}}`), nil
		}
		return response(404, ""), nil
	})}
	if err := e.Auth.ManageBinding(context.Background(), "feishu", "cli_test", "ou_subject", "owner", true); err != nil {
		t.Fatal(err)
	}
	start := httptest.NewRecorder()
	e.Handler().ServeHTTP(start, reqWithCookie("/api/auth/feishu/start", nil))
	c := stateCookie(start)
	if c == nil || start.Code != 302 {
		t.Fatal(start.Code, start.Header())
	}
	bad := httptest.NewRecorder()
	e.Handler().ServeHTTP(bad, reqWithCookie("/api/auth/feishu/callback?state=wrong&code=good", c))
	if bad.Code != 303 {
		t.Fatal(bad.Code)
	}
	good := httptest.NewRecorder()
	e.Handler().ServeHTTP(good, reqWithCookie("/api/auth/feishu/callback?state="+url.QueryEscape(c.Value)+"&code=good", c))
	if good.Code != 303 {
		t.Fatal(good.Code)
	}
	found := false
	for _, cookie := range good.Result().Cookies() {
		if cookie.Name == "rollbook_session" && cookie.Value != "" {
			found = true
		}
	}
	if !found {
		t.Fatal("missing session")
	}
	replay := httptest.NewRecorder()
	e.Handler().ServeHTTP(replay, reqWithCookie("/api/auth/feishu/callback?state="+url.QueryEscape(c.Value)+"&code=good", c))
	for _, cookie := range replay.Result().Cookies() {
		if cookie.Name == "rollbook_session" && cookie.Value != "" {
			t.Fatal("replay created session")
		}
	}
	start = httptest.NewRecorder()
	e.Handler().ServeHTTP(start, reqWithCookie("/api/auth/feishu/start", nil))
	c = stateCookie(start)
	e.Providers.HTTPClient = &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) { return response(401, `{"code":20066}`), nil })}
	rejected := httptest.NewRecorder()
	e.Handler().ServeHTTP(rejected, reqWithCookie("/api/auth/feishu/callback?state="+url.QueryEscape(c.Value)+"&code=bad", c))
	for _, cookie := range rejected.Result().Cookies() {
		if cookie.Name == "rollbook_session" && cookie.Value != "" {
			t.Fatal("upstream error created session")
		}
	}
	e.Providers.Feishu.ClientID = ""
	disabled := httptest.NewRecorder()
	e.Handler().ServeHTTP(disabled, reqWithCookie("/api/auth/feishu/start", nil))
	if disabled.Code != 404 {
		t.Fatal(disabled.Code)
	}
}

func TestOIDCNoncePKCEAndBinding(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var issuer, nonce, challenge string
	wrongNonce := false
	wrongAudience := false
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			json.NewEncoder(w).Encode(map[string]string{"issuer": issuer, "authorization_endpoint": issuer + "/authorize", "token_endpoint": issuer + "/token", "jwks_uri": issuer + "/jwks"})
		case "/jwks":
			json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]string{"kty": "RSA", "kid": "test", "alg": "RS256", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": "AQAB"}}})
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			r.ParseForm()
			sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
			if base64.RawURLEncoding.EncodeToString(sum[:]) != challenge {
				t.Error("PKCE mismatch")
			}
			n := nonce
			if wrongNonce {
				n = "wrong"
			}
			audience := "client"
			if wrongAudience {
				audience = "other-client"
			}
			claims := map[string]any{"iss": issuer, "sub": "stable-sub", "aud": audience, "exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(), "nonce": n}
			payload, _ := json.Marshal(claims)
			header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","kid":"test"}`))
			body := header + "." + base64.RawURLEncoding.EncodeToString(payload)
			digest := sha256.Sum256([]byte(body))
			sig, _ := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
			json.NewEncoder(w).Encode(map[string]any{"access_token": "token", "token_type": "Bearer", "id_token": body + "." + base64.RawURLEncoding.EncodeToString(sig)})
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	issuer = upstream.URL
	e := testEngine(t)
	e.Providers.OIDC = OAuthProviderConfig{Issuer: issuer, ClientID: "client", ClientSecret: "secret", RedirectURL: "https://rollbook.example/api/auth/oidc/callback"}
	e.Providers.HTTPClient = upstream.Client()
	if err = e.Auth.ManageBinding(context.Background(), "oidc", issuer, "stable-sub", "owner", true); err != nil {
		t.Fatal(err)
	}
	start := func() *http.Cookie {
		w := httptest.NewRecorder()
		e.Handler().ServeHTTP(w, reqWithCookie("/api/auth/oidc/start", nil))
		if w.Code != 302 {
			t.Fatal(w.Code, w.Body.String())
		}
		u, _ := url.Parse(w.Header().Get("Location"))
		nonce = u.Query().Get("nonce")
		challenge = u.Query().Get("code_challenge")
		if nonce == "" || challenge == "" || u.Query().Get("code_challenge_method") != "S256" {
			t.Fatal(u)
		}
		return stateCookie(w)
	}
	c := start()
	wrongNonce = true
	w := httptest.NewRecorder()
	e.Handler().ServeHTTP(w, reqWithCookie("/api/auth/oidc/callback?state="+c.Value+"&code=ok", c))
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "rollbook_session" && cookie.Value != "" {
			t.Fatal("nonce accepted")
		}
	}
	c = start()
	wrongNonce = false
	w = httptest.NewRecorder()
	e.Handler().ServeHTTP(w, reqWithCookie("/api/auth/oidc/callback?state="+c.Value+"&code=ok", c))
	ok := false
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "rollbook_session" && cookie.Value != "" {
			ok = true
		}
	}
	if !ok {
		t.Fatal("OIDC login failed", w.Code, w.Header())
	}
	c = start()
	wrongAudience = true
	w = httptest.NewRecorder()
	e.Handler().ServeHTTP(w, reqWithCookie("/api/auth/oidc/callback?state="+c.Value+"&code=ok", c))
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "rollbook_session" && cookie.Value != "" {
			t.Fatal("wrong audience accepted")
		}
	}
}
func TestLDAPRejectsInvalidConfigAndPlaintext(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		conn, e := listener.Accept()
		if e == nil {
			conn.Close()
		}
	}()
	c := ProviderConfig{LDAP: LDAPConfig{URL: "ldap://" + listener.Addr().String(), BaseDN: "dc=example,dc=com", BindDN: "cn=reader", BindPassword: "secret", Filter: "(uid={login})", SubjectAttribute: "entryUUID", Instance: "test"}}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	e := Engine{Providers: c}
	if _, err := e.ldapSubject("*)(uid=*)", "password"); err == nil {
		t.Fatal("connection without verified TLS succeeded")
	}
	filter, err := safeLDAPFilter("(&(uid={login})(active=1))", "*)(uid=*)")
	if err != nil || strings.Contains(filter, "*)(uid=*)") {
		t.Fatal("filter not escaped", filter, err)
	}
	c.LDAP.URL = "http://example.com"
	if err := c.Validate(); err == nil {
		t.Fatal("HTTP LDAP config accepted")
	}
}

func TestSessionIdentityContract(t *testing.T) {
	e := testEngine(t)
	token, _, _, err := e.Auth.Login(context.Background(), "owner", "initial password 123")
	if err != nil {
		t.Fatal(err)
	}
	r := reqWithCookie("/api/auth/session", &http.Cookie{Name: "rollbook_session", Value: token})
	w := httptest.NewRecorder()
	e.Handler().ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var got struct {
		Login        string `json:"login"`
		IdentityType string `json:"identity_type"`
		CanChange    bool   `json:"can_change_password"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Login != "owner" || got.IdentityType != "local" || !got.CanChange {
		t.Fatal(got)
	}
}

func TestFeishuBindPreservesLocalSessionAndPort(t *testing.T) {
	e := testEngine(t)
	ctx := context.Background()
	e.Providers.Feishu = OAuthProviderConfig{ClientID: "cli_test", ClientSecret: "test-only", RedirectURL: "https://rollbook.example:5443/api/auth/feishu/callback"}
	e.Providers.HTTPClient = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/oauth/v3/token" {
			return response(200, `{"code":0,"access_token":"test-token"}`), nil
		}
		if r.URL.Path == "/open-apis/authen/v1/user_info" {
			return response(200, `{"code":0,"data":{"open_id":"ou_bind","email":"owner@example.com"}}`), nil
		}
		return response(404, ""), nil
	})}
	token, _, p, err := e.Auth.Login(ctx, "owner", "initial password 123")
	if err != nil {
		t.Fatal(err)
	}
	localCookie := &http.Cookie{Name: "rollbook_session", Value: token}
	makeRequest := func(path string, state *http.Cookie) *http.Request {
		r := reqWithCookie(path, localCookie)
		r.Host = "rollbook.example:5443"
		if state != nil {
			r.AddCookie(state)
		}
		return r
	}
	start := httptest.NewRecorder()
	e.Handler().ServeHTTP(start, makeRequest("/api/auth/feishu/bind/start", nil))
	state := stateCookie(start)
	if start.Code != 302 || state == nil {
		t.Fatalf("start: %d", start.Code)
	}
	if loc := start.Header().Get("Location"); !strings.Contains(loc, "rollbook.example%3A5443") {
		t.Fatalf("redirect port missing: %s", loc)
	}
	callback := httptest.NewRecorder()
	e.Handler().ServeHTTP(callback, makeRequest("/api/auth/feishu/callback?state="+url.QueryEscape(state.Value)+"&code=good", state))
	if callback.Code != 303 || callback.Header().Get("Location") != "/settings/personal?bind=ok" {
		t.Fatalf("callback: %d %s", callback.Code, callback.Header().Get("Location"))
	}
	if _, err = e.Auth.Session(ctx, token, "", false); err != nil {
		t.Fatalf("original session revoked: %v", err)
	}
	bound, err := e.Auth.FeishuBinding(ctx, p.UserID, "cli_test")
	if err != nil || !bound {
		t.Fatalf("binding absent: %v", err)
	}
	replay := httptest.NewRecorder()
	e.Handler().ServeHTTP(replay, makeRequest("/api/auth/feishu/callback?state="+url.QueryEscape(state.Value)+"&code=good", state))
	if replay.Header().Get("Location") == "/settings/personal?bind=ok" {
		t.Fatal("replay accepted")
	}
}

func TestFeishuEmailCallbackScenarios(t *testing.T) {
	e := testEngine(t)
	ctx := context.Background()
	e.Providers.Feishu = OAuthProviderConfig{ClientID: "cli_test", ClientSecret: "test-only", RedirectURL: "https://rollbook.example/api/auth/feishu/callback"}
	var subject, email string
	e.Providers.HTTPClient = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/oauth/v3/token" {
			return response(200, `{"code":0,"access_token":"test-token"}`), nil
		}
		if r.URL.Path == "/open-apis/authen/v1/user_info" {
			data, _ := json.Marshal(map[string]any{"code": 0, "data": map[string]string{"open_id": subject, "email": email}})
			return response(200, string(data)), nil
		}
		return response(404, ""), nil
	})}
	ownerID, err := func() (string, error) {
		var id string
		err := e.Auth.DB.QueryRow("SELECT id FROM users WHERE login='owner'").Scan(&id)
		return id, err
	}()
	if err != nil {
		t.Fatal(err)
	}
	if err = e.Auth.SetEmail(ctx, ownerID, "owner@example.com"); err != nil {
		t.Fatal(err)
	}
	if err = e.Auth.CreateExternalUser(ctx, "other"); err != nil {
		t.Fatal(err)
	}
	var otherID string
	if err = e.Auth.DB.QueryRow("SELECT id FROM users WHERE login='other'").Scan(&otherID); err != nil {
		t.Fatal(err)
	}
	if err = e.Auth.SetEmail(ctx, otherID, "other@example.com"); err != nil {
		t.Fatal(err)
	}
	callback := func(sub, mail string) *httptest.ResponseRecorder {
		subject, email = sub, mail
		start := httptest.NewRecorder()
		e.Handler().ServeHTTP(start, reqWithCookie("/api/auth/feishu/start", nil))
		state := stateCookie(start)
		if state == nil {
			t.Fatal("missing state")
		}
		result := httptest.NewRecorder()
		e.Handler().ServeHTTP(result, reqWithCookie("/api/auth/feishu/callback?state="+url.QueryEscape(state.Value)+"&code=test", state))
		return result
	}
	if got := callback("ou_unmatched", "nobody@example.com").Header().Get("Location"); got != "/?auth_error=unmatched" {
		t.Fatal(got)
	}
	if got := callback("ou_auto", " OWNER@EXAMPLE.COM ").Header().Get("Location"); got != "/" {
		t.Fatal(got)
	}
	if got := callback("ou_auto", "owner@example.com").Header().Get("Location"); got != "/" {
		t.Fatal(got)
	}
	if got := callback("ou_auto", "other@example.com").Header().Get("Location"); got != "/?auth_error=conflict" {
		t.Fatal(got)
	}
	if err = e.Auth.ManageBinding(ctx, "feishu", "cli_test", "ou_disabled", "owner", false); err != nil {
		t.Fatal(err)
	}
	if got := callback("ou_disabled", "owner@example.com").Header().Get("Location"); got != "/?auth_error=disabled" {
		t.Fatal(got)
	}
	if err = e.Auth.SetUserEnabled(ctx, "owner", false); err != nil {
		t.Fatal(err)
	}
	if got := callback("ou_auto", "owner@example.com").Header().Get("Location"); got == "/" {
		t.Fatal("disabled user signed in")
	}
}
