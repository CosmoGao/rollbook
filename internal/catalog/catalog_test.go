package catalog_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/CosmoGao/rollbook/internal/catalog"
	"github.com/CosmoGao/rollbook/internal/db"
	"github.com/CosmoGao/rollbook/internal/search"
)

func setup(t *testing.T) (context.Context, *sql.DB, catalog.Store) {
	t.Helper()
	ctx := context.Background()
	d, e := db.Open(ctx, filepath.Join(t.TempDir(), "catalog.db"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { d.Close() })
	return ctx, d, catalog.Store{DB: d}
}
func add(t *testing.T, ctx context.Context, s catalog.Store, id, typ, name string, f map[string]any) catalog.Object {
	t.Helper()
	o, e := s.Create(ctx, catalog.Object{ID: id, Type: typ, Name: name, Fields: f}, "tester")
	if e != nil {
		t.Fatalf("create %s: %v", id, e)
	}
	return o
}

func TestCatalogRelationsSearchAndArchive(t *testing.T) {
	ctx, d, s := setup(t)
	add(t, ctx, s, "location:beijing", "location", "北京", map[string]any{"kind": "站点"})
	add(t, ctx, s, "location:xianghe", "location", "香河站点", map[string]any{"kind": "站点", "city": "香河"})
	add(t, ctx, s, "uplink:xianghe", "uplink", "联通接入", map[string]any{"location_id": "location:xianghe", "isp": "联通", "type": "光纤宽带", "public_ipv4": "待核实"})
	add(t, ctx, s, "uplink:mobile", "uplink", "移动备用接入", map[string]any{"location_id": "location:xianghe", "isp": "移动", "type": "4G·5G"})
	add(t, ctx, s, "host:router", "host", "香河路由器", map[string]any{"kind": "网络设备", "net_role": "主路由", "location_id": "location:xianghe"})
	add(t, ctx, s, "network:xianghe", "network", "192.168.10 网段", map[string]any{"cidr": "192.168.10.0/24", "location_id": "location:xianghe", "router_id": "host:router", "uplink_id": "uplink:xianghe"})
	add(t, ctx, s, "host:multica", "host", "Multica 服务端", map[string]any{"kind": "物理机", "location_id": "location:beijing"})
	add(t, ctx, s, "network:beijing", "network", "172.20.1 网段", map[string]any{"cidr": "172.20.1.0/24", "location_id": "location:beijing"})
	add(t, ctx, s, "network:beijing-sub", "network", "172.20.1.0/25", map[string]any{"cidr": "172.20.1.0/25", "location_id": "location:beijing"})
	ip := add(t, ctx, s, "ip:multica", "ip", "服务端 IP", map[string]any{"address": "172.20.1.126", "host_id": "host:multica", "assign": "静态"})
	if ip.Fields["network_id"] != "network:beijing-sub" {
		t.Fatal(ip.Fields)
	}
	add(t, ctx, s, "service:multica", "service", "Multica", map[string]any{"host_id": "host:multica"})
	add(t, ctx, s, "port:multica", "port", "Multica 端口", map[string]any{"service_id": "service:multica", "port": 8080, "protocol": "tcp"})
	add(t, ctx, s, "service:zoraxy", "service", "Zoraxy", map[string]any{"host_id": "host:multica"})
	add(t, ctx, s, "service:caddy", "service", "Caddy", map[string]any{"host_id": "host:multica"})
	add(t, ctx, s, "endpoint:multica", "endpoint", "Multica 入口", map[string]any{"service_id": "service:multica", "kind": "反向代理", "url": "https://multica.app.v2rich.com:5443", "via_json": []string{"service:zoraxy", "service:caddy"}})
	searcher := search.Store{DB: d}
	res, e := searcher.Search(ctx, "126")
	if e != nil {
		t.Fatal(e)
	}
	if len(res) != 1 || res[0].HostID != "host:multica" || res[0].NetworkID != "network:beijing-sub" || !contains(res[0].RelatedIDs, "endpoint:multica") {
		t.Fatalf("bad IP result: %+v", res)
	}
	res, e = searcher.Search(ctx, "8080")
	if e != nil || len(res) != 1 || res[0].ID != "port:multica" || res[0].HostID != "host:multica" {
		t.Fatalf("bad port result: %+v, %v", res, e)
	}
	res, e = searcher.Search(ctx, "香河")
	if e != nil {
		t.Fatal(e)
	}
	if len(res) == 0 || !hasResult(res, "location:xianghe") {
		t.Fatalf("site not found: %+v", res)
	}
	site, e := searcher.GetSite(ctx, "香河")
	if e != nil {
		t.Fatal(e)
	}
	if !contains(site.Networks, "network:xianghe") || !contains(site.Uplinks, "uplink:xianghe") || !contains(site.Routers, "host:router") {
		t.Fatalf("bad site: %+v", site)
	}
	top, e := searcher.Topology(ctx, "service:multica", 2)
	if e != nil {
		t.Fatal(e)
	}
	if !contains(top.Nodes, "service:zoraxy") || !contains(top.Nodes, "host:multica") {
		t.Fatalf("bad topology: %+v", top)
	}
	obj, e := s.Get(ctx, "ip:multica")
	if e != nil {
		t.Fatal(e)
	}
	if obj.Rev != 1 || obj.CreatedBy != "tester" || obj.Fields["network_id"] != "network:beijing-sub" {
		t.Fatalf("bad persisted object: %+v", obj)
	}
	add(t, ctx, s, "context:short", "context", "短期上下文", map[string]any{"scope": "会话", "key": "short", "body_md": "hello", "owner": "agent", "related_ids_json": []string{"host:multica"}, "expires_at": time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano)})
	active, e := s.Contexts(ctx, false, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	if len(active) != 0 {
		t.Fatalf("active: %+v", active)
	}
	archive, e := s.Contexts(ctx, true, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	if len(archive) != 1 {
		t.Fatalf("archive: %+v", archive)
	}
	if e = db.Migrate(ctx, d); e != nil {
		t.Fatal(e)
	}
}

func TestRejectInvalidRelations(t *testing.T) {
	ctx, _, s := setup(t)
	add(t, ctx, s, "location:a", "location", "A", map[string]any{"kind": "站点"})
	add(t, ctx, s, "host:a", "host", "A", map[string]any{"kind": "物理机", "location_id": "location:a"})
	add(t, ctx, s, "network:a", "network", "A", map[string]any{"cidr": "10.0.0.0/24", "location_id": "location:a"})
	bad := []catalog.Object{
		{ID: "ip:bad", Type: "ip", Name: "bad", Fields: map[string]any{"address": "10.0.1.1", "host_id": "host:a"}},
		{ID: "host:bad", Type: "host", Name: "bad", Fields: map[string]any{"kind": "物理机", "location_id": "location:a", "parent_host_id": "host:missing"}},
		{ID: "network:bad", Type: "network", Name: "bad", Fields: map[string]any{"cidr": "10.0.0.1/24", "location_id": "location:a"}},
		{ID: "endpoint:bad", Type: "endpoint", Name: "bad", Fields: map[string]any{"service_id": "service:missing", "kind": "反向代理", "url": "https://example.org", "via_json": []string{"service:missing"}}},
		{ID: "context:bad", Type: "context", Name: "bad", Fields: map[string]any{"scope": "全局", "key": "x", "body_md": "x", "owner": "x", "related_ids_json": []string{"host:missing"}}},
	}
	for _, o := range bad {
		if _, e := s.Create(ctx, o, "tester"); e == nil {
			t.Errorf("accepted %+v", o)
		}
	}
	add(t, ctx, s, "dependency:a", "dependency", "depends", map[string]any{"from_id": "host:a", "to_id": "location:a", "kind": "manual"})
	if _, e := s.Create(ctx, catalog.Object{ID: "dependency:b", Type: "dependency", Name: "cycle", Fields: map[string]any{"from_id": "location:a", "to_id": "host:a", "kind": "manual"}}, "tester"); e == nil {
		t.Fatal("accepted cycle")
	}
}

func TestIPUsesSiteTreeAndRejectsEqualPrefix(t *testing.T) {
	ctx, _, s := setup(t)
	add(t, ctx, s, "location:site", "location", "站点", map[string]any{"kind": "站点"})
	add(t, ctx, s, "location:room", "location", "房间", map[string]any{"kind": "房间", "parent_id": "location:site"})
	add(t, ctx, s, "host:room", "host", "房间主机", map[string]any{"kind": "物理机", "location_id": "location:room"})
	add(t, ctx, s, "network:site", "network", "站点网段", map[string]any{"cidr": "10.10.0.0/24", "location_id": "location:site"})
	ip := add(t, ctx, s, "ip:room", "ip", "房间 IP", map[string]any{"address": "10.10.0.9", "host_id": "host:room"})
	if ip.Fields["network_id"] != "network:site" {
		t.Fatal(ip.Fields)
	}
	add(t, ctx, s, "network:room", "network", "重叠网段", map[string]any{"cidr": "10.10.0.0/24", "location_id": "location:room"})
	if _, err := s.Create(ctx, catalog.Object{ID: "ip:ambiguous", Type: "ip", Name: "歧义", Fields: map[string]any{"address": "10.10.0.10", "host_id": "host:room"}}, "tester"); err == nil {
		t.Fatal("accepted ambiguous longest prefix")
	}
}
func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func hasResult(xs []search.Result, id string) bool {
	for _, x := range xs {
		if x.ID == id {
			return true
		}
	}
	return false
}
