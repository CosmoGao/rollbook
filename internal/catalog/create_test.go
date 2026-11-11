package catalog_test

import (
	"errors"
	"testing"

	"github.com/CosmoGao/rollbook/internal/auth"
	"github.com/CosmoGao/rollbook/internal/catalog"
)

func TestCreateChainAndApproval(t *testing.T) {
	ctx, db, service, keys, owner := serviceSetup(t)
	_, token, err := keys.IssueKey(ctx, owner, "writer", auth.Write, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	writer, err := keys.Authenticate(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	chain := []catalog.Object{
		{ID: "location:test", Type: "location", Name: "站点", Fields: map[string]any{"kind": "站点"}},
		{ID: "network:test", Type: "network", Name: "网段", Fields: map[string]any{"cidr": "192.0.2.0/24", "location_id": "location:test"}},
		{ID: "host:test", Type: "host", Name: "主机", Fields: map[string]any{"kind": "物理机", "location_id": "location:test"}},
		{ID: "ip:test", Type: "ip", Name: "IP", Fields: map[string]any{"address": "192.0.2.5", "host_id": "host:test"}},
		{ID: "service:test", Type: "service", Name: "服务", Fields: map[string]any{"host_id": "host:test"}},
		{ID: "endpoint:test", Type: "endpoint", Name: "入口", Fields: map[string]any{"service_id": "service:test", "kind": "内网直连", "url": "https://example.invalid"}},
		{ID: "dependency:test", Type: "dependency", Name: "手工依赖", Fields: map[string]any{"from_id": "service:test", "to_id": "network:test", "kind": "手工"}},
	}
	for _, input := range chain {
		created, e := service.CreateAsset(ctx, writer, input, 0, "初始化", "rest")
		if e != nil {
			t.Fatalf("%s: %v", input.ID, e)
		}
		if created.CreatedBy == "" || created.Rev != 1 {
			t.Fatal(created)
		}
		var action, state, via string
		if e = db.QueryRow("SELECT action,state,via FROM changes WHERE object_id=?", input.ID).Scan(&action, &state, &via); e != nil || action != "create" || state != "已生效" || via != "rest" {
			t.Fatal(input.ID, action, state, via, e)
		}
	}
	_, token, err = keys.IssueKey(ctx, owner, "proposer", auth.Propose, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	proposer, _ := keys.Authenticate(ctx, token)
	proposal := catalog.Object{ID: "note:proposed", Type: "note", Name: "备注", Fields: map[string]any{"target_id": "service:test", "kind": "运维笔记", "body_md": "说明"}}
	if _, e := service.CreateAsset(ctx, proposer, proposal, 0, "说明", "rest"); !errors.Is(e, auth.ErrForbidden) {
		t.Fatal(e)
	}
	id, e := service.ProposeCreate(ctx, proposer, proposal, 0, "说明", "mcp")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = service.Store.Get(ctx, proposal.ID); e == nil {
		t.Fatal("proposal applied before approval")
	}
	approved, e := service.Review(ctx, owner, id, true)
	if e != nil || approved.ID != proposal.ID {
		t.Fatal(approved, e)
	}
	if approved.Fields["author"] != "proposer (owner)" {
		t.Fatal(approved.Fields)
	}
	var state string
	if e = db.QueryRow("SELECT state FROM changes WHERE id=?", id).Scan(&state); e != nil || state != "已生效" {
		t.Fatal(state, e)
	}
	if _, e = service.Review(ctx, owner, id, true); !errors.Is(e, catalog.ErrConflict) {
		t.Fatal(e)
	}
	deleteID, e := service.ProposeDelete(ctx, proposer, "host:test", 1, "清理", "rest")
	if e != nil {
		t.Fatal(e)
	}
	_, e = service.Review(ctx, owner, deleteID, true)
	var refErr *catalog.ValidationError
	if !errors.As(e, &refErr) || len(refErr.Fields[0].References) == 0 {
		t.Fatal(e)
	}
}

func TestCreateValidationAndStaleApproval(t *testing.T) {
	ctx, db, service, keys, owner := serviceSetup(t)
	_, token, _ := keys.IssueKey(ctx, owner, "proposer", auth.Propose, nil, "")
	proposer, _ := keys.Authenticate(ctx, token)
	_, e := service.CreateAsset(ctx, owner, catalog.Object{ID: "location:bad", Type: "location", Name: "password=abc", Fields: map[string]any{"kind": "站点"}}, 0, "test", "rest")
	var v *catalog.ValidationError
	if !errors.As(e, &v) || v.Fields[0].Field != "object.name" {
		t.Fatal(e)
	}
	_, e = service.CreateAsset(ctx, owner, catalog.Object{ID: "host:bad", Type: "host", Name: "主机", Fields: map[string]any{"kind": "物理机"}}, 0, "test", "rest")
	if !errors.As(e, &v) || v.Fields[0].Field != "fields.location_id" {
		t.Fatalf("%v: %+v", e, v.Fields)
	}
	site := catalog.Object{ID: "location:stale", Type: "location", Name: "站点", Fields: map[string]any{"kind": "站点"}}
	id, e := service.ProposeCreate(ctx, proposer, site, 0, "创建", "rest")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = service.CreateAsset(ctx, owner, site, 0, "抢先创建", "rest"); e != nil {
		t.Fatal(e)
	}
	if _, e = service.Review(ctx, owner, id, true); !errors.Is(e, catalog.ErrConflict) {
		t.Fatal(e)
	}
	_, e = service.CreateAsset(ctx, owner, catalog.Object{ID: "host:temporary", Type: "host", Name: "临时主机", Fields: map[string]any{"kind": "物理机", "location_id": "location:stale"}}, 0, "创建", "rest")
	if e != nil {
		t.Fatal(e)
	}
	change, e := service.ProposeCreate(ctx, proposer, catalog.Object{ID: "service:pending", Type: "service", Name: "待审批服务", Fields: map[string]any{"host_id": "host:temporary"}}, 0, "创建", "mcp")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.ExecContext(ctx, "DELETE FROM hosts WHERE id=?", "host:temporary"); e != nil {
		t.Fatal(e)
	}
	if _, e = db.ExecContext(ctx, "DELETE FROM objects WHERE id=?", "host:temporary"); e != nil {
		t.Fatal(e)
	}
	_, e = service.Review(ctx, owner, change, true)
	if !errors.As(e, &v) || v.Fields[0].Field != "fields.host_id" {
		t.Fatal(e)
	}
}
