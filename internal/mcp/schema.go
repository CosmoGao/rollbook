package mcp

import (
	"sort"
	"strings"

	"github.com/CosmoGao/rollbook/internal/buildinfo"
	"github.com/CosmoGao/rollbook/internal/catalog"
	"github.com/CosmoGao/rollbook/internal/httpapi"
)

// version comes from the same Go build metadata as the running program.
func programVersion() string {
	v, _ := buildinfo.Info()
	return v
}

var descriptions = map[string]string{
	"search":          "按名称、IP、域名或别名查找资产；先搜索，再用 ID 获取详情。仅返回可见的清册数据。",
	"get_asset":       "已知资产 ID 时读取完整对象及 rev；修改前必须先调用。",
	"get_service":     "查看服务、宿主、入口、依赖及版本；排查服务时使用。",
	"list_by_host":    "查看主机下的 IP 和服务；已知主机 ID 时使用。",
	"list_by_network": "查看网段内的 IP 和主机；已知网段 ID 时使用。",
	"get_site":        "查看站点、网段、主机和上游；已知站点 ID 时使用。",
	"topology":        "查看服务或主机拓扑；id 为 service: 或 host:，view=access 查看入口到宿主的访问链，depth 可指定依赖层数。",
	"list_outdated":   "列出落后或待人工确认的版本；结果不是自动升级建议。",
	"get_changes":     "查看审计和待审批变更；用 state 筛选待确认提议。",
	"get_context":     "读取补充上下文；archived=true 才包含归档记录。",
	"put_context":     "新增或更新上下文；更新时先读取 rev，并提供 reason。不要写秘密。",
	"record_note":     "为资产记录笔记；先读取目标 rev，写清 reason，不写口令或密钥。",
	"propose_change":  "提交待网页用户审批的变更；提议级 key 使用，创建时 action=create、base_rev=0。",
	"create":          "新建资产；直接写 key 立即生效，提议级 key 产生待确认提议，只读 key 被拒绝。",
	"update":          "直接修改资产；先读取最新 rev，冲突时重读后再决定，禁止盲目覆盖。",
	"mark_verified":   "标记已人工核实；先读取最新 rev，reason 应写核实依据。",
	"check_versions":  "手动检查配置了公开上游的服务版本；仅直接写 key 或网页用户可调用。",
}

var fieldsByType = map[string][]string{
	"location": {}, "uplink": {}, "host": {}, "network": {}, "ip": {}, "service": {}, "port": {}, "endpoint": {}, "dependency": {}, "version": {}, "credential": {}, "note": {}, "context": {},
}
var requiredByType = map[string][]string{
	"location": {"kind"}, "uplink": {"location_id"}, "host": {"kind", "location_id"}, "network": {"cidr", "location_id"}, "ip": {"address", "host_id"}, "service": {"host_id"}, "port": {"service_id", "port", "protocol"}, "endpoint": {"service_id", "kind", "url"}, "dependency": {"from_id", "to_id", "kind"}, "version": {"service_id"}, "credential": {"target_id", "vault", "item_ref"}, "note": {"target_id", "kind", "body_md"},
}
var enums = map[string][]string{
	"status": {"在用", "停用", "计划中"}, "source": {"人工", "agent", "自动发现"}, "confidence": {"低", "中", "高"}, "protocol": {"tcp", "udp"}, "assign": {"静态", "DHCP保留", "动态"}, "net_role": {"光猫", "主路由", "旁路由", "交换机", "AP"}, "public_ipv4": {"无或CGNAT", "动态公网", "固定公网", "待核实"},
}
var kindEnums = map[string][]string{
	"location": {"站点", "房间", "机柜", "桌面"}, "host": {"物理机", "NAS", "虚拟机", "LXC", "容器", "网络设备"}, "endpoint": {"内网直连", "反向代理", "Cloudflare Tunnel", "导航页卡片"}, "note": {"运维笔记", "故障", "决策", "待办"},
}
var numberFields = map[string]bool{"port": true, "vlan_id": true, "bandwidth": true, "pinned": true, "generated": true}
var arrayFields = map[string]bool{"via_json": true, "related_ids_json": true, "inbound_ports_json": true}

