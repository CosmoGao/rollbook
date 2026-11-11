package catalog_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/CosmoGao/rollbook/internal/catalog"
	"github.com/CosmoGao/rollbook/internal/search"
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
