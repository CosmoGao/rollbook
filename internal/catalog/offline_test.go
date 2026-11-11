package catalog_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/CosmoGao/rollbook/internal/backup"
	"github.com/CosmoGao/rollbook/internal/catalog"
	"github.com/CosmoGao/rollbook/internal/search"
	"gopkg.in/yaml.v3"
)

func TestOfflineReleaseRestoreConflictAndManualChange(t *testing.T) {
	ctx, database, service, _, owner := serviceSetup(t)
	chain := []catalog.Object{
		{ID: "location:test", Type: "location", Name: "站点", Fields: map[string]any{"kind": "站点"}},
		{ID: "network:test", Type: "network", Name: "网段", Fields: map[string]any{"cidr": "192.0.2.0/24", "location_id": "location:test"}},
		{ID: "host:test", Type: "host", Name: "主机", Fields: map[string]any{"kind": "物理机", "location_id": "location:test"}},
		{ID: "ip:test", Type: "ip", Name: "192.0.2.5", Fields: map[string]any{"address": "192.0.2.5", "host_id": "host:test"}},
		{ID: "service:test", Type: "service", Name: "服务", Fields: map[string]any{"host_id": "host:test"}},
		{ID: "endpoint:test", Type: "endpoint", Name: "入口", Fields: map[string]any{"service_id": "service:test", "kind": "内网直连", "url": "https://example.invalid"}},
	}
	for _, input := range chain {
		if _, err := service.CreateAsset(ctx, owner, input, 0, "初始化", "web"); err != nil {
			t.Fatal(input.ID, err)
		}
	}
	preview, err := service.OfflinePreview(ctx, owner, "host:test")
	if err != nil || len(preview.Services) != 1 || len(preview.IPs) != 1 || len(preview.Endpoints) != 1 {
		t.Fatal(preview, err)
	}
	op, err := service.OfflineHost(ctx, owner, "host:test", 1, catalog.OfflineOptions{Services: true, IPs: true, Endpoints: true}, "迁移设备")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"host:test", "service:test", "ip:test", "endpoint:test"} {
		o, e := service.Store.Get(ctx, id)
		if e != nil || o.Status != "停用" {
			t.Fatal(id, o, e)
		}
	}
	hits, err := (search.Store{DB: database}).Search(ctx, "主机")
	if err != nil || len(hits) == 0 || hits[0].Status != "停用" {
		t.Fatal(hits, err)
	}
	other := catalog.Object{ID: "host:other", Type: "host", Name: "新主机", Fields: map[string]any{"kind": "物理机", "location_id": "location:test"}}
	if _, err = service.CreateAsset(ctx, owner, other, 0, "迁入", "web"); err != nil {
		t.Fatal(err)
	}
	occupied := catalog.Object{ID: "ip:occupied", Type: "ip", Name: "192.0.2.5", Fields: map[string]any{"address": "192.0.2.5", "host_id": "host:other"}}
	if _, err = service.CreateAsset(ctx, owner, occupied, 0, "分配释放地址", "web"); err != nil {
		t.Fatal(err)
	}
	_, err = service.RestoreHost(ctx, owner, "host:test", op, "设备恢复")
	var validation *catalog.ValidationError
	if !errors.As(err, &validation) || len(validation.Fields) == 0 || len(validation.Fields[0].References) == 0 || validation.Fields[0].References[0] != "ip:occupied" {
		t.Fatal(err)
	}
	if err = service.DeleteAsset(ctx, owner, "ip:occupied", 1, "撤销分配"); err != nil {
		t.Fatal(err)
	}
	// 停用后手工修改服务状态，恢复应跳过它并保留人工变更。
	status := "在用"
	if _, err = service.Update(ctx, owner, "service:test", 2, catalog.Patch{Status: &status}, "人工调整", "web"); err != nil {
		t.Fatal(err)
	}
	result, err := service.RestoreHost(ctx, owner, "host:test", op, "设备恢复")
	if err != nil || len(result.Skipped) != 1 || result.Skipped[0] != "service:test" || len(result.Restored) != 3 {
		t.Fatal(result, err)
	}
	ip, err := service.Store.Get(ctx, "ip:test")
	if err != nil || ip.Fields["address"] != "192.0.2.5" || ip.Status != "在用" {
		t.Fatal(ip, err)
	}
	var audits int
	if err = database.QueryRow(`SELECT count(*) FROM changes WHERE action IN ('host_offline','host_restore')`).Scan(&audits); err != nil || audits != 7 {
		t.Fatal(audits, err)
	}
	if _, err = service.RestoreHost(ctx, owner, "host:test", op, "重复恢复"); !errors.Is(err, catalog.ErrConflict) {
		t.Fatal(err)
	}
	if err = service.DeleteAsset(ctx, owner, "host:test", 3, "删除主机"); !errors.As(err, &validation) || !strings.Contains(strings.Join(validation.Fields[0].References, ","), "ip:test") {
		t.Fatal(err)
	}
}

