package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

// mcp forwards newline-delimited MCP JSON-RPC between stdio and the HTTP endpoint.
func (r cliRunner) mcp(base, key string) error {
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	var mu sync.Mutex
	var wg sync.WaitGroup
	active := map[string]context.CancelFunc{}
	write := func(v any) {
		mu.Lock()
		defer mu.Unlock()
		if err := json.NewEncoder(r.out).Encode(v); err != nil {
			fmt.Fprintln(r.err, "MCP 写入失败:", err)
			stop()
		}
	}
	scan := bufio.NewScanner(r.in)
	scan.Buffer(make([]byte, 4096), 1<<20)
	for scan.Scan() {
		line := append([]byte(nil), scan.Bytes()...)
		var req struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Method  string          `json:"method"`
			Params  struct {
				RequestID json.RawMessage `json:"requestId"`
			} `json:"params"`
		}
		if err := json.Unmarshal(line, &req); err != nil || req.JSONRPC != "2.0" {
			write(map[string]any{"jsonrpc": "2.0", "id": nil, "error": map[string]any{"code": -32700, "message": "Invalid JSON-RPC"}})
			continue
		}
		if req.Method == "notifications/cancelled" {
			mu.Lock()
			cancel := active[string(req.Params.RequestID)]
			mu.Unlock()
			if cancel != nil {
				cancel()
			}
			continue
		}
		if len(req.ID) == 0 { // initialized and other notifications need no response.
			continue
		}
		id := string(req.ID)
		callCtx, cancel := context.WithCancel(ctx)
		mu.Lock()
		active[id] = cancel
		mu.Unlock()
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer cancel()
			defer func() {
				mu.Lock()
				delete(active, id)
				mu.Unlock()
			}()
			httpReq, err := http.NewRequestWithContext(callCtx, http.MethodPost, base+"/mcp", bytes.NewReader(line))
			if err == nil {
				httpReq.Header.Set("Authorization", "Bearer "+key)
				httpReq.Header.Set("Content-Type", "application/json")
				httpReq.Header.Set("Accept", "application/json, text/event-stream")
				var resp *http.Response
				resp, err = r.client.Do(httpReq)
				if err == nil {
					defer resp.Body.Close()
					var body []byte
					body, err = io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
					if err == nil {
						if len(body) > 1<<20 {
							err = fmt.Errorf("MCP 响应过大")
						} else if resp.StatusCode != 200 || !strings.Contains(resp.Header.Get("Content-Type"), "application/json") || !json.Valid(body) {
							err = fmt.Errorf("远端 MCP 返回 HTTP %d 或无效 JSON", resp.StatusCode)
						} else {
							var reply struct {
								ID json.RawMessage `json:"id"`
							}
							if json.Unmarshal(body, &reply) != nil || string(reply.ID) != id {
								err = fmt.Errorf("远端 MCP 响应 ID 不匹配")
							} else if callCtx.Err() == nil {
								write(json.RawMessage(body))
								return
							}
						}
					}
				}
			}
			if callCtx.Err() == nil {
				fmt.Fprintln(r.err, "MCP 转发失败:", err)
				write(map[string]any{"jsonrpc": "2.0", "id": req.ID, "error": map[string]any{"code": -32000, "message": "远端 MCP 不可用"}})
			}
		}()
	}
	wg.Wait()
	stop()
	if err := scan.Err(); err != nil {
		return fmt.Errorf("MCP stdin: %w", err)
	}
	return nil
}
