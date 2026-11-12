package main

import (
	"bytes"
	"github.com/CosmoGao/rollbook/internal/buildinfo"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHTTP(t *testing.T) {
	for _, tc := range []struct {
		path     string
		status   int
		contains string
	}{
		{"/", 200, "Rollbook 清册"},
		{"/healthz", 200, "ok"},
		{"/unknown", 404, ""},
		{"/api/keys", 401, ""},
	} {
		response := httptest.NewRecorder()
		handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if response.Code != tc.status {
			t.Errorf("%s: status = %d", tc.path, response.Code)
		}
		for _, header := range []string{"Content-Security-Policy", "X-Frame-Options", "X-Content-Type-Options", "Referrer-Policy"} {
			if response.Header().Get(header) == "" {
				t.Errorf("%s: missing %s", tc.path, header)
			}
		}
		if !strings.Contains(response.Body.String(), tc.contains) {
			t.Errorf("%s: body = %q", tc.path, response.Body.String())
		}
	}
}

func TestManifestAndIcons(t *testing.T) {
	for _, tc := range []struct{ path, contentType string }{
		{"/manifest.webmanifest", "application/manifest+json; charset=utf-8"},
		{"/icon-192.png", "image/png"},
		{"/icon-512.png", "image/png"},
		{"/icon.svg", "image/svg+xml"},
	} {
		response := httptest.NewRecorder()
		handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if response.Code != http.StatusOK || response.Header().Get("Content-Type") != tc.contentType || response.Body.Len() == 0 {
			t.Errorf("%s: status=%d type=%q bytes=%d", tc.path, response.Code, response.Header().Get("Content-Type"), response.Body.Len())
		}
	}
}

func TestBuildIdentity(t *testing.T) {
	oldVersion, oldCommit := buildinfo.Version, buildinfo.Commit
	buildinfo.Version, buildinfo.Commit = "v0.3.0", "123456789abcdef"
	t.Cleanup(func() { buildinfo.Version, buildinfo.Commit = oldVersion, oldCommit })
	response := httptest.NewRecorder()
	handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/version", nil))
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"version":"v0.3.0"`) || !strings.Contains(response.Body.String(), `"commit":"1234567"`) {
		t.Fatalf("版本接口: %d %s", response.Code, response.Body.String())
	}
	var output bytes.Buffer
	if err := (cliRunner{out: &output}).run([]string{"version"}); err != nil {
		t.Fatal(err)
	}
	if output.String() != "rollbook v0.3.0 (1234567)\n" {
		t.Fatalf("CLI 版本: %q", output.String())
	}
}

func TestTrustedProxyConfig(t *testing.T) {
	t.Setenv("ROLLBOOK_TRUSTED_PROXY_CIDRS", "127.0.0.1/32, 192.0.2.0/24")
	got, err := trustedProxyCIDRs()
	if err != nil || len(got) != 2 {
		t.Fatal(got, err)
	}
	t.Setenv("ROLLBOOK_TRUSTED_PROXY_CIDRS", "not-a-cidr")
	if _, err = trustedProxyCIDRs(); err == nil {
		t.Fatal("invalid proxy CIDR accepted")
	}
}

func TestInsecureHTTPEnabled(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  bool
	}{
		{"", false}, {"false", false}, {"TRUE", false}, {"1", false}, {"true", true},
	} {
		t.Setenv("ROLLBOOK_ALLOW_INSECURE_HTTP", tc.value)
		if got := insecureHTTPEnabled(); got != tc.want {
			t.Errorf("value %q: got %t, want %t", tc.value, got, tc.want)
		}
	}
}

func TestAuthPepper(t *testing.T) {
	for _, name := range []string{"ROLLBOOK_AUTH_PEPPER", "ROLLBOOK_AUTH_PEPPER_FILE"} {
		value, present := os.LookupEnv(name)
		t.Cleanup(func() {
			if present {
				os.Setenv(name, value)
			} else {
				os.Unsetenv(name)
			}
		})
		os.Unsetenv(name)
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "pepper")
	content := strings.Repeat("x", 32) + "\n"
	if err := os.WriteFile(file, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	os.Setenv("ROLLBOOK_AUTH_PEPPER_FILE", file)
	if got, err := authPepper(); err != nil || string(got) != content {
		t.Fatalf("file pepper = %q, %v", got, err)
	}
	os.Setenv("ROLLBOOK_AUTH_PEPPER", strings.Repeat("y", 32))
	if _, err := authPepper(); err == nil || !strings.Contains(err.Error(), "cannot both be set") {
		t.Fatalf("both set: %v", err)
	}
	os.Unsetenv("ROLLBOOK_AUTH_PEPPER")
	if err := os.WriteFile(file, []byte(strings.Repeat("z", 31)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := authPepper(); err == nil || !strings.Contains(err.Error(), "at least 32 bytes") {
		t.Fatalf("short file: %v", err)
	}
	os.Setenv("ROLLBOOK_AUTH_PEPPER_FILE", filepath.Join(dir, "missing"))
	if _, err := authPepper(); err == nil || !strings.Contains(err.Error(), "read ROLLBOOK_AUTH_PEPPER_FILE") {
		t.Fatalf("missing file: %v", err)
	}
	os.Unsetenv("ROLLBOOK_AUTH_PEPPER_FILE")
	os.Setenv("ROLLBOOK_AUTH_PEPPER", strings.Repeat("p", 32))
	if got, err := authPepper(); err != nil || string(got) != strings.Repeat("p", 32) {
		t.Fatalf("env pepper = %q, %v", got, err)
	}
}

func TestInvalidPepperDoesNotCreateDatabase(t *testing.T) {
	value, present := os.LookupEnv("ROLLBOOK_AUTH_PEPPER")
	t.Cleanup(func() {
		if present {
			os.Setenv("ROLLBOOK_AUTH_PEPPER", value)
		} else {
			os.Unsetenv("ROLLBOOK_AUTH_PEPPER")
		}
	})
	t.Setenv("ROLLBOOK_DB_PATH", filepath.Join(t.TempDir(), "data", "rollbook.db"))
	t.Setenv("ROLLBOOK_AUTH_PEPPER_FILE", filepath.Join(t.TempDir(), "missing"))
	os.Unsetenv("ROLLBOOK_AUTH_PEPPER")
	if err := bootstrapUser([]string{"owner"}); err == nil {
		t.Fatal("bootstrap accepted missing pepper")
	}
	if _, err := os.Stat(os.Getenv("ROLLBOOK_DB_PATH")); !os.IsNotExist(err) {
		t.Fatalf("database created before pepper validation: %v", err)
	}
}

func TestBackupPath(t *testing.T) {
	t.Setenv("ROLLBOOK_BACKUP_DIR", "")
	if got := backupPath("/data/rollbook.db"); got != "/data/rollbook-latest.yaml" {
		t.Fatal(got)
	}
	t.Setenv("ROLLBOOK_BACKUP_DIR", "/backups")
	if got := backupPath("/data/rollbook.db"); got != "/backups/rollbook-latest.yaml" {
		t.Fatal(got)
	}
}
