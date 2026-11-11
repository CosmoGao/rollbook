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

func TestHostMoveOutsideNewSiteReturnsFieldError(t *testing.T) {
	ctx := context.Background()
	database, err := db.Open(ctx, filepath.Join(t.TempDir(), "host-move.db"))
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
	store := catalog.Store{DB: database}
	for _, item := range []catalog.Object{
		{ID: "location:old", Type: "location", Name: "旧站点", Fields: map[string]any{"kind": "站点"}},
		{ID: "location:new", Type: "location", Name: "新站点", Fields: map[string]any{"kind": "站点"}},
		{ID: "network:old", Type: "network", Name: "旧网段", Fields: map[string]any{"cidr": "10.1.0.0/24", "location_id": "location:old"}},
		{ID: "network:new", Type: "network", Name: "新网段", Fields: map[string]any{"cidr": "10.2.0.0/24", "location_id": "location:new"}},
		{ID: "host:one", Type: "host", Name: "主机", Fields: map[string]any{"kind": "物理机", "location_id": "location:old"}},
		{ID: "ip:one", Type: "ip", Name: "10.1.0.5", Fields: map[string]any{"address": "10.1.0.5", "host_id": "host:one"}},
	} {
		if _, err := store.Create(ctx, item, "owner"); err != nil {
			t.Fatalf("创建 %s: %v", item.ID, err)
		}
	}
	h := (httpapi.Engine{Auth: a, Catalog: catalog.Service{Store: store}}).Handler()
	r := httptest.NewRequest("PATCH", "/api/assets/host:one", strings.NewReader(`{"rev":1,"patch":{"fields":{"location_id":"location:new"}},"reason":"搬站点"}`))
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 422 {
		t.Fatalf("HTTP 状态=%d，响应=%s", w.Code, w.Body.String())
	}
	var response httpapi.APIError
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Code != "validation" || len(response.Fields) != 1 || response.Fields[0].Field != "fields.location_id" || !strings.Contains(response.Fields[0].Reason, "10.1.0.5") || !strings.Contains(response.Fields[0].Reason, "location:new") {
		t.Fatalf("字段错误未指出 IP 和新站点: %+v", response)
	}
	got, err := store.Get(ctx, "host:one")
	if err != nil || got.Rev != 1 || got.Fields["location_id"] != "location:old" {
		t.Fatalf("失败后主机被修改: %+v, %v", got, err)
	}
}
