package main

import (
	"net/http"
	"net/http/httptest"
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
		if !strings.Contains(response.Body.String(), tc.contains) {
			t.Errorf("%s: body = %q", tc.path, response.Body.String())
		}
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
