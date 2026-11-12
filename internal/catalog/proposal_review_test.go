package catalog_test

import (
	"errors"
	"testing"

	"github.com/CosmoGao/rollbook/internal/auth"
	"github.com/CosmoGao/rollbook/internal/catalog"
)

func TestProposalValidationAndFieldConflict(t *testing.T) {
	ctx, db, service, keys, owner := serviceSetup(t)
	_, raw, _ := keys.IssueKey(ctx, owner, "proposer", auth.Propose, nil, "")
	proposer, _ := keys.Authenticate(ctx, raw)
	base, err := service.CreateAsset(ctx, owner, catalog.Object{ID: "location:review", Type: "location", Name: "站点", Fields: map[string]any{"kind": "站点"}}, 0, "创建", "rest")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Propose(ctx, proposer, base.ID, base.Rev, catalog.Patch{Fields: map[string]any{"unknown": "value"}}, "非法字段", "rest"); err == nil {
		t.Fatal("invalid proposal accepted")
	}
	var pending int
	if err = db.QueryRowContext(ctx, "SELECT count(*) FROM changes WHERE object_id=? AND state='待确认'", base.ID).Scan(&pending); err != nil || pending != 0 {
		t.Fatal(pending, err)
	}
	name := "新名称"
	change, err := service.Propose(ctx, proposer, base.ID, base.Rev, catalog.Patch{Name: &name}, "更名", "rest")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.RecordNote(ctx, proposer, base.ID, base.Rev, "运维笔记", "现场记录", "追加笔记", "rest"); err != nil {
		t.Fatal(err)
	}
	verified, err := service.MarkVerified(ctx, owner, base.ID, base.Rev+1, "核实", "rest")
	if err != nil {
		t.Fatal(err)
	}
	approved, err := service.Review(ctx, owner, change, true)
	if err != nil || approved.Name != name || approved.Rev != verified.Rev+1 {
		t.Fatal(approved, err)
	}
	other := "待更新"
	conflict, err := service.Propose(ctx, proposer, base.ID, approved.Rev, catalog.Patch{Name: &other}, "再更名", "rest")
	if err != nil {
		t.Fatal(err)
	}
	actual := "直接修改"
	if _, err = service.Update(ctx, owner, base.ID, approved.Rev, catalog.Patch{Name: &actual}, "直接修改", "rest"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Review(ctx, owner, conflict, true); !errors.Is(err, catalog.ErrConflict) {
		t.Fatal(err)
	}
	var state string
	if err = db.QueryRowContext(ctx, "SELECT state FROM changes WHERE id=?", conflict).Scan(&state); err != nil || state != "待确认" {
		t.Fatal(state, err)
	}
}

func TestVerifyBatchHostRelationsAndAudit(t *testing.T) {
	ctx, db, service, _, owner := serviceSetup(t)
	objects := []catalog.Object{
		{ID: "location:batch", Type: "location", Name: "站点", Fields: map[string]any{"kind": "站点"}},
		{ID: "network:batch", Type: "network", Name: "网段", Fields: map[string]any{"cidr": "192.0.2.0/24", "location_id": "location:batch"}},
		{ID: "host:batch", Type: "host", Name: "主机", Fields: map[string]any{"kind": "物理机", "location_id": "location:batch"}},
		{ID: "ip:batch", Type: "ip", Name: "IP", Fields: map[string]any{"address": "192.0.2.8", "host_id": "host:batch"}},
		{ID: "service:batch", Type: "service", Name: "服务", Fields: map[string]any{"host_id": "host:batch"}},
		{ID: "port:batch", Type: "port", Name: "端口", Fields: map[string]any{"service_id": "service:batch", "port": 8080, "protocol": "tcp"}},
	}
	for _, o := range objects {
		if _, err := service.CreateAsset(ctx, owner, o, 0, "创建", "rest"); err != nil {
			t.Fatal(o.ID, err)
		}
	}
	change, err := service.Propose(ctx, owner, "port:batch", 1, catalog.Patch{Fields: map[string]any{"purpose": "管理"}}, "补用途", "rest")
	if err != nil {
		t.Fatal(err)
	}
	results, err := service.VerifyBatch(ctx, owner, []string{"host:batch"}, true, "批量核实", "rest")
	if err != nil || len(results) != 3 {
		t.Fatal(results, err)
	}
	for _, result := range results {
		if !result.OK {
			t.Fatal(result)
		}
		var n int
		if err := db.QueryRowContext(ctx, "SELECT count(*) FROM changes WHERE object_id=? AND action='mark_verified'", result.ID).Scan(&n); err != nil || n != 1 {
			t.Fatal(result.ID, n, err)
		}
	}
	if _, err = service.Review(ctx, owner, change, true); err != nil {
		t.Fatal(err)
	}
}
