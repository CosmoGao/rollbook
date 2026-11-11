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

func TestBackupPermissionAndApproval(t *testing.T) {
	ctx := context.Background()
	d, e := db.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer d.Close()
	a := auth.Service{DB: d, Pepper: []byte("0123456789abcdef0123456789abcdef")}
	id, e := a.BootstrapUser(ctx, "owner", "strong password 12345")
	if e != nil {
		t.Fatal(e)
	}
	read, readToken, e := a.IssueKey(ctx, auth.Principal{UserID: id, Login: "owner"}, "reader", auth.Read, nil, "")
	_ = read
	if e != nil {
		t.Fatal(e)
	}
	_, proposeToken, e := a.IssueKey(ctx, auth.Principal{UserID: id, Login: "owner"}, "proposer", auth.Propose, nil, "")
	if e != nil {
		t.Fatal(e)
	}
	store := catalog.Store{DB: d}
	if _, e = store.Create(ctx, catalog.Object{ID: "location:a", Type: "location", Name: "北京", Fields: map[string]any{"kind": "站点"}}, "owner"); e != nil {
		t.Fatal(e)
	}
	h := (httpapi.Engine{Auth: a, Catalog: catalog.Service{Store: store}}).Handler()
	call := func(method, path, body, token, csrf string, cookies []*http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Host = "rollbook.test"
		r.Header.Set("Origin", "https://rollbook.test")
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		if csrf != "" {
			r.Header.Set("X-CSRF-Token", csrf)
		}
		for _, c := range cookies {
			r.AddCookie(c)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	login := call("POST", "/api/auth/login", `{"login":"owner","password":"strong password 12345"}`, "", "", nil)
	if login.Code != 200 {
		t.Fatal(login.Code)
	}
	cookies := login.Result().Cookies()
	csrf := ""
	for _, c := range cookies {
		if c.Name == "rollbook_csrf" {
			csrf = c.Value
		}
	}
	catalogFile := call("GET", "/api/backup/export?kind=catalog", "", readToken, "", nil)
	if catalogFile.Code != 200 {
		t.Fatal(catalogFile.Code, catalogFile.Body)
	}
	if w := call("GET", "/api/backup/export?kind=full", "", readToken, "", nil); w.Code != 403 {
		t.Fatal("key downloaded full backup", w.Code)
	}
	if w := call("POST", "/api/backup/import", catalogFile.Body.String(), readToken, "", nil); w.Code != 403 {
		t.Fatal("read key submitted import", w.Code)
	}
	submitted := call("POST", "/api/backup/import", catalogFile.Body.String(), proposeToken, "", nil)
	if submitted.Code != 201 {
		t.Fatal(submitted.Code, submitted.Body)
	}
	var pending struct {
		ID string `json:"id"`
	}
	if e = json.Unmarshal(submitted.Body.Bytes(), &pending); e != nil {
		t.Fatal(e)
	}
	if w := call("POST", "/api/backup/imports/"+pending.ID+"/approve", "", proposeToken, "", nil); w.Code != 403 {
		t.Fatal("key approved import", w.Code)
	}
	if w := call("POST", "/api/backup/imports/"+pending.ID+"/approve", "", "", csrf, cookies); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if w := call("GET", "/api/auth/session", "", "", "", cookies); w.Code != 401 {
		t.Fatal("session survived restore", w.Code)
	}
	if w := call("GET", "/api/backup/export?kind=catalog", "", readToken, "", nil); w.Code != 200 {
		t.Fatal("key lost after catalog restore", w.Code)
	}
}
