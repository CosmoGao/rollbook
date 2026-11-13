package catalog_test

import (
	"errors"
	"testing"

	"github.com/CosmoGao/rollbook/internal/auth"
	"github.com/CosmoGao/rollbook/internal/catalog"
)

func TestRecursiveImpactAndOfflinePreviewShareResult(t *testing.T) {
	ctx, d, service, _, owner := serviceSetup(t)
	assets := []catalog.Object{
		{ID: "location:site", Type: "location", Name: "北京家", Fields: map[string]any{"kind": "站点"}},
		{ID: "host:proxmox", Type: "host", Name: "Proxmox", Fields: map[string]any{"kind": "物理机", "location_id": "location:site"}},
		{ID: "host:vm", Type: "host", Name: "虚拟机", Fields: map[string]any{"kind": "虚拟机", "location_id": "location:site", "parent_host_id": "host:proxmox"}},
		{ID: "host:container", Type: "host", Name: "容器", Fields: map[string]any{"kind": "容器", "location_id": "location:site", "parent_host_id": "host:vm"}},
		{ID: "host:other", Type: "host", Name: "另一主机", Fields: map[string]any{"kind": "物理机", "location_id": "location:site"}},
		{ID: "service:app", Type: "service", Name: "Immich", Fields: map[string]any{"host_id": "host:container"}},
		{ID: "service:proxy", Type: "service", Name: "Zoraxy", Fields: map[string]any{"host_id": "host:vm"}},
		{ID: "service:consumer", Type: "service", Name: "相册客户端", Fields: map[string]any{"host_id": "host:other"}},
		{ID: "dependency:consumer", Type: "dependency", Name: "依赖", Fields: map[string]any{"from_id": "service:consumer", "to_id": "service:app", "kind": "依赖"}},
		{ID: "endpoint:app", Type: "endpoint", Name: "相册入口", Fields: map[string]any{"service_id": "service:app", "kind": "反向代理", "url": "https://photos.invalid", "via_json": []string{"service:proxy"}}},
		{ID: "endpoint:consumer", Type: "endpoint", Name: "客户端入口", Fields: map[string]any{"service_id": "service:consumer", "kind": "内网直连", "url": "https://client.invalid"}},
	}
	for _, asset := range assets {
		if _, err := service.CreateAsset(ctx, owner, asset, 0, "初始化", "web"); err != nil {
			t.Fatal(asset.ID, err)
		}
	}
	q := catalog.Query{Service: service}
	impact, err := q.Impact(ctx, owner, "host", "host:proxmox", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(impact.Hosts) != 2 || impact.Hosts[0].Object.ID != "host:container" || impact.Hosts[1].Object.ID != "host:vm" {
		t.Fatalf("hosts: %+v", impact.Hosts)
	}
	if len(impact.Services) != 3 || len(impact.Endpoints) != 2 {
		t.Fatalf("unexpected impact: %+v", impact)
	}
	if len(impact.Hosts[0].Path) != 3 {
		t.Fatalf("path: %+v", impact.Hosts[0])
	}
	preview, err := service.OfflinePreview(ctx, owner, "host:proxmox")
	if err != nil || len(preview.Impact.Services) != len(impact.Services) || len(preview.Services) != 0 {
		t.Fatalf("preview: %+v %v", preview, err)
	}
	serviceImpact, err := q.Impact(ctx, owner, "service", "service:app", true)
	if err != nil || len(serviceImpact.Services) != 1 || serviceImpact.Services[0].Object.ID != "service:consumer" || len(serviceImpact.Endpoints) != 2 {
		t.Fatalf("service impact: %+v %v", serviceImpact, err)
	}
	proxyImpact, err := q.Impact(ctx, owner, "service", "service:proxy", true)
	if err != nil || len(proxyImpact.Endpoints) != 1 || proxyImpact.Endpoints[0].Object.ID != "endpoint:app" {
		t.Fatalf("via impact: %+v %v", proxyImpact, err)
	}
	// 模拟导入数据中的环：遍历应终止，且不把根对象计入影响面。
	if _, err = d.Exec(`UPDATE hosts SET parent_host_id='host:container' WHERE id='host:proxmox'`); err != nil {
		t.Fatal(err)
	}
	cyclic, err := q.Impact(ctx, owner, "host", "host:proxmox", true)
	if err != nil || len(cyclic.Hosts) != 2 {
		t.Fatalf("cycle: %+v %v", cyclic, err)
	}
	if _, err = q.Impact(ctx, auth.Principal{}, "host", "host:proxmox", true); !errors.Is(err, auth.ErrUnauthorized) {
		t.Fatalf("authorization: %v", err)
	}
}
