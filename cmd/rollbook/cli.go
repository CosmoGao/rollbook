package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

func runCLI(args []string) error {
	if len(args) < 1 {
		return errors.New("usage: rollbook search <query>|get <id>|create <object-json> <reason>|update <id> <rev> <patch-json> <reason>|export|import <file.yaml>")
	}
	base := strings.TrimRight(os.Getenv("ROLLBOOK_URL"), "/")
	if base == "" {
		base = "http://127.0.0.1:8080"
	}
	key := os.Getenv("ROLLBOOK_KEY")
	if key == "" {
		return errors.New("ROLLBOOK_KEY is required")
	}
	method, path := http.MethodGet, ""
	var body []byte
	switch args[0] {
	case "create":
		if len(args) != 3 {
			return errors.New("usage: rollbook create <object-json> <reason>")
		}
		var object json.RawMessage = []byte(args[1])
		if !json.Valid(object) {
			return errors.New("invalid object JSON")
		}
		body, _ = json.Marshal(map[string]any{"object": object, "base_rev": 0, "reason": args[2]})
		method, path = http.MethodPost, "/api/assets"
	case "search":
		if len(args) != 2 {
			return errors.New("usage: rollbook search <query>")
		}
		path = "/api/search?q=" + url.QueryEscape(args[1])
	case "get":
		if len(args) != 2 {
			return errors.New("usage: rollbook get <id>")
		}
		path = "/api/assets/" + url.PathEscape(args[1])
	case "export":
		if len(args) != 1 {
			return errors.New("usage: rollbook export")
		}
		path = "/api/backup/export?kind=catalog"
	case "import":
		if len(args) != 2 {
			return errors.New("usage: rollbook import <file.yaml>")
		}
		var err error
		body, err = os.ReadFile(args[1])
		if err != nil {
			return err
		}
		if len(body) > 16<<20 {
			return errors.New("import file too large")
		}
		var doc map[string]any
		if err = yaml.Unmarshal(body, &doc); err != nil || doc == nil {
			return errors.New("invalid import YAML")
		}
		if kind, ok := doc["kind"]; ok && kind != "merge" {
			return errors.New("CLI 仅支持合并导入；整库恢复请在网页提交")
		}
		doc["kind"] = "merge"
		if _, ok := doc["schema_version"]; !ok {
			doc["schema_version"] = 1
		}
		body, err = yaml.Marshal(doc)
		if err != nil {
			return err
		}
		method, path = http.MethodPost, "/api/backup/import"
	case "update":
		if len(args) != 5 {
			return errors.New("usage: rollbook update <id> <rev> <patch-json> <reason>")
		}
		var patch json.RawMessage = []byte(args[3])
		if !json.Valid(patch) {
			return errors.New("invalid patch JSON")
		}
		var rev int64
		if _, err := fmt.Sscan(args[2], &rev); err != nil || rev < 1 {
			return errors.New("rev must be positive")
		}
		body, _ = json.Marshal(map[string]any{"rev": rev, "patch": patch, "reason": args[4]})
		method = http.MethodPatch
		path = "/api/assets/" + url.PathEscape(args[1])
	default:
		return errors.New("usage: rollbook search|get|create|update|export|import")
	}
	req, err := http.NewRequest(method, base+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	if body != nil {
		if args[0] == "import" {
			req.Header.Set("Content-Type", "application/yaml")
		} else {
			req.Header.Set("Content-Type", "application/json")
		}
	}
	client := &http.Client{Timeout: 15 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 17<<20))
	if err != nil {
		return err
	}
	if res.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(data)))
	}
	_, err = os.Stdout.Write(data)
	if err == nil && args[0] == "import" {
		_, err = fmt.Fprintln(os.Stdout, "合并导入已提交，等待网页用户批准；批准前会重新校验 rev 和关联关系。")
	}
	return err
}
