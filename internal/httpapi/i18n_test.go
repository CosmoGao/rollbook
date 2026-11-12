package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CosmoGao/rollbook/internal/auth"
	"github.com/CosmoGao/rollbook/internal/catalog"
	"github.com/CosmoGao/rollbook/internal/db"
	"github.com/CosmoGao/rollbook/internal/httpapi"
)

func TestEnglishAliasesAndLocalizedErrors(t *testing.T) {
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
	_, token, err := a.IssueKey(ctx, auth.Principal{UserID: userID, Login: "owner"}, "writer", auth.Write, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	h := (httpapi.Engine{Auth: a, Catalog: catalog.Service{Store: catalog.Store{DB: database}}}).Handler()
	call := func(method, path, body, lang string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("Accept-Language", lang)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	create := call("POST", "/api/assets", `{"object":{"id":"location:en","type":"location","name":"English site","status":"active","fields":{"kind":"site"}},"base_rev":0,"reason":"initial registration"}`, "en")
	if create.Code != 200 {
		t.Fatalf("create: %d %s", create.Code, create.Body.String())
	}
	var location catalog.Object
	if err := json.Unmarshal(create.Body.Bytes(), &location); err != nil {
		t.Fatal(err)
	}
	if location.Status != "在用" || location.Fields["kind"] != "站点" {
		t.Fatalf("non-canonical response: %+v", location)
	}
	host := call("POST", "/api/assets", `{"object":{"id":"host:en","type":"host","name":"English host","status":"active","fields":{"kind":"physical","location_id":"location:en"}},"base_rev":0,"reason":"initial registration"}`, "en")
	if host.Code != 200 {
		t.Fatalf("host: %d %s", host.Code, host.Body.String())
	}
	var created catalog.Object
	if err := json.Unmarshal(host.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Fields["kind"] != "物理机" {
		t.Fatal(created.Fields)
	}
	updated := call("PATCH", "/api/assets/host:en", `{"rev":1,"patch":{"status":"inactive","fields":{"kind":"virtual"}},"reason":"test update"}`, "en")
	if updated.Code != 200 {
		t.Fatalf("update: %d %s", updated.Code, updated.Body.String())
	}
	var changed catalog.Object
	if err := json.Unmarshal(updated.Body.Bytes(), &changed); err != nil {
		t.Fatal(err)
	}
	if changed.Status != "停用" || changed.Fields["kind"] != "虚拟机" {
		t.Fatalf("non-canonical update: %+v", changed)
	}
	for _, lang := range []string{"en", "zh-CN"} {
		bad := call("POST", "/api/assets", `{"object":{"id":"location:bad","type":"location","name":"bad","fields":{"kind":"site","unsupported":"x"}},"base_rev":0,"reason":"bad input"}`, lang)
		if bad.Code != 422 {
			t.Fatalf("%s: %d %s", lang, bad.Code, bad.Body.String())
		}
		var apiErr httpapi.APIError
		if err := json.Unmarshal(bad.Body.Bytes(), &apiErr); err != nil {
			t.Fatal(err)
		}
		if apiErr.Code != "validation" || len(apiErr.Fields) == 0 || apiErr.Fields[0].Field != "fields.unsupported" {
			t.Fatal(apiErr)
		}
		if lang == "en" && strings.Contains(apiErr.Fields[0].Reason, "对象") {
			t.Fatalf("untranslated reason: %+v", apiErr)
		}
		if lang == "zh-CN" && !strings.Contains(apiErr.Fields[0].Reason, "对象") {
			t.Fatalf("Chinese request changed: %+v", apiErr)
		}
	}
	// Old Chinese input remains valid.
	old := call("POST", "/api/assets", `{"object":{"id":"location:zh","type":"location","name":"中文站点","status":"在用","fields":{"kind":"站点"}},"base_rev":0,"reason":"登记"}`, "zh-CN")
	if old.Code != 200 {
		t.Fatalf("legacy: %d %s", old.Code, old.Body.String())
	}
}
