package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

const cliHelp = `用法: rollbook [--profile 名称] [--output table|json|yaml] 命令 [参数]
命令: login search get create update propose note changes topology site versions verify
      context get|put export import completion version
JSON 对象可直接传入或使用 @文件名。运行 rollbook <命令> --help 查看此帮助。`

type cliProfile struct{ URL, Key string }
type cliConfig struct {
	Default  string
	Profiles map[string]cliProfile
}
type cliRunner struct {
	client     *http.Client
	out, err   io.Writer
	in         io.Reader
	configPath string
}

func runCLI(args []string) error {
	r := cliRunner{client: &http.Client{Timeout: 15 * time.Second}, out: os.Stdout, in: os.Stdin, err: os.Stderr}
	return r.run(args)
}
func (r cliRunner) run(args []string) error {
	if r.client == nil {
		r.client = &http.Client{Timeout: 15 * time.Second}
	}
	if r.out == nil {
		r.out = io.Discard
	}
	if r.err == nil {
		r.err = io.Discard
	}
	if r.configPath == "" {
		dir, e := os.UserConfigDir()
		if e != nil {
			return e
		}
		r.configPath = filepath.Join(dir, "rollbook", "config.toml")
	}
	profile, output, a, err := cliGlobals(args)
	if err != nil {
		return err
	}
	if len(a) == 0 || a[0] == "help" || a[0] == "--help" || a[0] == "-h" {
		_, err = fmt.Fprintln(r.out, cliHelp)
		return err
	}
	cmd := a[0]
	a = a[1:]
	if len(a) > 0 && (a[0] == "--help" || a[0] == "-h") {
		_, err = fmt.Fprintln(r.out, cliHelp)
		return err
	}
	switch cmd {
	case "version":
		_, err = fmt.Fprintln(r.out, "rollbook dev")
		return err
	case "completion":
		if len(a) != 1 {
			return errors.New("用法: rollbook completion bash|zsh|fish")
		}
		return completion(r.out, a[0])
	case "login":
		return r.login(profile, a)
	}
	cfg, err := readCLIConfig(r.configPath)
	if err != nil {
		return err
	}
	if profile == "" {
		profile = cfg.Default
	}
	if profile == "" {
		profile = "default"
	}
	p := cfg.Profiles[profile]
	base, key := p.URL, p.Key
	if v := os.Getenv("ROLLBOOK_URL"); v != "" {
		base = v
	}
	if v := os.Getenv("ROLLBOOK_KEY"); v != "" {
		key = v
	}
	if base == "" {
		base = "http://127.0.0.1:8080"
	}
	base = strings.TrimRight(base, "/")
	if key == "" {
		return fmt.Errorf("配置 %q 缺少 key；运行 rollbook login --profile %s，或设置 ROLLBOOK_KEY", profile, profile)
	}
	method, path, body, ctype, err := r.command(cmd, a, base, key)
	if err != nil {
		return err
	}
	data, err := r.request(method, base+path, key, body, ctype)
	if err != nil {
		return err
	}
	if cmd == "export" {
		_, err = r.out.Write(data)
		return err
	}
	if cmd == "import" {
		_, err = fmt.Fprintln(r.out, "合并导入已提交，等待网页用户批准。")
		return err
	}
	return formatCLI(r.out, data, output)
}
func cliGlobals(args []string) (profile, output string, rest []string, err error) {
	output = "json"
	for i := 0; i < len(args); i++ {
		s := args[i]
		if s == "--profile" || s == "--output" {
			if i+1 >= len(args) {
				err = fmt.Errorf("%s 需要参数", s)
				return
			}
			i++
			if s == "--profile" {
				profile = args[i]
			} else {
				output = args[i]
			}
			continue
		}
		if strings.HasPrefix(s, "--profile=") {
			profile = strings.TrimPrefix(s, "--profile=")
			continue
		}
		if strings.HasPrefix(s, "--output=") {
			output = strings.TrimPrefix(s, "--output=")
			continue
		}
		rest = append(rest, s)
	}
	if output != "json" && output != "yaml" && output != "table" {
		err = errors.New("--output 仅支持 table、json、yaml")
	}
	return
}
func (r cliRunner) login(profile string, a []string) error {
	if len(a) != 0 {
		return errors.New("用法: rollbook login [--profile 名称]")
	}
	cfg, err := readCLIConfig(r.configPath)
	if err != nil {
		return err
	}
	if profile == "" {
		profile = cfg.Default
	}
	if profile == "" {
		profile = "default"
	}
	base := os.Getenv("ROLLBOOK_URL")
	if base == "" {
		base = cfg.Profiles[profile].URL
	}
	if base == "" {
		base = "http://127.0.0.1:8080"
	}
	fmt.Fprintf(r.err, "服务地址 [%s]: ", base)
	line, err := readCLILine(r.in)
	if err != nil {
		return err
	}
	if line != "" {
		base = line
	}
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
		return errors.New("服务地址必须是 http(s) URL，且不能包含账号密码")
	}
	fmt.Fprint(r.err, "API Key（输入不回显）: ")
	var secret []byte
	if f, ok := r.in.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		secret, err = term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(r.err)
	} else {
		line, err = readCLILine(r.in)
		secret = []byte(line)
	}
	if err != nil {
		return err
	}
	defer func() {
		for i := range secret {
			secret[i] = 0
		}
	}()
	if len(secret) == 0 {
		return errors.New("key 不能为空")
	}
	cfg.Profiles[profile] = cliProfile{URL: strings.TrimRight(base, "/"), Key: string(secret)}
	cfg.Default = profile
	if err = writeCLIConfig(r.configPath, cfg); err != nil {
		return err
	}
	_, err = fmt.Fprintf(r.out, "配置 %s 已保存。\n", profile)
	return err
}
func readCLILine(reader io.Reader) (string, error) {
	var b [1]byte
	var buf []byte
	for {
		n, e := reader.Read(b[:])
		if n > 0 {
			if b[0] == '\n' {
				break
			}
			if b[0] != '\r' {
				buf = append(buf, b[0])
			}
			if len(buf) > 1<<20 {
				return "", errors.New("输入过长")
			}
		}
		if e != nil {
			if e == io.EOF && len(buf) > 0 {
				break
			}
			return "", e
		}
	}
	return strings.TrimSpace(string(buf)), nil
}

