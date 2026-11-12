package catalog_test

import (
	"reflect"
	"testing"

	"github.com/CosmoGao/rollbook/internal/search"
)

func TestTopologyDeduplicatesAndSortsEdges(t *testing.T) {
	ctx, db, store := setup(t)
	add(t, ctx, store, "location:site", "location", "站点", map[string]any{"kind": "站点"})
	add(t, ctx, store, "host:physical", "host", "物理机", map[string]any{"kind": "物理机", "location_id": "location:site"})
	add(t, ctx, store, "host:vm", "host", "虚拟机", map[string]any{"kind": "虚拟机", "location_id": "location:site", "parent_host_id": "host:physical"})
	add(t, ctx, store, "service:app", "service", "Overleaf", map[string]any{"host_id": "host:vm"})
	add(t, ctx, store, "service:proxy", "service", "反代", map[string]any{"host_id": "host:physical"})
	add(t, ctx, store, "host:remote", "host", "另一主机", map[string]any{"kind": "物理机", "location_id": "location:site"})
	add(t, ctx, store, "service:dependent", "service", "依赖 Overleaf 的服务", map[string]any{"host_id": "host:remote"})
	add(t, ctx, store, "endpoint:dependent", "endpoint", "受影响入口", map[string]any{"service_id": "service:dependent", "kind": "内网直连", "url": "https://dependent.example.org"})
	add(t, ctx, store, "dependency:dependent", "dependency", "服务依赖", map[string]any{"from_id": "service:dependent", "to_id": "service:app", "kind": "依赖"})
	for _, id := range []string{"one", "two"} {
		add(t, ctx, store, "endpoint:"+id, "endpoint", id, map[string]any{"service_id": "service:app", "kind": "反向代理", "url": "https://example.org/" + id, "via_json": []string{"service:proxy"}})
	}
	searcher := search.Store{DB: db}
	first, err := searcher.Topology(ctx, "service:app", 2)
	if err != nil {
		t.Fatal(err)
	}
	second, err := searcher.Topology(ctx, "service:app", 2)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("拓扑顺序不稳定: %+v / %+v", first, second)
	}
	seen := map[search.Edge]bool{}
	for _, e := range first.Edges {
		if seen[e] {
			t.Fatalf("重复边: %+v", e)
		}
		seen[e] = true
	}
	if !seen[search.Edge{From: "service:app", To: "host:vm", Kind: "宿主"}] {
		t.Fatalf("缺少宿主边: %+v", first.Edges)
	}
	if len(first.Details) != len(first.Nodes) {
		t.Fatalf("节点详情缺失: %+v", first)
	}
	host, err := searcher.Topology(ctx, "host:physical", 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"host:vm", "service:app", "endpoint:one", "endpoint:two", "service:dependent", "endpoint:dependent"} {
		if !contains(host.Nodes, id) {
			t.Errorf("主机影响范围缺少 %s", id)
		}
	}
	chain, err := searcher.TopologyView(ctx, "service:app", 2, "access")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range []search.Edge{{From: "endpoint:one", To: "service:proxy", Kind: "访问"}, {From: "service:proxy", To: "service:app", Kind: "访问"}, {From: "service:app", To: "host:vm", Kind: "宿主"}, {From: "host:vm", To: "host:physical", Kind: "父主机"}} {
		found := false
		for _, got := range chain.Edges {
			if got == e {
				found = true
			}
		}
		if !found {
			t.Errorf("访问链缺少 %+v: %+v", e, chain.Edges)
		}
	}
	if _, err = searcher.TopologyView(ctx, "service:app", 2, "bad"); err == nil {
		t.Fatal("接受未知视图")
	}
}

func TestAccessChainStopsWithoutProxyOrParent(t *testing.T) {
	ctx, db, store := setup(t)
	add(t, ctx, store, "location:a", "location", "站点", map[string]any{"kind": "站点"})
	add(t, ctx, store, "host:a", "host", "物理机", map[string]any{"kind": "物理机", "location_id": "location:a"})
	add(t, ctx, store, "service:a", "service", "服务", map[string]any{"host_id": "host:a"})
	add(t, ctx, store, "endpoint:a", "endpoint", "直连", map[string]any{"service_id": "service:a", "kind": "内网直连", "url": "https://example.org"})
	chain, err := (search.Store{DB: db}).TopologyView(ctx, "service:a", 2, "access")
	if err != nil {
		t.Fatal(err)
	}
	if len(chain.Nodes) != 3 || len(chain.Edges) != 2 {
		t.Fatalf("访问链应在物理机终止: %+v", chain)
	}
}
