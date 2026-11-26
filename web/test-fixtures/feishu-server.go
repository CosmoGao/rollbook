package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/CosmoGao/rollbook/internal/auth"
	"github.com/CosmoGao/rollbook/internal/catalog"
	"github.com/CosmoGao/rollbook/internal/db"
	"github.com/CosmoGao/rollbook/internal/httpapi"
)

type fakeFeishu struct{}

func (fakeFeishu) RoundTrip(r *http.Request) (*http.Response, error) {
	var body string
	if strings.HasSuffix(r.URL.Path, "/token") {
		raw, _ := io.ReadAll(r.Body)
		code, _ := url.ParseQuery(string(raw))
		data, _ := json.Marshal(map[string]any{"code": 0, "access_token": code.Get("code")})
		body = string(data)
	} else if strings.HasSuffix(r.URL.Path, "/user_info") {
		value := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		parts := strings.SplitN(value, "|", 2)
		email := ""
		if len(parts) > 1 {
			email = parts[1]
		}
		data, _ := json.Marshal(map[string]any{"code": 0, "data": map[string]string{"open_id": parts[0], "email": email}})
		body = string(data)
	} else {
		return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	}
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
}
func main() {
	root, err := os.MkdirTemp("", "rollbook-feishu-browser-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(root)
	d, err := db.Open(context.Background(), filepath.Join(root, "test.db"))
	if err != nil {
		panic(err)
	}
	defer d.Close()
	a := auth.Service{DB: d, Pepper: []byte("test-only-123456789012345678901234567890")}
	ctx := context.Background()
	owner, err := a.BootstrapUser(ctx, "owner", "initial password 123")
	if err != nil {
		panic(err)
	}
	if err = a.SetEmail(ctx, owner, "owner@example.com"); err != nil {
		panic(err)
	}
	if err = a.CreateExternalUser(ctx, "other"); err != nil {
		panic(err)
	}
	if err = a.ManageBinding(ctx, "feishu", "cli_test", "bind_other", "other", true); err != nil {
		panic(err)
	}
	if err = a.ManageBinding(ctx, "feishu", "cli_test", "conflict", "other", true); err != nil {
		panic(err)
	}
	if err = a.ManageBinding(ctx, "feishu", "cli_test", "disabled", "other", false); err != nil {
		panic(err)
	}
	var engine httpapi.Engine
	engine.Auth = a
	engine.Catalog = catalog.Service{Store: catalog.Store{DB: d}}
	engine.Providers.HTTPClient = &http.Client{Transport: fakeFeishu{}}
	server := httptest.NewUnstartedServer(nil)
	engine.Providers.Feishu = httpapi.OAuthProviderConfig{ClientID: "cli_test", ClientSecret: "test-only", RedirectURL: "https://" + server.Listener.Addr().String() + "/api/auth/feishu/callback"}
	api := engine.Handler()
	static := http.FileServer(http.Dir("web/dist"))
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fixture/reset-owner" && r.Method == http.MethodPost {
			d.Exec("DELETE FROM identity_bindings WHERE provider='feishu' AND subject='new-bind'")
			w.WriteHeader(204)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			api.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/" || strings.HasPrefix(r.URL.Path, "/settings/") {
			data, _ := os.ReadFile("web/dist/index.html")
			w.Header().Set("Content-Type", "text/html")
			w.Write(data)
			return
		}
		static.ServeHTTP(w, r)
	})
	server.StartTLS()
	defer server.Close()
	fmt.Println(server.URL)
	var wait string
	fmt.Scanln(&wait)
}