// 配置仅接受 CLI 自身写出的 TOML 字符串。
func readCLIConfig(path string) (cliConfig, error) {
	c := cliConfig{Profiles: map[string]cliProfile{}}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	section := ""
	for n, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[profiles.") && strings.HasSuffix(line, "]") {
			section = strings.TrimSuffix(strings.TrimPrefix(line, "[profiles."), "]")
			if strings.HasPrefix(section, "\"") {
				if err := json.Unmarshal([]byte(section), &section); err != nil {
					return c, fmt.Errorf("配置第 %d 行无效", n+1)
				}
			}
			if section == "" || strings.ContainsAny(section, "[]\n\r") {
				return c, fmt.Errorf("配置第 %d 行无效", n+1)
			}
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			return c, fmt.Errorf("配置第 %d 行无效", n+1)
		}
		k = strings.TrimSpace(k)
		var value string
		if json.Unmarshal([]byte(strings.TrimSpace(v)), &value) != nil {
			return c, fmt.Errorf("配置第 %d 行无效", n+1)
		}
		if section == "" {
			if k != "default_profile" {
				return c, fmt.Errorf("配置第 %d 行无效", n+1)
			}
			c.Default = value
			continue
		}
		p := c.Profiles[section]
		switch k {
		case "url":
			p.URL = value
		case "key":
			p.Key = value
		default:
			return c, fmt.Errorf("配置第 %d 行无效", n+1)
		}
		c.Profiles[section] = p
	}
	return c, nil
}
func writeCLIConfig(path string, c cliConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "default_profile = %q\n", c.Default)
	names := make([]string, 0, len(c.Profiles))
	for n := range c.Profiles {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if n == "" || strings.ContainsAny(n, "[]\n\r") {
			return errors.New("profile 名称不能包含括号或换行")
		}
		p := c.Profiles[n]
		fmt.Fprintf(&b, "\n[profiles.%q]\nurl = %q\nkey = %q\n", n, p.URL, p.Key)
	}
	f, err := os.OpenFile(path+".tmp", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	if err = f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if _, err = io.WriteString(f, b.String()); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}
