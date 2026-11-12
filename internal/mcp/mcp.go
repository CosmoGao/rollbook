// Package mcp implements the stateless JSON response mode of Streamable HTTP.
package mcp

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/CosmoGao/rollbook/internal/buildinfo"
	"github.com/CosmoGao/rollbook/internal/httpapi"
)

type Server struct{ Engine httpapi.Engine }
type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  struct {
		Name      string       `json:"name"`
		URI       string       `json:"uri"`
		Arguments httpapi.Args `json:"arguments"`
	} `json:"params"`
}
type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   any             `json:"error,omitempty"`
}

func (s Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	p, err := s.Engine.Principal(r)
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	body := http.MaxBytesReader(w, r.Body, 1<<20)
	defer body.Close()
	var req request
	dec := json.NewDecoder(body)
	if err := dec.Decode(&req); err != nil {
		rpcError(w, nil, -32700, "Invalid JSON")
		return
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		rpcError(w, req.ID, -32600, "Invalid request")
		return
	}
	if req.JSONRPC != "2.0" {
		rpcError(w, req.ID, -32600, "Invalid request")
		return
	}
	if len(req.ID) == 0 {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	switch req.Method {
	case "initialize":
		_, commit := buildinfo.Info()
		httpapi.WriteJSON(w, 200, response{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"protocolVersion": "2025-03-26", "capabilities": map[string]any{"tools": map[string]any{}, "resources": map[string]any{}, "prompts": map[string]any{}}, "instructions": "先 search 再 get_asset 读取 ID 与 rev；创建用 create(object,base_rev=0,reason)，提议级 key 需网页审批。更新先读取最新 rev，冲突时重读，禁止盲目覆盖。只读 key 不可写；不要保存密码、私钥、token，只保存凭据管理器的引用。每次写入给出具体 reason。", "serverInfo": map[string]string{"name": "rollbook", "version": programVersion(), "commit": commit}}})
	case "ping":
		httpapi.WriteJSON(w, 200, response{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{}})
	case "tools/list":
		tools := make([]map[string]any, 0, len(httpapi.Tools))
		for _, name := range httpapi.Tools {
			tools = append(tools, map[string]any{"name": name, "description": descriptions[name], "inputSchema": schema(name)})
		}
		httpapi.WriteJSON(w, 200, response{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"tools": tools}})
	case "resources/list":
		writeResult(w, req.ID, map[string]any{"resources": []any{map[string]string{"uri": "rollbook://rules", "name": "Rollbook 使用规则", "mimeType": "text/plain", "description": "查询、写入和秘密处理规则"}}})
	case "resources/templates/list":
		writeResult(w, req.ID, map[string]any{"resourceTemplates": []any{
			map[string]string{"uriTemplate": "rollbook://sites/{id}", "name": "站点概览", "mimeType": "application/json", "description": "站点 ID，须先搜索或用 get_asset 确认"},
			map[string]string{"uriTemplate": "rollbook://services/{id}", "name": "服务全貌", "mimeType": "application/json", "description": "服务 ID，包含宿主、入口、依赖和凭据引用"},
		}})
	case "resources/read":
		op, id, ok := resourceRoute(req.Params.URI)
		if !ok {
			rpcError(w, req.ID, -32602, "Invalid resource URI")
			return
		}
		if op == "rules" {
			writeResult(w, req.ID, map[string]any{"contents": []any{map[string]string{"uri": req.Params.URI, "mimeType": "text/plain", "text": rulesText}}})
			return
		}
		result, err := s.Engine.Call(r.Context(), p, op, httpapi.Args{"id": httpapi.StringArg(id)}, "mcp")
		if err != nil {
			resourceError(w, req.ID, err)
			return
		}
		data, err := json.Marshal(result)
		if err != nil {
			rpcError(w, req.ID, -32603, "Internal error")
			return
		}
		writeResult(w, req.ID, map[string]any{"contents": []any{map[string]string{"uri": req.Params.URI, "mimeType": "application/json", "text": string(data)}}})
	case "prompts/list":
		writeResult(w, req.ID, map[string]any{"prompts": promptList()})
	case "prompts/get":
		messages, ok := promptMessages(req.Params.Name, req.Params.Arguments)
		if !ok {
			rpcError(w, req.ID, -32602, "Invalid prompt name or arguments")
			return
		}
		writeResult(w, req.ID, map[string]any{"messages": messages})
	case "tools/call":
		valid := false
		for _, name := range httpapi.Tools {
			if req.Params.Name == name {
				valid = true
				break
			}
		}
		if !valid {
			rpcError(w, req.ID, -32602, "Unknown tool")
			return
		}
		result, err := s.Engine.Call(r.Context(), p, req.Params.Name, req.Params.Arguments, "mcp")
		if err != nil {
			status, apiErr := httpapi.Error(err)
			_ = status
			b, _ := json.Marshal(apiErr)
			httpapi.WriteJSON(w, 200, response{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"content": []any{map[string]string{"type": "text", "text": string(b)}}, "isError": true, "code": apiErr.Code, "structuredContent": apiErr}})
			return
		}
		data, _ := json.Marshal(result)
		httpapi.WriteJSON(w, 200, response{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"content": []any{map[string]string{"type": "text", "text": string(data)}}, "structuredContent": result, "isError": false}})
	default:
		rpcError(w, req.ID, -32601, "Method not found")
	}
}

