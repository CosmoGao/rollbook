package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

func runCLI(args []string) error {
	if len(args) < 1 {
		return errors.New("usage: rollbook search <query>|get <id>|update <id> <rev> <patch-json> <reason>")
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
		return errors.New("usage: rollbook search|get|update")
	}
	req, err := http.NewRequest(method, base+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: 15 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return err
	}
	if res.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(data)))
	}
	_, err = os.Stdout.Write(data)
	return err
}