func (r cliRunner) request(method, target, key string, body []byte, ctype string) ([]byte, error) {
	req, err := http.NewRequest(method, target, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	if body != nil {
		if ctype == "" {
			ctype = "application/json"
		}
		req.Header.Set("Content-Type", ctype)
	}
	res, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 17<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		return nil, cliHTTPError(res.StatusCode, data)
	}
	return data, nil
}
func cliHTTPError(status int, data []byte) error {
	var api struct {
		Message string                           `json:"message"`
		Fields  []struct{ Field, Reason string } `json:"fields"`
	}
	_ = json.Unmarshal(data, &api)
	detail := []string{}
	for _, f := range api.Fields {
		detail = append(detail, f.Field+": "+f.Reason)
	}
	if len(detail) == 0 && api.Message != "" && len(api.Message) < 200 {
		detail = append(detail, api.Message)
	}
	suffix := ""
	if len(detail) > 0 {
		suffix = "（" + strings.Join(detail, "；") + "）"
	}
	switch status {
	case 401:
		return fmt.Errorf("401 认证失败%s；请运行 rollbook login 更新 key", suffix)
	case 403:
		return fmt.Errorf("403 权限不足%s；请在网页申请对应权限的 key", suffix)
	case 409:
		return fmt.Errorf("409 版本冲突%s；请重新 get 读取当前 rev 后重试", suffix)
	case 422:
		return fmt.Errorf("422 输入校验失败%s；请修正字段或参数后重试", suffix)
	default:
		return fmt.Errorf("HTTP %d%s", status, suffix)
	}
}
func formatCLI(out io.Writer, data []byte, format string) error {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	v = redactCLI(v)
	switch format {
	case "json":
		e := json.NewEncoder(out)
		e.SetIndent("", "  ")
		return e.Encode(v)
	case "yaml":
		return yaml.NewEncoder(out).Encode(v)
	case "table":
		w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
		rows := []map[string]any{}
		switch x := v.(type) {
		case []any:
			for _, item := range x {
				if m, ok := item.(map[string]any); ok {
					rows = append(rows, m)
				} else {
					rows = append(rows, map[string]any{"value": item})
				}
			}
		case map[string]any:
			rows = append(rows, x)
		default:
			_, err := fmt.Fprintln(out, x)
			return err
		}
		if len(rows) == 0 {
			_, err := fmt.Fprintln(out, "（无结果）")
			return err
		}
		seen := map[string]bool{}
		keys := []string{}
		for _, row := range rows {
			for k := range row {
				if !seen[k] {
					seen[k] = true
					keys = append(keys, k)
				}
			}
		}
		sort.Strings(keys)
		fmt.Fprintln(w, strings.Join(keys, "\t"))
		for _, row := range rows {
			values := make([]string, len(keys))
			for i, k := range keys {
				values[i] = cellCLI(row[k])
			}
			fmt.Fprintln(w, strings.Join(values, "\t"))
		}
		return w.Flush()
	}
	return errors.New("不支持的输出格式")
}
func cellCLI(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return strings.ReplaceAll(x, "\n", " ")
	case map[string]any, []any:
		b, _ := json.Marshal(x)
		return string(b)
	default:
		return fmt.Sprint(v)
	}
}
func redactCLI(v any) any {
	switch x := v.(type) {
	case map[string]any:
		for k, item := range x {
			lower := strings.ToLower(k)
			if lower == "key" || lower == "token" || lower == "password" || lower == "secret" || strings.HasSuffix(lower, "_key") || strings.HasSuffix(lower, "_token") {
				x[k] = "[已隐藏]"
			} else {
				x[k] = redactCLI(item)
			}
		}
		return x
	case []any:
		for i := range x {
			x[i] = redactCLI(x[i])
		}
		return x
	default:
		return v
	}
}
func readJSONArg(s string) (json.RawMessage, error) {
	var b []byte
	var err error
	if strings.HasPrefix(s, "@") {
		b, err = os.ReadFile(strings.TrimPrefix(s, "@"))
	} else {
		b = []byte(s)
	}
	if err != nil {
		return nil, err
	}
	if !json.Valid(b) {
		return nil, errors.New("JSON 无效")
	}
	return b, nil
}
func (r cliRunner) fetchRev(base, key, id string) (int64, error) {
	data, err := r.request("GET", base+"/api/assets/"+url.PathEscape(id), key, nil, "")
	if err != nil {
		return 0, err
	}
	var o struct {
		Rev int64 `json:"rev"`
	}
	if err = json.Unmarshal(data, &o); err != nil {
		return 0, err
	}
	if o.Rev < 1 {
		return 0, errors.New("对象没有有效 rev")
	}
	return o.Rev, nil
}

