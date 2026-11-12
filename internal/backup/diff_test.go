package backup

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/CosmoGao/rollbook/internal/db"
)

func TestRestorePreviewShowsDeletedObjectsAndChanges(t *testing.T) {
	ctx := context.Background()
	d, err := db.Open(ctx, filepath.Join(t.TempDir(), "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	s := Service{DB: d}
	data, err := s.Export(ctx, "catalog")
	if err != nil {
		t.Fatal(err)
	}
	target, _, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		`INSERT INTO objects(id,type,name,created_at,created_by,updated_at,updated_by) VALUES('location:new','location','新站点','now','owner','now','owner')`,
		`INSERT INTO locations(id,kind) VALUES('location:new','站点')`,
		`INSERT INTO objects(id,type,name,created_at,created_by,updated_at,updated_by) VALUES('change:new','change','新变更','now','owner','now','owner')`,
		`INSERT INTO changes(id,object_type,object_id,action,diff_json,actor,via,reason,state) VALUES('change:new','location','location:new','create','{}','owner','rest','新增站点','已生效')`,
	} {
		if _, err = d.ExecContext(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	p, hash, err := s.RestorePreview(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	if hash == "" || p.Differences["objects"].Deleted != 2 || p.ObjectDifferences["location"].Deleted != 1 || len(p.LostChanges) != 1 || p.LostChanges[0].Reason != "新增站点" {
		t.Fatal(p)
	}
	if _, err = d.ExecContext(ctx, `UPDATE objects SET name='已修改' WHERE id='location:new'`); err != nil {
		t.Fatal(err)
	}
	_, changedHash, err := s.RestorePreview(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	if changedHash == hash {
		t.Fatal("changed data retained preview fingerprint")
	}
}
