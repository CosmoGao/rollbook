package catalog_test

import (
	"github.com/CosmoGao/rollbook/internal/catalog"
	"testing"
)

func TestBrowseAndSearchDerivedContext(t *testing.T) {
	ctx, d, service, _, user := serviceSetup(t)
	const now = "2026-11-12T10:00:00Z"
	add := func(id, typ, name string) {
		t.Helper()
		if _, err := d.Exec(`INSERT INTO objects(id,type,name,created_at,created_by,updated_at,updated_by) VALUES(?,?,?,?,?,?,?)`, id, typ, name, now, "owner", now, "owner"); err != nil {
			t.Fatal(err)
		}
	}
	for _, o := range [][3]string{{"location:bj", "location", "北京家"}, {"location:rack", "location", "主卧机柜"}, {"location:xh", "location", "香河"}, {"host:pve", "host", "Proxmox"}, {"host:immich", "host", "Immich 虚拟机"}, {"network:lan", "network", "业务网"}, {"ip:immich", "ip", "172.20.2.22"}, {"service:immich", "service", "Immich"}, {"endpoint:web", "endpoint", "相册入口"}, {"endpoint:bad", "endpoint", "危险入口"}} {
		add(o[0], o[1], o[2])
	}
	for _, stmt := range []string{
		`INSERT INTO locations(id,kind) VALUES('location:bj','站点'),('location:xh','站点')`,
		`INSERT INTO locations(id,kind,parent_id) VALUES('location:rack','机柜','location:bj')`,
		`INSERT INTO hosts(id,kind,location_id) VALUES('host:pve','物理机','location:rack')`,
		`INSERT INTO hosts(id,kind,location_id,parent_host_id) VALUES('host:immich','虚拟机','location:rack','host:pve')`,
		`INSERT INTO networks(id,cidr,location_id) VALUES('network:lan','172.20.2.0/24','location:rack')`,
		`INSERT INTO ips(id,address,network_id,host_id) VALUES('ip:immich','172.20.2.22','network:lan','host:immich')`,
		`INSERT INTO services(id,host_id) VALUES('service:immich','host:immich')`,
		`INSERT INTO endpoints(id,service_id,kind,url) VALUES('endpoint:web','service:immich','反向代理','https://photos.example.invalid/')`,
		`INSERT INTO endpoints(id,service_id,kind,url) VALUES('endpoint:bad','service:immich','反向代理','javascript:alert(1)')`,
	} {
		if _, err := d.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	q := catalog.Query{Service: service}
	locations, err := q.Browse(ctx, user, "location")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range locations {
		if v.ID == "location:rack" && (v.Kind != "机柜" || v.SiteID != "location:bj") {
			t.Fatalf("rack: %+v", v.Summary)
		}
	}
	hosts, err := q.Browse(ctx, user, "host")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range hosts {
		if v.ID == "host:immich" && (v.SiteID != "location:bj" || v.PrimaryIP != "172.20.2.22" || len(v.Services) != 1) {
			t.Fatalf("host: %+v", v.Summary)
		}
	}
	hits, err := q.Search(ctx, user, "172.20.2.22")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, v := range hits {
		if v.ID != "ip:immich" {
			continue
		}
		found = true
		if v.SiteID != "location:bj" || v.HostName != "Immich 虚拟机" || len(v.Services) != 1 || len(v.Endpoints) != 1 || v.Endpoints[0] != "https://photos.example.invalid/" {
			t.Fatalf("IP: %+v", v.Summary)
		}
	}
	if !found {
		t.Fatal("IP result missing")
	}
}
