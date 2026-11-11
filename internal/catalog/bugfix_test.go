package catalog_test

import (
	"strings"
	"testing"

	"github.com/CosmoGao/rollbook/internal/catalog"
	storedb "github.com/CosmoGao/rollbook/internal/db"
	"github.com/CosmoGao/rollbook/internal/search"
)

func TestSharedProxyEntriesAndOrder(t *testing.T) {
	ctx, db, service, _, user := serviceSetup(t)
	store := service.Store
	add(t, ctx, store, "location:site", "location", "站点", map[string]any{"kind": "站点"})
	add(t, ctx, store, "host:one", "host", "主机", map[string]any{"kind": "物理机", "location_id": "location:site"})
	for _, id := range []string{"app", "zoraxy", "caddy"} {
		add(t, ctx, store, "service:"+id, "service", id, map[string]any{"host_id": "host:one"})
	}
	entry := func(id string, via []string) {
		add(t, ctx, store, "endpoint:"+id, "endpoint", id, map[string]any{"service_id": "service:app", "kind": "反向代理", "url": "https://example.org/" + id, "via_json": via})
	}
	entry("outside", []string{"service:zoraxy", "service:caddy"})
	entry("inside", []string{"service:zoraxy"})                   // B-01：共用第一跳不会撞唯一约束。
	entry("reverse", []string{"service:caddy", "service:zoraxy"}) // B-02：顺序相反也不是循环。
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM dependencies WHERE kind='反代'`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("反代边数量=%d, err=%v", count, err)
	}
	top, err := (search.Store{DB: db}).Topology(ctx, "service:app", 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, edge := range top.Edges {
		if edge.Kind == "反代" && edge.From != "service:app" {
			t.Fatalf("存在虚构的反代间依赖: %+v", edge)
		}
	}
	// 移除持有共享边的入口后，其余入口仍保有该边。
	_, err = service.Update(ctx, user, "endpoint:outside", 1, catalog.Patch{Fields: map[string]any{"via_json": []string{}}}, "修改入口", "test")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM dependencies WHERE from_id='service:app' AND to_id='service:zoraxy' AND kind='反代'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("共享边丢失: %d, %v", count, err)
	}
	// 模拟旧库中保存的虚构第二跳；增量迁移应删掉它并保留入口链。
	const legacyID = "dependency:auto:endpoint:reverse:1"
	_, err = db.ExecContext(ctx, `INSERT INTO objects(id,type,name,created_at,created_by,updated_at,updated_by) VALUES(?,'dependency','反代','2020-01-01','test','2020-01-01','test')`, legacyID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO dependencies(id,from_id,to_id,kind,generated) VALUES(?,'service:caddy','service:zoraxy','反代',1)`, legacyID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `DELETE FROM schema_migrations WHERE version=6`); err != nil {
		t.Fatal(err)
	}
	if err = storedb.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `SELECT count(*) FROM dependencies WHERE id=?`, legacyID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("旧版虚构边未清理: %d, %v", count, err)
	}
}

func TestIPAddressMoveUpdatesNetworkNameAndSearch(t *testing.T) {
	ctx, db, service, _, user := serviceSetup(t)
	store := service.Store
	add(t, ctx, store, "location:site", "location", "站点", map[string]any{"kind": "站点"})
	add(t, ctx, store, "host:one", "host", "主机", map[string]any{"kind": "物理机", "location_id": "location:site"})
	add(t, ctx, store, "network:first", "network", "网段一", map[string]any{"cidr": "10.1.0.0/24", "location_id": "location:site"})
	add(t, ctx, store, "network:second", "network", "网段二", map[string]any{"cidr": "10.2.0.0/24", "location_id": "location:site"})
	add(t, ctx, store, "ip:one", "ip", "10.1.0.5", map[string]any{"address": "10.1.0.5", "host_id": "host:one"})
	for rev, address := range []string{"10.1.0.6", "10.2.0.6"} {
		got, err := service.Update(ctx, user, "ip:one", int64(rev+1), catalog.Patch{Fields: map[string]any{"address": address}}, "换地址", "test")
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != address {
			t.Fatalf("B-04: 名称=%s", got.Name)
		}
	}
	got, err := store.Get(ctx, "ip:one")
	if err != nil {
		t.Fatal(err)
	}
	if got.Fields["network_id"] != "network:second" || got.Name != "10.2.0.6" {
		t.Fatalf("B-03/B-04: %+v", got)
	}
	results, err := (search.Store{DB: db}).Search(ctx, "10.2.0.6")
	if err != nil || len(results) != 1 || results[0].Name != got.Name || results[0].NetworkID != "network:second" {
		t.Fatalf("搜索不同步: %+v, %v", results, err)
	}
}

func TestHostMoveRejectsIPOutsideNewSite(t *testing.T) {
	ctx, _, service, _, user := serviceSetup(t)
	store := service.Store
	for _, site := range []string{"old", "new"} {
		add(t, ctx, store, "location:"+site, "location", site, map[string]any{"kind": "站点"})
	}
	add(t, ctx, store, "network:old", "network", "旧网段", map[string]any{"cidr": "10.1.0.0/24", "location_id": "location:old"})
	add(t, ctx, store, "network:new", "network", "新网段", map[string]any{"cidr": "10.2.0.0/24", "location_id": "location:new"})
	add(t, ctx, store, "host:one", "host", "主机", map[string]any{"kind": "物理机", "location_id": "location:old"})
	add(t, ctx, store, "ip:one", "ip", "10.1.0.5", map[string]any{"address": "10.1.0.5", "host_id": "host:one"})
	_, err := service.Update(ctx, user, "host:one", 1, catalog.Patch{Fields: map[string]any{"location_id": "location:new"}}, "搬站点", "test")
	if err == nil || !strings.Contains(err.Error(), "10.1.0.5") || !strings.Contains(err.Error(), "location_id") {
		t.Fatalf("B-05: 错误未指出 IP 和字段: %v", err)
	}
	got, err := store.Get(ctx, "host:one")
	if err != nil || got.Fields["location_id"] != "location:old" || got.Rev != 1 {
		t.Fatalf("失败后修改未回滚: %+v, %v", got, err)
	}
}
