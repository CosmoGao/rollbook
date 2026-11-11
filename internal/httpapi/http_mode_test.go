package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CosmoGao/rollbook/internal/auth"
	"github.com/CosmoGao/rollbook/internal/catalog"
	"github.com/CosmoGao/rollbook/internal/db"
	"github.com/CosmoGao/rollbook/internal/httpapi"
)

func TestHTTPCookiesAndCSRF(t *testing.T) {
	for _, allowInsecureHTTP := range []bool{false, true} {
		name := "default"
		if allowInsecureHTTP {
			name = "http mode"
		}
		t.Run(name, func(t *testing.T) {
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
			h := (httpapi.Engine{Auth: a, Catalog: catalog.Service{Store: catalog.Store{DB: database}}, AllowInsecureHTTP: allowInsecureHTTP}).Handler()
			call := func(method, path, body, csrf string, cookies []*http.Cookie) *httptest.ResponseRecorder {
				r := httptest.NewRequest(method, "http://rollbook.test"+path, strings.NewReader(body))
				r.Header.Set("Origin", "http://rollbook.test")
				if csrf != "" {
					r.Header.Set("X-CSRF-Token", csrf)
				}
				for _, cookie := range cookies {
					r.AddCookie(cookie)
				}
				w := httptest.NewRecorder()
				h.ServeHTTP(w, r)
				return w
			}
			config := call("GET", "/api/config", "", "", nil)
			if config.Code != 200 {
				t.Fatalf("config status = %d", config.Code)
			}
			var mode struct {
				AllowInsecureHTTP bool `json:"allow_insecure_http"`
			}
			if err := json.Unmarshal(config.Body.Bytes(), &mode); err != nil || mode.AllowInsecureHTTP != allowInsecureHTTP {
				t.Fatalf("config = %+v, err = %v", mode, err)
			}
			login := call("POST", "/api/auth/login", `{"login":"owner","password":"strong password 12345"}`, "", nil)
			if login.Code != 200 {
				t.Fatalf("login status = %d: %s", login.Code, login.Body.String())
			}
			cookies := login.Result().Cookies()
			csrf := checkModeCookies(t, cookies, !allowInsecureHTTP, false)
			if w := call("POST", "/api/keys", `{"name":"agent","level":"read"}`, "", cookies); w.Code != 403 {
				t.Fatalf("missing CSRF status = %d", w.Code)
			}
			if w := call("POST", "/api/keys", `{"name":"agent","level":"read"}`, csrf, cookies); w.Code != 201 {
				t.Fatalf("create key status = %d: %s", w.Code, w.Body.String())
			}
			logout := call("POST", "/api/auth/logout", "", csrf, cookies)
			if logout.Code != 200 {
				t.Fatalf("logout status = %d: %s", logout.Code, logout.Body.String())
			}
			checkModeCookies(t, logout.Result().Cookies(), !allowInsecureHTTP, true)
		})
	}
}

func checkModeCookies(t *testing.T, cookies []*http.Cookie, secure, clearing bool) string {
	t.Helper()
	var csrf string
	seen := map[string]bool{}
	for _, cookie := range cookies {
		if cookie.Name != "rollbook_session" && cookie.Name != "rollbook_csrf" {
			continue
		}
		seen[cookie.Name] = true
		if cookie.Secure != secure || cookie.SameSite != http.SameSiteStrictMode {
			t.Errorf("%s attributes: %+v", cookie.Name, cookie)
		}
		if cookie.HttpOnly != (cookie.Name == "rollbook_session") {
			t.Errorf("%s HttpOnly = %t", cookie.Name, cookie.HttpOnly)
		}
		if clearing && cookie.MaxAge != -1 || !clearing && cookie.MaxAge != 12*3600 {
			t.Errorf("%s MaxAge = %d", cookie.Name, cookie.MaxAge)
		}
		if cookie.Name == "rollbook_csrf" {
			csrf = cookie.Value
		}
	}
	if !seen["rollbook_session"] || !seen["rollbook_csrf"] {
		t.Fatalf("missing cookies: %+v", cookies)
	}
	return csrf
}
