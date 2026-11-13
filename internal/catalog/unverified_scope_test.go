package catalog_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/CosmoGao/rollbook/internal/catalog"
)

func TestUnverifiedOnlySitesHostsServices(t *testing.T) {
	ctx, database, service, _, owner := serviceSetup(t)
	assets := []catalog.Object{
		{ID: "location:site", Type: "location", Name: "北京家", Fields: map[string]any{"kind": "站点"}},
		{ID: "location:room", Type: "location", Name: "机柜", Fields: map[string]any{"kind": "机柜", "parent_id": "location:site"}},
		{ID: "network:lan", Type: "network", Name: "LAN", Fields: map[string]any{"cidr": "192.0.2.0/24", "location_id": "location:site"}},
		{ID: "host:box", Type: "host", Name: "主机", Fields: map[string]any{"kind": "物理机", "location_id": "location:room"}},
		{ID: "service:app", Type: "service", Name: "服务", Fields: map[string]any{"host_id": "host:box"}},
		{ID: "ip:box", Type: "ip", Name: "IP", Fields: map[string]any{"address": "192.0.2.10", "host_id": "host:box", "network_id": "network:lan"}},
		{ID: "port:app", Type: "port", Name: "端口", Fields: map[string]any{"service_id": "service:app", "port": 8080, "protocol": "tcp"}},
		{ID: "context:hint", Type: "context", Name: "上下文", Fields: map[string]any{"scope": "全局", "key": "hint", "body_md": "内容", "owner": "tester"}},
	}
	for _, asset := range assets {
		var err error
		if asset.Type == "context" {
			_, err = service.Store.Create(ctx, asset, "tester")
		} else {
			_, err = service.CreateAsset(ctx, owner, asset, 0, "录入", "test")
		}
		if err != nil {
			t.Fatal(asset.ID, err)
		}
	}
	// 以足以挤满旧查询结果的辅助对象验证角标口径不会随库规模膨胀。
	for i := 0; i < 180; i++ {
		for _, asset := range []catalog.Object{
			{ID: fmt.Sprintf("ip:extra-%d", i), Type: "ip", Name: "附加 IP", Fields: map[string]any{"address": fmt.Sprintf("192.0.2.%d", i+30), "host_id": "host:box", "network_id": "network:lan"}},
			{ID: fmt.Sprintf("port:extra-%d", i), Type: "port", Name: "附加端口", Fields: map[string]any{"service_id": "service:app", "port": i + 8000, "protocol": "tcp"}},
			{ID: fmt.Sprintf("context:extra-%d", i), Type: "context", Name: "附加上下文", Fields: map[string]any{"scope": "全局", "key": fmt.Sprintf("hint-%d", i), "body_md": "内容", "owner": "tester"}},
		} {
			if _, err := service.Store.Create(ctx, asset, "tester"); err != nil {
				t.Fatal(asset.ID, err)
			}
		}
	}
	query := catalog.Query{Service: service}
	check := func(want ...string) {
		t.Helper()
		got, err := query.Unverified(ctx, owner)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		set := map[string]bool{}
		for _, item := range got {
			set[item.ID] = true
		}
		for _, id := range want {
			if !set[id] {
				t.Fatalf("missing %s from %v", id, got)
			}
		}
	}
	check("location:site", "host:box", "service:app")
	if _, err := database.ExecContext(ctx, "UPDATE objects SET verified_at=? WHERE id='host:box'", time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	check("location:site", "service:app")
	if _, err := database.ExecContext(ctx, "UPDATE objects SET verified_at=? WHERE id='host:box'", time.Now().AddDate(0, 0, -31).UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	check("location:site", "host:box", "service:app")
	var total int
	if err := database.QueryRowContext(ctx, "SELECT count(*) FROM objects WHERE type IN ('ip','port','context')").Scan(&total); err != nil || total != 543 {
		t.Fatal(total, err)
	}
}
