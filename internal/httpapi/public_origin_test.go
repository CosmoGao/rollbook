package httpapi

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

// 实际 TLS 入口与内层明文请求；公开 Host 固定为非标准端口。
func TestFeishuPublicOriginTLSProxy(t *testing.T) {
	const browserOrigin = "https://rollbook.example:5443"
	const callback = browserOrigin + "/api/auth/feishu/callback"
	const fakeSecret = "FAKE-SECRET-DO-NOT-EXPOSE"
	for _, tc := range []struct {
		name, backendHost, forwardedHost, forwardedPort string
		trusted, expectSuccess                          bool
	}{
		{"host_keeps_port", "rollbook.example:5443", "", "", true, true},
		{"trusted_forwarded_host", "rollbook.example", "rollbook.example:5443", "", true, true},
		{"trusted_forwarded_port", "rollbook.example", "rollbook.example", "5443", true, true},
		{"missing_public_port", "rollbook.example", "", "", true, false},
		{"untrusted_spoof", "rollbook.example", "rollbook.example:5443", "", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := testEngine(t)
			registry, err := NewProviderRegistry(context.Background(), e.Auth.DB, e.Auth.Pepper, ProviderConfig{}, map[string]bool{})
			if err != nil {
				t.Fatal(err)
			}
			e.ProviderRegistry = registry
			e.Auth.EnabledProvider = registry.Enabled
			if tc.trusted {
				e.TrustedProxyCIDRs = []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}
			}
			backend := e.Handler()
			proxy := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				r.TLS = nil
				r.Host = tc.backendHost
				r.Header.Del("X-Forwarded-Host")
				r.Header.Del("X-Forwarded-Port")
				r.Header.Del("X-Forwarded-Proto")
				r.Header.Set("X-Forwarded-Proto", "https")
				if tc.forwardedHost != "" {
					r.Header.Set("X-Forwarded-Host", tc.forwardedHost)
				}
				if tc.forwardedPort != "" {
					r.Header.Set("X-Forwarded-Port", tc.forwardedPort)
				}
				backend.ServeHTTP(w, r)
			}))
			defer proxy.Close()
			client := proxy.Client()
			client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
			var cookies []*http.Cookie
			csrf := ""
			call := func(method, path, body, origin, language string) (int, string) {
				req, err := http.NewRequest(method, proxy.URL+path, strings.NewReader(body))
				if err != nil {
					t.Fatal(err)
				}
				req.Host = "rollbook.example:5443"
				if origin != "" {
					req.Header.Set("Origin", origin)
				}
				if language != "" {
					req.Header.Set("X-Rollbook-Language", language)
				}
				if body != "" {
					req.Header.Set("Content-Type", "application/json")
				}
				if csrf != "" {
					req.Header.Set("X-CSRF-Token", csrf)
				}
				for _, cookie := range cookies {
					req.AddCookie(cookie)
				}
				resp, err := client.Do(req)
				if err != nil {
					t.Fatal(err)
				}
				defer resp.Body.Close()
				data, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(data), fakeSecret) {
					t.Fatal("secret in response")
				}
				if path == "/api/auth/login" && resp.StatusCode == 200 {
					cookies = resp.Cookies()
					for _, cookie := range cookies {
						if cookie.Name == "rollbook_csrf" {
							csrf = cookie.Value
						}
					}
				}
				return resp.StatusCode, string(data)
			}
			status, body := call("POST", "/api/auth/login", `{"login":"owner","password":"initial password 123"}`, browserOrigin, "en")
			if !tc.expectSuccess {
				if status != 403 || !strings.Contains(body, `"code":"forbidden"`) {
					t.Fatalf("browser host=%s backend Host=%s forwarded Host=%s port=%s Origin=%s: status=%d", "rollbook.example:5443", tc.backendHost, tc.forwardedHost, tc.forwardedPort, browserOrigin, status)
				}
				if tc.name == "missing_public_port" {
					status, _ = call("POST", "/api/auth/login", `{"login":"owner","password":"initial password 123"}`, "https://rollbook.example", "en")
					if status != 200 {
						t.Fatal("control login failed")
					}
					payload := `{"enabled":true,"config":{"client_id":"app","redirect_url":"` + callback + `"},"secret_action":"replace","secret":"` + fakeSecret + `"}`
					status, body = call("PUT", "/api/auth/settings/feishu", payload, browserOrigin, "en")
					if status != 403 || !strings.Contains(body, `"code":"public_origin_mismatch"`) {
						t.Fatal("origin mismatch was not identified")
					}
					status, body = call("PUT", "/api/auth/settings/feishu", payload, "https://rollbook.example", "en")
					if status != 422 || !strings.Contains(body, `"field":"redirect_url"`) {
						t.Fatal("callback mismatch was not localized")
					}
				}
				return
			}
			if status != 200 {
				t.Fatalf("login status=%d body=%s", status, body)
			}
			payload := `{"enabled":true,"config":{"client_id":"app","redirect_url":"` + callback + `"},"secret_action":"replace","secret":"` + fakeSecret + `"}`
			status, body = call("PUT", "/api/auth/settings/feishu", payload, browserOrigin, "en")
			if status != 200 {
				t.Fatalf("save status=%d", status)
			}
			status, body = call("GET", "/api/config", "", "", "en")
			if status != 200 || !strings.Contains(body, `"feishu":true`) {
				t.Fatal("provider not active")
			}
			status, _ = call("GET", "/api/auth/feishu/start", "", "", "en")
			if status != 302 {
				t.Fatalf("start status=%d", status)
			}
			status, body = call("GET", "/api/auth/feishu/callback", "", "", "en")
			if status != 303 {
				t.Fatalf("callback failure status=%d body=%s", status, body)
			}
		})
	}
}

func TestPublicOriginHeaderValidationAndFields(t *testing.T) {
	e := testEngine(t)
	e.TrustedProxyCIDRs = []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}
	for _, tc := range []struct {
		name    string
		headers http.Header
		valid   bool
	}{
		{"forwarded", http.Header{"Forwarded": {"host=rollbook.example:5443;proto=https"}}, true},
		{"client_spoof_conflict", http.Header{"X-Forwarded-Host": {"evil.example:5443"}, "Forwarded": {"host=rollbook.example:5443;proto=https"}}, false},
		{"multiple_host", http.Header{"X-Forwarded-Host": {"rollbook.example:5443", "evil.example"}, "X-Forwarded-Proto": {"https"}}, false},
		{"host_chain", http.Header{"X-Forwarded-Host": {"evil.example, rollbook.example:5443"}, "X-Forwarded-Proto": {"https"}}, false},
		{"port_conflict", http.Header{"X-Forwarded-Host": {"rollbook.example:5443"}, "X-Forwarded-Port": {"443"}, "X-Forwarded-Proto": {"https"}}, false},
		{"proto_conflict", http.Header{"X-Forwarded-Proto": {"https"}, "Forwarded": {"proto=http"}}, false},
		{"userinfo", http.Header{"X-Forwarded-Host": {"user@rollbook.example:5443"}, "X-Forwarded-Proto": {"https"}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.Host = "rollbook.example"
			r.RemoteAddr = "127.0.0.1:12345"
			r.Header = tc.headers
			origin, ok := e.publicOrigin(r)
			if ok != tc.valid || (ok && origin != "https://rollbook.example:5443") {
				t.Fatalf("origin=%q valid=%t", origin, ok)
			}
			r.RemoteAddr = "192.0.2.1:12345"
			origin, ok = e.publicOrigin(r)
			if !ok || origin != "http://rollbook.example" {
				t.Fatal("untrusted forwarding was used")
			}
		})
	}
}
