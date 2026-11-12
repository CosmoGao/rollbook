package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"unicode"

	"github.com/CosmoGao/rollbook/internal/catalog"
)

// 英文别名仅在协议入口转换，清册内仍使用中文规范值。
var enumAliases = map[string]map[string]string{
	"status":        {"active": "在用", "inactive": "停用", "planned": "计划中"},
	"source":        {"manual": "人工", "auto_discovery": "自动发现"},
	"confidence":    {"low": "低", "medium": "中", "high": "高"},
	"kind:location": {"site": "站点", "room": "房间", "rack": "机柜", "desk": "桌面"},
	"kind:host":     {"physical": "物理机", "nas": "NAS", "virtual": "虚拟机", "vm": "虚拟机", "lxc": "LXC", "container": "容器", "network_device": "网络设备"},
	"kind:endpoint": {"direct": "内网直连", "reverse_proxy": "反向代理", "cloudflare_tunnel": "Cloudflare Tunnel", "navigation_card": "导航页卡片"},
	"kind:note":     {"operations": "运维笔记", "incident": "故障", "decision": "决策", "todo": "待办"},
	"assign":        {"static": "静态", "dhcp_reservation": "DHCP保留", "dynamic": "动态"},
	"net_role":      {"modem": "光猫", "main_router": "主路由", "secondary_router": "旁路由", "switch": "交换机", "ap": "AP"},
	"public_ipv4":   {"none_or_cgnat": "无或CGNAT", "dynamic_public": "动态公网", "static_public": "固定公网", "unverified": "待核实"},
}

func canonical(field, typ string, value any) any {
	s, ok := value.(string)
	if !ok {
		return value
	}
	key := field
	if field == "kind" {
		key = "kind:" + typ
	}
	if v, ok := enumAliases[key][strings.ToLower(s)]; ok {
		return v
	}
	return value
}
func canonicalObject(o map[string]any) {
	typ, _ := o["type"].(string)
	for _, key := range []string{"status", "source", "confidence"} {
		if value, ok := o[key]; ok {
			o[key] = canonical(key, typ, value)
		}
	}
	if fields, ok := o["fields"].(map[string]any); ok {
		canonicalFields(fields, typ)
	}
}
func canonicalFields(fields map[string]any, typ string) {
	for _, key := range []string{"kind", "assign", "net_role", "public_ipv4"} {
		if v, ok := fields[key]; ok {
			fields[key] = canonical(key, typ, v)
		}
	}
}
func canonicalPatch(p map[string]any, typ string) {
	for _, key := range []string{"status", "source", "confidence"} {
		if v, ok := p[key]; ok {
			p[key] = canonical(key, typ, v)
		}
	}
	if fields, ok := p["fields"].(map[string]any); ok {
		canonicalFields(fields, typ)
	}
}

type localizedWriter struct {
	http.ResponseWriter
	language string
}

func requestLanguage(r *http.Request) string {
	value := r.Header.Get("X-Rollbook-Language")
	if value == "" {
		value = r.Header.Get("Accept-Language")
	}
	if strings.HasPrefix(strings.ToLower(value), "en") {
		return "en"
	}
	return "zh-CN"
}

var errorMessages = map[string][2]string{
	"validation": {"校验失败", "Validation failed"}, "unauthorized": {"请先登录", "Authentication required"},
	"forbidden": {"权限不足", "Permission denied"}, "conflict": {"数据冲突，请刷新后重试", "Conflict; refresh and retry"},
	"invalid": {"输入无效", "Invalid input"}, "not_found": {"未找到", "Not found"},
	"internal": {"服务器内部错误", "Internal error"}, "method_not_allowed": {"不支持此请求方法", "Method not allowed"},
}
var reasonMessages = map[string]string{
	"名称必填": "Name is required", "未知对象类型": "Unknown object type", "ID 缺失或类型前缀不符": "ID is missing or has the wrong type prefix",
	"必填字段缺失": "Required field is missing", "关联对象不存在": "Referenced object does not exist",
	"关联对象不存在或仍被引用": "Referenced object does not exist or is still in use",
	"字段值不符合模型约束":   "Value is outside the allowed range", "仅接受绝对 HTTP 或 HTTPS 地址": "Only absolute HTTP or HTTPS URLs are accepted",
	"疑似包含秘密": "Content appears to contain a secret", "创建时必须提供整数版本": "An integer base revision is required for creation",
	"对象必填且必须是有效 JSON": "Object is required and must be valid JSON", "密码至少 12 个字符": "Password must have at least 12 characters", "当前密码不正确": "Current password is incorrect",
}

func hasHan(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}
func localizeError(v any, language string) any {
	e, ok := v.(APIError)
	if !ok {
		return v
	}
	index := 0
	if language == "en" {
		index = 1
	}
	if pair, ok := errorMessages[e.Code]; ok {
		e.Message = pair[index]
	} else if language == "en" && hasHan(e.Message) {
		e.Message = "Request failed"
	}
	if language == "en" {
		fields := append([]catalog.FieldError(nil), e.Fields...)
		for i := range fields {
			if translated, ok := reasonMessages[fields[i].Reason]; ok {
				fields[i].Reason = translated
			} else if hasHan(fields[i].Reason) {
				fields[i].Reason = "Invalid value"
			}
			if hasHan(fields[i].Expected) {
				fields[i].Expected = "Valid value"
			}
		}
		e.Fields = fields
	}
	return e
}

// REST 和 MCP 在调用清册前转换别名；CLI 使用 REST。
func canonicalArgs(op string, args Args) {
	key := ""
	if op == "create" {
		key = "object"
	} else if op == "update" || op == "propose_change" {
		if op == "propose_change" && args.String("action") == "create" {
			key = "object"
		} else {
			key = "patch"
		}
	}
	if op == "record_note" {
		if kind := args.String("kind"); kind != "" {
			args["kind"] = StringArg(canonical("kind", "note", kind).(string))
		}
	}
	if key == "" {
		return
	}
	var value map[string]any
	if json.Unmarshal(args[key], &value) != nil || value == nil {
		return
	}
	if key == "object" {
		canonicalObject(value)
	} else {
		id := args.String("id")
		typ := strings.SplitN(id, ":", 2)[0]
		canonicalPatch(value, typ)
	}
	if encoded, err := json.Marshal(value); err == nil {
		args[key] = encoded
	}
}

// EnumAliases 返回 MCP Schema 所需的协议入参别名。
func EnumAliases(field string) map[string]string { return enumAliases[field] }