func TestReleasedIPBackupAndUnrelatedMerge(t *testing.T) {
	ctx, database, service, _, owner := serviceSetup(t)
	backups := backup.Service{DB: database}
	for _, input := range []catalog.Object{
		{ID: "location:site", Type: "location", Name: "站点", Fields: map[string]any{"kind": "站点"}},
		{ID: "network:net", Type: "network", Name: "网段", Fields: map[string]any{"cidr": "192.0.2.0/24", "location_id": "location:site"}},
		{ID: "host:box", Type: "host", Name: "主机", Fields: map[string]any{"kind": "物理机", "location_id": "location:site"}},
		{ID: "ip:one", Type: "ip", Name: "192.0.2.5", Fields: map[string]any{"address": "192.0.2.5", "host_id": "host:box"}},
	} {
		if _, err := service.CreateAsset(ctx, owner, input, 0, "创建", "web"); err != nil {
			t.Fatal(err)
		}
	}
	before, err := backups.Export(ctx, "catalog")
	if err != nil {
		t.Fatal(err)
	}
	op, err := service.OfflineHost(ctx, owner, "host:box", 1, catalog.OfflineOptions{IPs: true}, "下线并释放地址")
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"catalog", "full"} {
		data, e := backups.Export(ctx, kind)
		if e != nil {
			t.Fatal(kind, e)
		}
		f, preview, e := backup.Parse(data)
		if e != nil || preview.Counts["host_offline_operations"] != 1 {
			t.Fatal(kind, preview, e)
		}
		if e = backup.Validate(ctx, f); e != nil {
			t.Fatal(kind, e)
		}
	}
	merge := []byte("schema_version: 1\nkind: merge\nobjects:\n  - id: location:new\n    type: location\n    name: 新站点\n    fields: {kind: 站点}\n")
	pending, err := backups.Submit(ctx, merge, owner)
	if err != nil {
		t.Fatal(err)
	}
	if err = backups.Review(ctx, pending.ID, true, "owner"); err != nil {
		t.Fatal("无关合并失败", err)
	}
	if _, err = service.Store.Get(ctx, "location:new"); err != nil {
		t.Fatal(err)
	}
	data, err := backups.Export(ctx, "catalog")
	if err != nil {
		t.Fatal(err)
	}
	f, _, err := backup.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if err = backups.Restore(ctx, f); err != nil {
		t.Fatal("带快照恢复", err)
	}
	if _, err = service.RestoreHost(ctx, owner, "host:box", op, "恢复主机"); err != nil {
		t.Fatal("快照未恢复", err)
	}
	// 旧版备份缺少快照表，恢复时应清掉当前操作，且不能误用。
	old, _, err := backup.Parse(before)
	if err != nil {
		t.Fatal(err)
	}
	delete(old.Tables, "host_offline_operations")
	oldYAML, err := yaml.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	old, _, err = backup.Parse(oldYAML)
	if err != nil {
		t.Fatal("旧备份无法解析", err)
	}
	if err = backups.Restore(ctx, old); err != nil {
		t.Fatal("旧备份无法恢复", err)
	}
	var count int
	if err = database.QueryRowContext(ctx, "SELECT count(*) FROM host_offline_operations").Scan(&count); err != nil || count != 0 {
		t.Fatal("旧快照残留", count, err)
	}
}
