-- 扩容 hosts.kind，保留原表列和外键。迁移器在专用连接的事务外暂时关闭外键。
CREATE TABLE hosts_v07 (id TEXT PRIMARY KEY REFERENCES objects(id) ON DELETE RESTRICT, kind TEXT NOT NULL CHECK(kind IN ('物理机','NAS','虚拟机','LXC','容器','网络设备','智能电视','游戏机','机顶盒与播放器','音箱与音频设备','摄像头与门铃','打印与扫描设备','智能家居中枢与网关','家电','传感器与控制器','其他设备')), net_role TEXT CHECK(net_role IN ('光猫','主路由','旁路由','交换机','AP')), uplink_id TEXT REFERENCES uplinks(id) ON DELETE RESTRICT, parent_host_id TEXT REFERENCES hosts(id) ON DELETE RESTRICT, location_id TEXT NOT NULL REFERENCES locations(id) ON DELETE RESTRICT, model TEXT, os TEXT, os_version TEXT, resources_json TEXT NOT NULL DEFAULT '{}' CHECK(json_valid(resources_json)), mgmt_url TEXT);
INSERT INTO hosts_v07 SELECT * FROM hosts;
DROP TABLE hosts;
ALTER TABLE hosts_v07 RENAME TO hosts;
CREATE INDEX hosts_location ON hosts(location_id);
CREATE INDEX hosts_parent ON hosts(parent_host_id);
-- 使用现有 description 列索引 ID、型号和类别，避免改变 FTS 表结构。
DELETE FROM search_documents;
INSERT INTO search_documents(object_id,name,aliases,tags,description,url)
 SELECT o.id,o.name,coalesce((SELECT group_concat(value,' ') FROM json_each(o.aliases_json)),''),
 coalesce((SELECT group_concat(value,' ') FROM json_each(o.tags_json)),''),
 trim(o.id||' '||coalesce(l.description,'')||' '||coalesce(h.model,'')||' '||coalesce(h.kind,'')),coalesce(e.url,'')
 FROM objects o LEFT JOIN locations l ON l.id=o.id LEFT JOIN hosts h ON h.id=o.id LEFT JOIN endpoints e ON e.id=o.id;
