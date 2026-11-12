package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CosmoGao/rollbook/internal/auth"
	"github.com/CosmoGao/rollbook/internal/catalog"
	"github.com/CosmoGao/rollbook/internal/db"
	"github.com/CosmoGao/rollbook/internal/httpapi"
)

func TestHTTPCookiesAndCSRF(t *testing.T) {
	cases := []struct {
		name, url, remote, xProto, forwarded string
		allowHTTP, trusted, secure           bool
	}{
		{"direct HTTP denied", "http://rollbook.test", "192.0.2.5:1234", "", "", false, false, false},
		{"direct HTTP allowed", "http://rollbook.test", "192.0.2.5:1234", "", "", true, false, false},
		{"direct TLS", "https://rollbook.test", "192.0.2.5:1234", "http", "", true, false, true},
		{"trusted proxy", "http://rollbook.test", "127.0.0.1:1234", "https", "", false, true, true},
		{"standard Forwarded", "http://rollbook.test", "127.0.0.1:1234", "", "for=192.0.2.5; proto=https", false, true, true},
		{"untrusted spoof", "http://rollbook.test", "192.0.2.5:1234", "https", "", false, true, false},
		{"conflicting headers", "http://rollbook.test", "127.0.0.1:1234", "https", "proto=http", true, true, false},
		{"ambiguous chain", "http://rollbook.test", "127.0.0.1:1234", "http, https", "", true, true, false},
		{"invalid value", "http://rollbook.test", "127.0.0.1:1234", "https;bad", "", true, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
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
			var proxies []netip.Prefix
			if tc.trusted {
				proxies = []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}
			}
			h := (httpapi.Engine{Auth: a, Catalog: catalog.Service{Store: catalog.Store{DB: database}}, AllowInsecureHTTP: tc.allowHTTP, TrustedProxyCIDRs: proxies}).Handler()
			call := func(method, path, body, csrf string, cookies []*http.Cookie) *httptest.ResponseRecorder {
				r := httptest.NewRequest(method, tc.url+path, strings.NewReader(body))
				r.RemoteAddr = tc.remote
				r.Header.Set("Origin", tc.url)
				if tc.xProto != "" {
					r.Header.Set("X-Forwarded-Proto", tc.xProto)
				}
				if tc.forwarded != "" {
					r.Header.Set("Forwarded", tc.forwarded)
				}
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
				HTTPRisk          bool `json:"http_risk"`
			}
			if err := json.Unmarshal(config.Body.Bytes(), &mode); err != nil || mode.AllowInsecureHTTP != tc.allowHTTP || mode.HTTPRisk == tc.secure {
				t.Fatalf("config = %+v, err = %v", mode, err)
			}
			login := call("POST", "/api/auth/login", `{"login":"owner","password":"strong password 12345"}`, "", nil)
			if !tc.secure && !tc.allowHTTP {
				if login.Code != 403 || len(login.Result().Cookies()) != 0 {
					t.Fatalf("HTTP login = %d, cookies = %+v", login.Code, login.Result().Cookies())
				}
				return
			}
			if login.Code != 200 {
				t.Fatalf("login status = %d: %s", login.Code, login.Body.String())
			}
			cookies := login.Result().Cookies()
			csrf := checkModeCookies(t, cookies, tc.secure, false)
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
			checkModeCookies(t, logout.Result().Cookies(), tc.secure, true)
			for _, action := range []struct{ path, body string }{
				{"/api/auth/logout-all", ""},
				{"/api/auth/change-password", `{"old_password":"strong password 12345","new_password":"new strong password 12345"}`},
			} {
				login = call("POST", "/api/auth/login", `{"login":"owner","password":"strong password 12345"}`, "", nil)
				if login.Code != 200 {
					t.Fatalf("repeat login status = %d: %s", login.Code, login.Body.String())
				}
				cookies = login.Result().Cookies()
				csrf = checkModeCookies(t, cookies, tc.secure, false)
				result := call("POST", action.path, action.body, csrf, cookies)
				if result.Code != 200 {
					t.Fatalf("%s status = %d: %s", action.path, result.Code, result.Body.String())
				}
				checkModeCookies(t, result.Result().Cookies(), tc.secure, true)
			}
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
