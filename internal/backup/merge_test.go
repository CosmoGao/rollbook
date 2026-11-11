package backup

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/CosmoGao/rollbook/internal/audit"
	"github.com/CosmoGao/rollbook/internal/auth"
	"github.com/CosmoGao/rollbook/internal/db"
)

func TestMergePreviewApprovalAndRollback(t *testing.T) {
	ctx := context.Background()
	d, err := db.Open(ctx, filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	for _, q := range []string{
		`INSERT INTO objects(id,type,name,created_at,created_by,updated_at,updated_by) VALUES('location:a','location','旧站点','now','owner','now','owner')`,
		`INSERT INTO locations(id,kind) VALUES('location:a','站点')`,
		`INSERT INTO objects(id,type,name,created_at,created_by,updated_at,updated_by) VALUES('location:keep','location','保留站点','now','owner','now','owner')`,
		`INSERT INTO locations(id,kind) VALUES('location:keep','站点')`,
	} {
		if _, err = d.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	s := Service{DB: d}
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	oldChange, err := audit.AppendTx(ctx, tx, audit.Entry{ObjectID: "location:keep", ObjectType: "location", Action: "create", Diff: map[string]any{"name": "保留站点"}, Actor: auth.Principal{Login: "owner"}, Via: "test", Reason: "初始记录", State: "已生效"})
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	actor := auth.Principal{KeyID: "key-1", KeyName: "agent", IssuerID: "owner-id", IssuerName: "owner", Level: auth.Propose}
	yaml := []byte(`schema_version: 1
kind: merge
objects:
  - id: location:a
    type: location
    rev: 1
    name: 新站点
    fields: {kind: 站点}
  - id: location:b
    type: location
    name: 新增站点
    fields: {kind: 站点}
  - id: location:keep
    type: location
    rev: 99
    name: 冲突
    fields: {kind: 站点}
`)
	pending, err := s.Submit(ctx, yaml, actor)
	if err != nil {
		t.Fatal(err)
	}
	if pending.Preview.Counts["create"] != 1 || pending.Preview.Counts["update"] != 1 || pending.Preview.Counts["conflict"] != 1 {
		t.Fatal(pending.Preview)
	}
	if err = s.Review(ctx, pending.ID, true, "owner"); err == nil {
		t.Fatal("conflict approved")
	}
	var count int
	d.QueryRowContext(ctx, "SELECT count(*) FROM objects WHERE id='location:b'").Scan(&count)
	if count != 0 {
		t.Fatal("partial merge")
	}
	var old string
	d.QueryRowContext(ctx, "SELECT name FROM objects WHERE id='location:a'").Scan(&old)
	if old != "旧站点" {
		t.Fatal(old)
	}
	valid := []byte(`schema_version: 1
kind: merge
objects:
  - id: location:a
    type: location
    rev: 1
    name: 新站点
    fields: {kind: 站点}
  - id: location:b
    type: location
    name: 新增站点
    aliases: [别名站点]
    fields: {kind: 站点}
`)
	pending, err = s.Submit(ctx, valid, actor)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Review(ctx, pending.ID, true, "owner"); err != nil {
		t.Fatal(err)
	}
	d.QueryRowContext(ctx, "SELECT name FROM objects WHERE id='location:keep'").Scan(&old)
	if old != "保留站点" {
		t.Fatal(old)
	}
	d.QueryRowContext(ctx, "SELECT count(*) FROM changes WHERE actor_type='key' AND action IN ('create','update')").Scan(&count)
	if count != 2 {
		t.Fatal("audit count", count)
	}
	d.QueryRowContext(ctx, "SELECT count(*) FROM changes WHERE id=?", oldChange).Scan(&count)
	if count != 1 {
		t.Fatal("history lost")
	}
	d.QueryRowContext(ctx, "SELECT count(*) FROM objects WHERE id='location:b'").Scan(&count)
	if count != 1 {
		t.Fatal("create missing")
	}
	// 已批准的变更再次审批不得覆盖后续修改。
	if err = s.Review(ctx, pending.ID, true, "owner"); err == nil {
		t.Fatal("replayed approval")
	}
	invalid := []byte(`schema_version: 1
kind: merge
objects:
  - id: host:bad
    type: host
    name: 错误主机
    fields: {kind: 物理机, location_id: 'location:missing'}
`)
	pending, err = s.Submit(ctx, invalid, actor)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Review(ctx, pending.ID, true, "owner"); err == nil {
		t.Fatal("invalid relation committed")
	}
	d.QueryRowContext(ctx, "SELECT count(*) FROM objects WHERE id='host:bad'").Scan(&count)
	if count != 0 {
		t.Fatal("rollback failed")
	}
}
