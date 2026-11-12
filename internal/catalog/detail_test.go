package catalog_test

import (
	"errors"
	"testing"
	"time"

	"github.com/CosmoGao/rollbook/internal/auth"
	"github.com/CosmoGao/rollbook/internal/catalog"
)

func TestAssetDetailAndChangeFilters(t *testing.T) {
	ctx, d, service, _, user := serviceSetup(t)
	const now = "2026-11-12T10:00:00Z"
	add := func(id, typ, name string) {
		t.Helper()
		if _, err := d.Exec(`INSERT INTO objects(id,type,name,created_at,created_by,updated_at,updated_by) VALUES(?,?,?,?,?,?,?)`, id, typ, name, now, "owner", now, "owner"); err != nil {
			t.Fatal(err)
		}
	}
	add("location:site", "location", "站点")
	add("host:box", "host", "主机")
	add("network:lan", "network", "网段")
	add("service:app", "service", "服务")
	add("endpoint:web", "endpoint", "入口")
	add("note:one", "note", "笔记")
	add("credential:one", "credential", "凭据")
	add("change:one", "change", "变更")
	add("change:two", "change", "变更")
	stmts := []string{
		`INSERT INTO locations(id,kind) VALUES('location:site','站点')`,
		`INSERT INTO hosts(id,kind,location_id) VALUES('host:box','物理机','location:site')`,
		`INSERT INTO networks(id,cidr,location_id) VALUES('network:lan','192.0.2.0/24','location:site')`,
		`INSERT INTO services(id,host_id) VALUES('service:app','host:box')`,
		`INSERT INTO endpoints(id,service_id,kind,url) VALUES('endpoint:web','service:app','内网直连','https://example.invalid')`,
		`INSERT INTO notes(id,target_id,kind,body_md,author) VALUES('note:one','host:box','运维笔记','<script>alert(1)</script>','agent')`,
		`INSERT INTO credentials(id,target_id,vault,item_ref) VALUES('credential:one','host:box','vault','item')`,
		`INSERT INTO changes(id,object_type,object_id,action,diff_json,actor,actor_key_id,actor_key_name,via,reason,state) VALUES('change:one','host','host:box','update','{}','agent','key:one','agent-one','rest','修改','已生效')`,
		`INSERT INTO changes(id,object_type,object_id,action,diff_json,actor,actor_key_id,actor_key_name,via,reason,state) VALUES('change:two','service','service:app','update','{}','agent','key:two','agent-two','rest','修改','已生效')`,
	}
	for _, s := range stmts {
		if _, err := d.Exec(s); err != nil {
			t.Fatal(err)
		}
	}
	q := catalog.Query{Service: service}
	for _, id := range []string{"host:box", "network:lan", "service:app", "endpoint:web"} {
		got, err := q.AssetDetail(ctx, user, id)
		if err != nil {
			t.Fatal(id, err)
		}
		if got.Object.ID != id {
			t.Fatal(got.Object.ID)
		}
		if id == "host:box" && (len(got.Notes) != 1 || len(got.Credentials) != 1 || len(got.Changes) != 1 || len(got.ReferencedBy) != 1 || got.ReferencedBy[0].ID != "service:app") {
			t.Fatalf("host detail: %+v", got)
		}
		if id == "service:app" && (len(got.ReferencedBy) != 1 || got.ReferencedBy[0].ID != "endpoint:web") {
			t.Fatalf("service refs: %+v", got.ReferencedBy)
		}
	}
	if _, err := q.AssetDetail(ctx, auth.Principal{}, "host:box"); !errors.Is(err, auth.ErrUnauthorized) {
		t.Fatal(err)
	}
	filtered, err := q.FilterChanges(ctx, user, "", "", "key:one", "host", now, time.Date(2026, 11, 12, 11, 0, 0, 0, time.UTC).Format(time.RFC3339))
	if err != nil || len(filtered) != 1 || filtered[0].ID != "change:one" {
		t.Fatal(filtered, err)
	}
	for _, args := range [][2]string{{"key:two", "host"}, {"key:one", "service"}} {
		out, err := q.FilterChanges(ctx, user, "", "", args[0], args[1], "", "")
		if err != nil || len(out) != 0 {
			t.Fatal(out, err)
		}
	}
	if _, err := q.FilterChanges(ctx, user, "", "", "", "host", "bad", ""); !errors.Is(err, catalog.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := q.FilterChanges(ctx, auth.Principal{}, "", "", "", "", "", ""); !errors.Is(err, auth.ErrUnauthorized) {
		t.Fatal(err)
	}
}