func rpcError(w http.ResponseWriter, id json.RawMessage, code int, message string) {
	httpapi.WriteJSON(w, 200, response{JSONRPC: "2.0", ID: id, Error: map[string]any{"code": code, "message": message}})
}
func schema(name string) map[string]any {
	props := map[string]any{}
	required := []string{}
	add := func(key, typ string, req bool) {
		props[key] = map[string]string{"type": typ}
		if req {
			required = append(required, key)
		}
	}
	switch name {
	case "search":
		add("q", "string", true)
	case "get_asset", "get_service", "list_by_host", "list_by_network", "get_site", "topology", "put_context", "record_note", "propose_change", "update", "mark_verified":
		add("id", "string", true)
	}
	switch name {
	case "put_context", "record_note", "update", "mark_verified":
		add("rev", "integer", true)
		add("reason", "string", true)
	case "propose_change":
		add("rev", "integer", false)
		add("reason", "string", true)
	}
	if name == "update" || name == "propose_change" {
		props["patch"] = patchSchema()
	}
	if name == "propose_change" {
		props["action"] = map[string]any{"type": "string", "enum": []string{"create", "update", "delete"}}
		props["object"] = objectSchema()
		add("base_rev", "integer", false)
	}
	if name == "create" {
		props["object"] = objectSchema()
		required = append(required, "object")
		add("base_rev", "integer", true)
		add("reason", "string", true)
	}
	if name == "put_context" {
		add("body_md", "string", false)
		add("expires_at", "string", false)
		props["object"] = objectSchema()
	}
	if name == "record_note" {
		add("kind", "string", true)
		add("body_md", "string", true)
	}
	if name == "topology" {
		add("depth", "integer", false)
		props["view"] = map[string]any{"type": "string", "enum": []string{"graph", "access"}, "description": "graph 为拓扑，access 为入口到物理机的访问链"}
	}
	if name == "get_context" {
		add("archived", "boolean", false)
	}
	if name == "get_changes" {
		add("object_id", "string", false)
		add("state", "string", false)
	}
	result := map[string]any{"type": "object", "properties": props, "required": required, "additionalProperties": false}
	if name == "propose_change" {
		result["anyOf"] = []any{map[string]any{"required": []string{"action", "object", "base_rev"}}, map[string]any{"required": []string{"id", "rev", "patch"}}}
	}
	return result
}