type writeOptions struct {
	set                                  []string
	reason, body, kind, action, oldPatch string
	rev                                  int64
}
type stringList []string

func (s *stringList) String() string     { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }
func parseWriteFlags(a []string) (writeOptions, error) {
	var o writeOptions
	if len(a) == 3 && !strings.HasPrefix(a[0], "-") {
		rev, e := strconv.ParseInt(a[0], 10, 64)
		if e != nil || rev < 1 {
			return o, errors.New("rev 必须为正整数")
		}
		o.rev = rev
		o.oldPatch = a[1]
		o.reason = a[2]
		return o, nil
	}
	f := flag.NewFlagSet("write", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	f.Var((*stringList)(&o.set), "set", "字段=值")
	f.StringVar(&o.reason, "reason", "", "原因")
	f.StringVar(&o.body, "body", "", "正文")
	f.StringVar(&o.kind, "kind", "", "类型")
	f.StringVar(&o.action, "action", "update", "动作")
	f.Int64Var(&o.rev, "rev", 0, "版本")
	if err := f.Parse(a); err != nil {
		return o, err
	}
	if len(f.Args()) > 0 {
		return o, errors.New("多余参数")
	}
	if o.rev < 0 {
		return o, errors.New("rev 不能为负")
	}
	return o, nil
}
func makePatch(sets []string) (json.RawMessage, error) {
	if len(sets) == 0 {
		return nil, errors.New("至少提供一个 --set field=value")
	}
	patch := map[string]any{}
	fields := map[string]any{}
	for _, s := range sets {
		k, v, ok := strings.Cut(s, "=")
		if !ok || k == "" {
			return nil, fmt.Errorf("无效 --set %q", s)
		}
		var val any = v
		if json.Valid([]byte(v)) && v != "null" {
			_ = json.Unmarshal([]byte(v), &val)
		}
		if strings.HasPrefix(k, "fields.") {
			fields[strings.TrimPrefix(k, "fields.")] = val
		} else if k == "name" || k == "source" || k == "confidence" || k == "status" || k == "aliases" || k == "tags" {
			patch[k] = val
		} else {
			fields[k] = val
		}
	}
	if len(fields) > 0 {
		patch["fields"] = fields
	}
	b, _ := json.Marshal(patch)
	return b, nil
}
func completion(out io.Writer, shell string) error {
	cmds := "login search get create update propose note changes topology site versions verify context export import completion version help"
	var script string
	switch shell {
	case "bash":
		script = "_rollbook_complete() { COMPREPLY=( $(compgen -W '" + cmds + "' -- \"\u0024{COMP_WORDS[COMP_CWORD]}\") ); }\ncomplete -F _rollbook_complete rollbook\n"
	case "zsh":
		script = "#compdef rollbook\n_arguments '*:command:(" + cmds + ")'\n"
	case "fish":
		script = "complete -c rollbook -f -a '" + cmds + "'\n"
	default:
		return errors.New("仅支持 bash、zsh、fish")
	}
	_, err := io.WriteString(out, script)
	return err
}
func (r cliRunner) command(cmd string, a []string, base, key string) (method, path string, body []byte, ctype string, err error) {
	method = "GET"
	jsonBody := func(v any) { body, _ = json.Marshal(v) }
	idPath := func(prefix, id string) string { return prefix + url.PathEscape(id) }
	switch cmd {
	case "search":
		if len(a) != 1 {
			err = errors.New("用法: rollbook search <查询>")
			return
		}
		path = "/api/search?q=" + url.QueryEscape(a[0])
	case "get":
		if len(a) != 1 {
			err = errors.New("用法: rollbook get <id>")
			return
		}
		path = idPath("/api/assets/", a[0])
	case "site":
		if len(a) != 1 {
			err = errors.New("用法: rollbook site <id>")
			return
		}
		path = idPath("/api/sites/", a[0])
	case "topology":
		if len(a) < 1 || len(a) > 3 {
			err = errors.New("用法: rollbook topology <service-id> [--depth 1|2]")
			return
		}
		path = idPath("/api/services/", a[0]) + "/topology"
		if len(a) == 3 && a[1] == "--depth" {
			if a[2] != "1" && a[2] != "2" {
				err = errors.New("depth 仅支持 1 或 2")
				return
			}
			path += "?depth=" + a[2]
		} else if len(a) != 1 {
			err = errors.New("用法: rollbook topology <service-id> [--depth 1|2]")
			return
		}
	case "versions":
		if len(a) == 0 || len(a) == 1 && a[0] == "--outdated" {
			path = "/api/versions/outdated"
		} else if len(a) == 1 && a[0] == "--check" {
			method = "POST"
			path = "/api/versions/check"
			body = []byte("{}")
		} else {
			err = errors.New("用法: rollbook versions [--outdated|--check]")
		}
	case "changes":
		path = "/api/changes"
		if len(a) == 1 && a[0] == "--pending" {
			path += "?state=" + url.QueryEscape("待确认")
		} else if len(a) != 0 {
			err = errors.New("用法: rollbook changes [--pending]")
		}
	case "context":
		if len(a) == 1 && a[0] == "get" {
			path = "/api/contexts"
		} else if len(a) == 2 && a[0] == "put" {
			var obj json.RawMessage
			obj, err = readJSONArg(a[1])
			if err != nil {
				return
			}
			var o struct {
				ID  string `json:"id"`
				Rev int64  `json:"rev"`
			}
			if err = json.Unmarshal(obj, &o); err != nil {
				return
			}
			if o.ID == "" {
				err = errors.New("context 对象需要 id")
				return
			}
			method = "PUT"
			path = idPath("/api/contexts/", o.ID)
			if o.Rev == 0 {
				jsonBody(map[string]any{"object": obj, "rev": 0, "reason": "CLI 记录上下文"})
			} else {
				body = obj
			}
		} else {
			err = errors.New("用法: rollbook context get | put <object-json|@file>")
		}
	case "create":
		if len(a) != 2 {
			err = errors.New("用法: rollbook create <object-json|@file> <reason>")
			return
		}
		var obj json.RawMessage
		obj, err = readJSONArg(a[0])
		if err != nil {
			return
		}
		method = "POST"
		path = "/api/assets"
		jsonBody(map[string]any{"object": obj, "base_rev": 0, "reason": a[1]})
	case "update", "propose":
		if cmd == "propose" && len(a) > 0 && a[0] == "create" {
			if len(a) != 3 {
				err = errors.New("用法: rollbook propose create <object-json|@file> <reason>")
				return
			}
			var obj json.RawMessage
			obj, err = readJSONArg(a[1])
			if err != nil {
				return
			}
			method = "POST"
			path = "/api/changes/proposals"
			jsonBody(map[string]any{"action": "create", "object": obj, "base_rev": 0, "reason": a[2]})
			return
		}
		if len(a) < 2 {
			err = errors.New("用法: rollbook update|propose <id> --set field=value --reason 文本 [--rev N]")
			return
		}
		id := a[0]
		opts, e := parseWriteFlags(a[1:])
		if e != nil {
			err = e
			return
		}
		if opts.reason == "" {
			err = errors.New("--reason 必填")
			return
		}
		rev := opts.rev
		if rev == 0 {
			rev, err = r.fetchRev(base, key, id)
			if err != nil {
				return
			}
		}
		if cmd == "propose" && opts.action == "delete" {
			method = "POST"
			path = "/api/changes/proposals"
			jsonBody(map[string]any{"action": "delete", "id": id, "rev": rev, "reason": opts.reason})
			return
		}
		var patch json.RawMessage
		if opts.oldPatch != "" {
			patch, err = readJSONArg(opts.oldPatch)
		} else {
			patch, err = makePatch(opts.set)
		}
		if err != nil {
			return
		}
		if cmd == "update" {
			method = "PATCH"
			path = idPath("/api/assets/", id)
			jsonBody(map[string]any{"rev": rev, "patch": patch, "reason": opts.reason})
		} else {
			method = "POST"
			path = "/api/changes/proposals"
			jsonBody(map[string]any{"action": "update", "id": id, "rev": rev, "patch": patch, "reason": opts.reason})
		}
	case "note", "verify":
		if len(a) < 1 {
			err = errors.New("需要对象 id")
			return
		}
		id := a[0]
		opts, e := parseWriteFlags(a[1:])
		if e != nil {
			err = e
			return
		}
		if opts.reason == "" {
			err = errors.New("--reason 必填")
			return
		}
		rev := opts.rev
		if rev == 0 {
			rev, err = r.fetchRev(base, key, id)
			if err != nil {
				return
			}
		}
		method = "POST"
		if cmd == "verify" {
			path = idPath("/api/assets/", id) + "/verify"
			jsonBody(map[string]any{"rev": rev, "reason": opts.reason})
		} else {
			if opts.body == "" {
				err = errors.New("note 需要 --body 文本")
				return
			}
			kind := opts.kind
			if kind == "" {
				kind = "运维笔记"
			}
			path = "/api/notes"
			jsonBody(map[string]any{"id": id, "rev": rev, "kind": kind, "body_md": opts.body, "reason": opts.reason})
		}
	case "export":
		if len(a) != 0 {
			err = errors.New("用法: rollbook export")
			return
		}
		path = "/api/backup/export?kind=catalog"
	case "import":
		if len(a) != 1 {
			err = errors.New("用法: rollbook import <file.yaml>")
			return
		}
		body, err = os.ReadFile(a[0])
		if err != nil {
			return
		}
		if len(body) > 16<<20 {
			err = errors.New("导入文件过大")
			return
		}
		var doc map[string]any
		if err = yaml.Unmarshal(body, &doc); err != nil {
			return
		}
		if doc == nil {
			err = errors.New("YAML 无效")
			return
		}
		if kind, ok := doc["kind"]; ok && kind != "merge" {
			err = errors.New("CLI 仅支持合并导入；整库恢复请在网页提交")
			return
		}
		doc["kind"] = "merge"
		if _, ok := doc["schema_version"]; !ok {
			doc["schema_version"] = 1
		}
		body, err = yaml.Marshal(doc)
		if err != nil {
			return
		}
		method = "POST"
		path = "/api/backup/import"
		ctype = "application/yaml"
	default:
		err = fmt.Errorf("未知命令 %q；运行 rollbook --help 查看命令", cmd)
	}
	return
}
