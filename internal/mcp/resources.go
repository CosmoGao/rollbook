package mcp

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/CosmoGao/rollbook/internal/httpapi"
)

const rulesText = "先搜索并读取对象 ID 与最新 rev；修改时提供具体 reason。只读 key 只能查询，提议级 key 的改动须网页审批。遇到 rev 冲突先重读，不覆盖他人更改。不得记录密码、私钥或 token；凭据只记录密码管理器中的引用。"

func writeResult(w http.ResponseWriter, id json.RawMessage, result any) {
	httpapi.WriteJSON(w, http.StatusOK, response{JSONRPC: "2.0", ID: id, Result: result})
}

// 只接受规范 URI，避免路径解码后绕过资源类型或对象 ID 检查。
func resourceRoute(raw string) (operation, id string, ok bool) {
	if raw == "rollbook://rules" {
		return "rules", "", true
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "rollbook" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Port() != "" || u.RawPath != "" {
		return "", "", false
	}
	switch u.Host {
	case "sites":
		operation = "get_site"
	case "services":
		operation = "get_service"
	default:
		return "", "", false
	}
	if !strings.HasPrefix(u.Path, "/") || strings.Count(u.Path, "/") != 1 {
		return "", "", false
	}
	id = strings.TrimPrefix(u.Path, "/")
	if id == "" || strings.ContainsAny(id, "?#\\") || strings.Contains(id, "%") || strings.Contains(id, "..") {
		return "", "", false
	}
	if u.String() != raw {
		return "", "", false
	}
	return operation, id, true
}

func resourceError(w http.ResponseWriter, id json.RawMessage, err error) {
	status, _ := httpapi.Error(err)
	switch status {
	case http.StatusNotFound:
		rpcError(w, id, -32002, "Resource not found")
	case http.StatusForbidden, http.StatusUnauthorized:
		rpcError(w, id, -32003, "Permission denied")
	case http.StatusUnprocessableEntity:
		rpcError(w, id, -32602, "Invalid resource URI")
	default:
		rpcError(w, id, -32603, "Internal error")
	}
}

func promptList() []any {
	arg := func(name, description string) map[string]any {
		return map[string]any{"name": name, "description": description, "required": true}
	}
	return []any{
		map[string]any{"name": "register_service", "description": "登记新服务", "arguments": []any{arg("name", "服务名称")}},
		map[string]any{"name": "retire_host", "description": "下线主机", "arguments": []any{arg("host_id", "主机 ID")}},
		map[string]any{"name": "handoff_context", "description": "交接时写共享上下文", "arguments": []any{arg("topic", "交接主题")}},
	}
}

func promptMessages(name string, args httpapi.Args) ([]any, bool) {
	var key, instruction string
	switch name {
	case "register_service":
		key = "name"
		instruction = "先 search 确认服务未登记，查明站点、宿主、入口和依赖的对象 ID。与用户核实事实和凭据引用，再按引用顺序 create；提供 base_rev=0 和具体 reason。提议级 key 的新建须等待网页审批。不要记录秘密。"
	case "retire_host":
		key = "host_id"
		instruction = "先 get_asset 和 list_by_host 检查主机、IP、服务及入口，列出影响并与用户确认。按现有下线流程处理，保留审计和历史；每次修改使用最新 rev 与具体 reason。只读 key 不得写入，提议级 key 须等待网页审批。"
	case "handoff_context":
		key = "topic"
		instruction = "先 get_context 检查已有交接信息，整理现状、依据、待办、负责人和有效期。写入时使用 put_context、最新 rev 与具体 reason；不得包含密码、私钥、token 或连接字符串。只读 key 不得写入。"
	default:
		return nil, false
	}
	if len(args) != 1 || len(args[key]) == 0 {
		return nil, false
	}
	var value string
	if json.Unmarshal(args[key], &value) != nil || strings.TrimSpace(value) == "" || len(value) > 200 {
		return nil, false
	}
	return []any{map[string]any{"role": "user", "content": map[string]string{"type": "text", "text": name + "（" + key + "：" + value + "）。" + instruction}}}, true
}
