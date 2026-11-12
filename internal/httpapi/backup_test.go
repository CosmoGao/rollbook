package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/CosmoGao/rollbook/internal/auth"
	"github.com/CosmoGao/rollbook/internal/backup"
	"github.com/CosmoGao/rollbook/internal/catalog"
	"github.com/CosmoGao/rollbook/internal/db"
	"github.com/CosmoGao/rollbook/internal/httpapi"
)

func TestBackupPermissionAndApproval(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	d, e := db.Open(ctx, dbPath)
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
	h := (httpapi.Engine{Auth: a, Catalog: catalog.Service{Store: store}, AllowInsecureHTTP: true}).Handler()
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
	if w := call("POST", "/api/backup/import", catalogFile.Body.String(), proposeToken, "", nil); w.Code != 403 {
		t.Fatal("proposal key submitted restore", w.Code)
	}
	merge := "schema_version: 1\nkind: merge\ntables:\n  objects:\n    - id: location:b\n      type: location\n      name: 上海\n  locations:\n    - id: location:b\n      kind: 站点\n"
	if w := call("POST", "/api/backup/import", merge, readToken, "", nil); w.Code != 403 {
		t.Fatal("read key submitted merge", w.Code)
	}
	submitted := call("POST", "/api/backup/import", merge, proposeToken, "", nil)
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
	if w := call("GET", "/api/auth/session", "", "", "", cookies); w.Code != 200 {
		t.Fatal("merge invalidated session", w.Code)
	}
	if w := call("GET", "/api/backup/csv/template?type=host", "", readToken, "", nil); w.Code != 200 || !strings.Contains(w.Body.String(), "location_id") {
		t.Fatal("CSV template", w.Code, w.Body)
	}
	badCSV := "id,name,kind,location_id\nhost:bad,错误主机,物理机,location:missing\n"
	if w := call("POST", "/api/backup/csv/import?type=host", badCSV, proposeToken, "", nil); w.Code != 422 || !strings.Contains(w.Body.String(), "第 2 行 location_id 列") {
		t.Fatal("CSV relation error", w.Code, w.Body)
	}
	goodCSV := "id,name,kind,location_id\nhost:csv,CSV主机,物理机,location:a\n"
	if w := call("POST", "/api/backup/csv/import?type=host", goodCSV, readToken, "", nil); w.Code != 403 {
		t.Fatal("read key submitted CSV", w.Code)
	}
	if w := call("POST", "/api/backup/csv/import?type=host", goodCSV, proposeToken, "", nil); w.Code != 201 {
		t.Fatal("CSV submit", w.Code, w.Body)
	} else if e = json.Unmarshal(w.Body.Bytes(), &pending); e != nil {
		t.Fatal(e)
	}
	var csvCount int
	d.QueryRowContext(ctx, "SELECT count(*) FROM hosts WHERE id='host:csv'").Scan(&csvCount)
	if csvCount != 0 {
		t.Fatal("CSV bypassed approval")
	}
	if w := call("POST", "/api/backup/imports/"+pending.ID+"/approve", "", "", csrf, cookies); w.Code != 200 {
		t.Fatal("CSV approval", w.Code, w.Body)
	}
	d.QueryRowContext(ctx, "SELECT count(*) FROM search_documents WHERE object_id='host:csv'").Scan(&csvCount)
	if csvCount != 1 {
		t.Fatal("CSV search index missing")
	}
	managedData, e := (backup.Service{DB: d}).Export(ctx, "full")
	if e != nil {
		t.Fatal(e)
	}
	if e = backup.SaveManaged(filepath.Dir(dbPath), managedData, time.Now(), 7, 4, 7*24*time.Hour); e != nil {
		t.Fatal(e)
	}
	managedFiles, e := backup.ListManaged(filepath.Dir(dbPath))
	if e != nil || len(managedFiles) != 2 {
		t.Fatal(managedFiles, e)
	}
	filePath := "/api/backup/files/" + managedFiles[0].Name
	if w := call("GET", "/api/backup/files", "", readToken, "", nil); w.Code != 403 {
		t.Fatal("key listed backups", w.Code)
	}
	if w := call("GET", filePath+"/download", "", readToken, "", nil); w.Code != 403 {
		t.Fatal("key downloaded managed backup", w.Code)
	}
	if w := call("POST", filePath+"/restore", "", proposeToken, "", nil); w.Code != 403 {
		t.Fatal("key submitted managed restore", w.Code)
	}
	if w := call("GET", "/api/backup/files/..%2Fsecret/download", "", "", "", cookies); w.Code == 200 {
		t.Fatal("traversal downloaded")
	}
	if w := call("GET", filePath+"/download", "", "", "", cookies); w.Code != 200 || w.Body.String() != string(managedData) {
		t.Fatal("managed download", w.Code)
	}
	managed := call("POST", filePath+"/restore", "", "", csrf, cookies)
	if managed.Code != 201 {
		t.Fatal("managed restore submit", managed.Code, managed.Body)
	}
	if e = json.Unmarshal(managed.Body.Bytes(), &pending); e != nil {
		t.Fatal(e)
	}
	if _, e = store.Create(ctx, catalog.Object{ID: "location:c", Type: "location", Name: "广州", Fields: map[string]any{"kind": "站点"}}, "owner"); e != nil {
		t.Fatal(e)
	}
	if w := call("POST", "/api/backup/imports/"+pending.ID+"/approve", "", "", csrf, cookies); w.Code != 409 {
		t.Fatal("stale preview accepted", w.Code, w.Body)
	}
	restore := call("POST", "/api/backup/import", catalogFile.Body.String(), "", csrf, cookies)
	if restore.Code != 201 {
		t.Fatal("user restore submit", restore.Code, restore.Body)
	}
	if e = json.Unmarshal(restore.Body.Bytes(), &pending); e != nil {
		t.Fatal(e)
	}
	if w := call("POST", "/api/backup/imports/"+pending.ID+"/approve", "", "", csrf, cookies); w.Code != 200 {
		t.Fatal("restore approve", w.Code, w.Body)
	}
	files, e := filepath.Glob(filepath.Join(filepath.Dir(dbPath), "before-restore-*.yaml"))
	if e != nil || len(files) != 1 {
		t.Fatal("backup before restore", files, e)
	}
	if w := call("GET", "/api/auth/session", "", "", "", cookies); w.Code != 401 {
		t.Fatal("session survived restore", w.Code)
	}
	if w := call("GET", "/api/backup/export?kind=catalog", "", readToken, "", nil); w.Code != 200 {
		t.Fatal("key lost after catalog restore", w.Code)
	}
}
