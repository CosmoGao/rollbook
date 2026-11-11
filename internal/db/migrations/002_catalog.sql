CREATE TABLE objects (
 id TEXT PRIMARY KEY, type TEXT NOT NULL, name TEXT NOT NULL,
 aliases_json TEXT NOT NULL DEFAULT '[]' CHECK(json_valid(aliases_json) AND json_type(aliases_json)='array'),
 tags_json TEXT NOT NULL DEFAULT '[]' CHECK(json_valid(tags_json) AND json_type(tags_json)='array'),
 status TEXT NOT NULL DEFAULT '在用' CHECK(status IN ('在用','停用','计划中')),
 source TEXT NOT NULL DEFAULT '人工' CHECK(source IN ('人工','agent','自动发现')),
 confidence TEXT NOT NULL DEFAULT '低' CHECK(confidence IN ('高','中','低')),
 verified_at TEXT, verified_by TEXT, rev INTEGER NOT NULL DEFAULT 1 CHECK(rev>0),
 created_at TEXT NOT NULL, created_by TEXT NOT NULL, updated_at TEXT NOT NULL, updated_by TEXT NOT NULL,
 CHECK(type IN ('location','uplink','host','network','ip','service','port','endpoint','dependency','version','credential','note','context','change'))
);
CREATE INDEX objects_type_status_verified ON objects(type,status,verified_at);
CREATE TABLE locations (id TEXT PRIMARY KEY REFERENCES objects(id) ON DELETE RESTRICT, kind TEXT NOT NULL CHECK(kind IN ('站点','房间','机柜','桌面')), parent_id TEXT REFERENCES locations(id) ON DELETE RESTRICT, city TEXT, region TEXT, address TEXT, timezone TEXT, description TEXT);
CREATE INDEX locations_parent ON locations(parent_id);
CREATE TABLE uplinks (id TEXT PRIMARY KEY REFERENCES objects(id) ON DELETE RESTRICT, location_id TEXT NOT NULL REFERENCES locations(id) ON DELETE RESTRICT, isp TEXT, type TEXT CHECK(type IN ('光纤宽带','专线','4G/5G','其他')), bandwidth TEXT, public_ipv4 TEXT CHECK(public_ipv4 IN ('无或CGNAT','动态公网','固定公网','待核实')), public_ipv4_addr TEXT, public_ipv6 TEXT, ddns TEXT, inbound_ports_json TEXT NOT NULL DEFAULT '[]' CHECK(json_valid(inbound_ports_json)), edge_device_id TEXT REFERENCES hosts(id) ON DELETE RESTRICT, account_ref TEXT REFERENCES credentials(id) ON DELETE RESTRICT);
CREATE INDEX uplinks_location ON uplinks(location_id);
CREATE TABLE hosts (id TEXT PRIMARY KEY REFERENCES objects(id) ON DELETE RESTRICT, kind TEXT NOT NULL CHECK(kind IN ('物理机','NAS','虚拟机','LXC','容器','网络设备')), net_role TEXT CHECK(net_role IN ('光猫','主路由','旁路由','交换机','AP')), uplink_id TEXT REFERENCES uplinks(id) ON DELETE RESTRICT, parent_host_id TEXT REFERENCES hosts(id) ON DELETE RESTRICT, location_id TEXT NOT NULL REFERENCES locations(id) ON DELETE RESTRICT, model TEXT, os TEXT, os_version TEXT, resources_json TEXT NOT NULL DEFAULT '{}' CHECK(json_valid(resources_json)), mgmt_url TEXT);
CREATE INDEX hosts_location ON hosts(location_id);
CREATE INDEX hosts_parent ON hosts(parent_host_id);
CREATE TABLE networks (id TEXT PRIMARY KEY REFERENCES objects(id) ON DELETE RESTRICT, cidr TEXT NOT NULL, location_id TEXT NOT NULL REFERENCES locations(id) ON DELETE RESTRICT, router_id TEXT REFERENCES hosts(id) ON DELETE RESTRICT, uplink_id TEXT REFERENCES uplinks(id) ON DELETE RESTRICT, vlan_id INTEGER, gateway TEXT, dhcp_range TEXT, purpose TEXT, UNIQUE(location_id,cidr));
CREATE INDEX networks_location ON networks(location_id);
CREATE TABLE ips (id TEXT PRIMARY KEY REFERENCES objects(id) ON DELETE RESTRICT, address TEXT NOT NULL UNIQUE, network_id TEXT NOT NULL REFERENCES networks(id) ON DELETE RESTRICT, host_id TEXT NOT NULL REFERENCES hosts(id) ON DELETE RESTRICT, interface TEXT, mac TEXT, assign TEXT CHECK(assign IN ('静态','DHCP保留','动态')));
CREATE INDEX ips_host ON ips(host_id);
CREATE INDEX ips_network ON ips(network_id);
CREATE TABLE services (id TEXT PRIMARY KEY REFERENCES objects(id) ON DELETE RESTRICT, host_id TEXT NOT NULL REFERENCES hosts(id) ON DELETE RESTRICT, kind TEXT, deploy_method TEXT, upstream_json TEXT CHECK(upstream_json IS NULL OR json_valid(upstream_json)), owner TEXT, config_path TEXT, docs_url TEXT);
CREATE INDEX services_host ON services(host_id);
CREATE TABLE ports (id TEXT PRIMARY KEY REFERENCES objects(id) ON DELETE RESTRICT, service_id TEXT NOT NULL REFERENCES services(id) ON DELETE RESTRICT, bind TEXT, port INTEGER NOT NULL CHECK(port BETWEEN 1 AND 65535), protocol TEXT NOT NULL CHECK(protocol IN ('tcp','udp')), purpose TEXT);
CREATE INDEX ports_number ON ports(port);
CREATE INDEX ports_service ON ports(service_id);
CREATE TABLE endpoints (id TEXT PRIMARY KEY REFERENCES objects(id) ON DELETE RESTRICT, service_id TEXT NOT NULL REFERENCES services(id) ON DELETE RESTRICT, kind TEXT NOT NULL CHECK(kind IN ('内网直连','反向代理','Cloudflare Tunnel','导航页卡片')), url TEXT NOT NULL, via_json TEXT NOT NULL DEFAULT '[]' CHECK(json_valid(via_json) AND json_type(via_json)='array'), auth TEXT, exposure TEXT, nav_card_json TEXT CHECK(nav_card_json IS NULL OR json_valid(nav_card_json)));
CREATE INDEX endpoints_service ON endpoints(service_id);
CREATE TABLE dependencies (id TEXT PRIMARY KEY REFERENCES objects(id) ON DELETE RESTRICT, from_id TEXT NOT NULL REFERENCES objects(id) ON DELETE RESTRICT, to_id TEXT NOT NULL REFERENCES objects(id) ON DELETE RESTRICT, kind TEXT NOT NULL, note TEXT, generated INTEGER NOT NULL DEFAULT 0 CHECK(generated IN (0,1)), UNIQUE(from_id,to_id,kind), CHECK(from_id<>to_id));
CREATE INDEX dependencies_to ON dependencies(to_id);
CREATE TABLE versions (id TEXT PRIMARY KEY REFERENCES objects(id) ON DELETE RESTRICT, service_id TEXT NOT NULL UNIQUE REFERENCES services(id) ON DELETE RESTRICT, current_version TEXT, current_source TEXT, latest_version TEXT, latest_source TEXT, latest_url TEXT, eol_date TEXT, pinned INTEGER NOT NULL DEFAULT 0 CHECK(pinned IN (0,1)), pinned_reason TEXT, checked_at TEXT);
CREATE TABLE credentials (id TEXT PRIMARY KEY REFERENCES objects(id) ON DELETE RESTRICT, target_id TEXT NOT NULL REFERENCES objects(id) ON DELETE RESTRICT, vault TEXT NOT NULL, item_ref TEXT NOT NULL, purpose TEXT);
CREATE INDEX credentials_target ON credentials(target_id);
CREATE TABLE notes (id TEXT PRIMARY KEY REFERENCES objects(id) ON DELETE RESTRICT, target_id TEXT NOT NULL REFERENCES objects(id) ON DELETE RESTRICT, kind TEXT NOT NULL CHECK(kind IN ('运维笔记','故障','决策','待办')), body_md TEXT NOT NULL, author TEXT NOT NULL);
CREATE INDEX notes_target ON notes(target_id);
CREATE TRIGGER notes_no_update BEFORE UPDATE ON notes BEGIN SELECT RAISE(ABORT,'notes are append-only'); END;
CREATE TABLE contexts (id TEXT PRIMARY KEY REFERENCES objects(id) ON DELETE RESTRICT, scope TEXT NOT NULL CHECK(scope IN ('会话','项目','全局')), project TEXT, key TEXT NOT NULL, body_md TEXT NOT NULL, owner TEXT NOT NULL, related_ids_json TEXT NOT NULL DEFAULT '[]' CHECK(json_valid(related_ids_json) AND json_type(related_ids_json)='array'), expires_at TEXT, archived_at TEXT);
CREATE INDEX contexts_expiry ON contexts(expires_at,archived_at);
CREATE TABLE changes (id TEXT PRIMARY KEY REFERENCES objects(id) ON DELETE RESTRICT, object_type TEXT NOT NULL, object_id TEXT NOT NULL, action TEXT NOT NULL, diff_json TEXT NOT NULL CHECK(json_valid(diff_json)), actor TEXT NOT NULL, actor_type TEXT, actor_key_id TEXT, actor_key_name TEXT, actor_issuer_id TEXT, actor_issuer_name TEXT, via TEXT NOT NULL, reason TEXT NOT NULL CHECK(length(trim(reason))>0), state TEXT NOT NULL CHECK(state IN ('已生效','待确认','已驳回')), base_rev INTEGER, reviewed_by TEXT, reviewed_at TEXT);
CREATE INDEX changes_object ON changes(object_id,id);