func fieldSchema(name, typ string) map[string]any {
	p := map[string]any{"type": "string", "description": "清册业务字段；不存储秘密"}
	if strings.HasSuffix(name, "_json") {
		p = map[string]any{"type": "object", "description": "JSON 结构，按该对象类型提供内容"}
	}
	if arrayFields[name] {
		p = map[string]any{"type": "array", "items": map[string]any{"type": "string"}}
	}
	if numberFields[name] {
		p = map[string]any{"type": "integer"}
	}
	if strings.HasSuffix(name, "_id") || name == "from_id" || name == "to_id" || name == "target_id" {
		p["description"] = "已存在对象的 ID；先搜索确认关联对象"
		p["examples"] = []string{"location:example"}
	}
	if name == "kind" {
		if values := kindEnums[typ]; len(values) > 0 {
			p["enum"] = withAliases(name, typ, values)
		} else if typ == "" {
			p["description"] = "按对象类型填写中文值或英文别名：location site/room/rack/desk；host physical/nas/virtual/vm/lxc/container/network_device；endpoint direct/reverse_proxy/cloudflare_tunnel/navigation_card；note operations/incident/decision/todo"
		}
	}
	if values := enums[name]; len(values) > 0 {
		p["enum"] = withAliases(name, typ, values)
	}
	if name == "cidr" {
		p["examples"] = []string{"192.0.2.0/24"}
	}
	if name == "address" {
		p["examples"] = []string{"192.0.2.10"}
	}
	if name == "url" {
		p["examples"] = []string{"https://example.invalid"}
	}
	return p
}
func objectSchema() map[string]any {
	branches := make([]any, 0, len(fieldsByType))
	for typ := range fieldsByType {
		fields := map[string]any{}
		for _, name := range catalog.EditableFields(typ) {
			if name == "generated" {
				continue
			}
			fields[name] = fieldSchema(name, typ)
		}
		required := requiredByType[typ]
		if required == nil {
			required = []string{}
		}
		branches = append(branches, map[string]any{"type": "object", "properties": map[string]any{
			"id":   map[string]any{"type": "string", "description": "唯一 ID，前缀必须与 type 一致", "examples": []string{typ + ":example"}},
			"type": map[string]any{"const": typ}, "name": map[string]any{"type": "string", "examples": []string{"示例资产"}},
			"aliases": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "tags": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"status": fieldSchema("status", typ), "source": fieldSchema("source", typ), "confidence": fieldSchema("confidence", typ),
			"fields": map[string]any{"type": "object", "properties": fields, "required": required, "additionalProperties": false},
		}, "required": []string{"id", "type", "name", "fields"}, "additionalProperties": false})
	}
	return map[string]any{"oneOf": branches, "description": "仅提交业务字段；rev 与审计字段由服务端生成"}
}
func patchSchema() map[string]any {
	fields := map[string]any{}
	for typ := range fieldsByType {
		for _, name := range catalog.EditableFields(typ) {
			if name != "generated" {
				fields[name] = fieldSchema(name, typ)
			}
		}
	}
	return map[string]any{"type": "object", "description": "只写要修改的字段；fields 中字段按对象类型约束", "properties": map[string]any{
		"name": map[string]any{"type": "string"}, "aliases": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "tags": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		"status": fieldSchema("status", ""), "source": fieldSchema("source", ""), "confidence": fieldSchema("confidence", ""),
		"fields": map[string]any{"type": "object", "properties": fields, "additionalProperties": false},
	}, "additionalProperties": false, "examples": []any{map[string]any{"name": "新名称", "fields": map[string]any{"location_id": "location:example"}}}}
}

// Schema 列出可接受的入参；响应仍为中文规范值。
func withAliases(name, typ string, values []string) []string {
	out := append([]string(nil), values...)
	key := name
	if name == "kind" {
		key = "kind:" + typ
	}
	for alias := range httpapi.EnumAliases(key) {
		out = append(out, alias)
	}
	sort.Strings(out)
	return out
}
