package httpapi

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/CosmoGao/rollbook/internal/auth"
	"github.com/CosmoGao/rollbook/internal/catalog"
	"github.com/go-ldap/ldap/v3"
	"golang.org/x/crypto/hkdf"
)

// 每次发布完整快照；处理一个请求时只读取一次快照。
type ProviderRegistry struct {
	mu          sync.RWMutex
	current     ProviderConfig
	version     string
	environment ProviderConfig
	owned       map[string]bool
	pepper      []byte
	db          *sql.DB
}
type storedProvider struct {
	Kind             string            `json:"kind"`
	Enabled          bool              `json:"enabled"`
	Source           string            `json:"source"`
	Shadowed         bool              `json:"shadowed"`
	Config           map[string]string `json:"config"`
	SecretConfigured bool              `json:"secret_configured"`
	Error            string            `json:"error,omitempty"`
}

var errCredential = errors.New("saved credential cannot be decrypted")

func providerKinds() []string { return []string{"ldap", "oidc", "feishu"} }
func (c ProviderConfig) instance(kind string) string {
	switch kind {
	case "ldap":
		return c.LDAP.Instance
	case "oidc":
		return c.OIDC.Issuer
	case "feishu":
		return c.Feishu.ClientID
	}
	return ""
}
func (e Engine) providerSnapshot() (ProviderConfig, string) {
	if e.ProviderRegistry == nil {
		return e.Providers, "static"
	}
	e.ProviderRegistry.mu.RLock()
	defer e.ProviderRegistry.mu.RUnlock()
	return e.ProviderRegistry.current, e.ProviderRegistry.version
}
func (p *ProviderRegistry) Enabled(kind, instance string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.current.Enabled(kind) && p.current.instance(kind) == instance
}
func (p *ProviderRegistry) Reload(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	c, err := p.load(ctx)
	if err != nil {
		return err
	}
	p.current = c
	p.version, _ = randomToken()
	return nil
}
func (e Engine) withProviders() Engine {
	e.Providers, e.providerVersion = e.providerSnapshot()
	return e
}

func NewProviderRegistry(ctx context.Context, db *sql.DB, pepper []byte, environment ProviderConfig, owned map[string]bool) (*ProviderRegistry, error) {
	if len(pepper) < 32 {
		return nil, auth.ErrInvalid
	}
	p := &ProviderRegistry{db: db, pepper: append([]byte(nil), pepper...), environment: environment, owned: owned}
	c, err := p.load(ctx)
	if err != nil {
		return nil, err
	}
	p.current = c
	p.version, _ = randomToken()
	return p, nil
}
func (p *ProviderRegistry) aead() (cipher.AEAD, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(hkdf.New(sha256.New, p.pepper, nil, []byte("rollbook/auth-provider-settings/v1")), key); err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
func (p *ProviderRegistry) seal(kind, secret string) (string, error) {
	a, err := p.aead()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, a.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", err
	}
	data := a.Seal(nil, nonce, []byte(secret), []byte(kind+":secret"))
	return "v1:" + base64.RawStdEncoding.EncodeToString(append(nonce, data...)), nil
}
func (p *ProviderRegistry) open(kind, encoded string) (string, error) {
	if encoded == "" {
		return "", nil
	}
	if !strings.HasPrefix(encoded, "v1:") {
		return "", errCredential
	}
	data, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(encoded, "v1:"))
	if err != nil {
		return "", errCredential
	}
	a, err := p.aead()
	if err != nil {
		return "", errCredential
	}
	if len(data) < a.NonceSize() {
		return "", errCredential
	}
	plain, err := a.Open(nil, data[:a.NonceSize()], data[a.NonceSize():], []byte(kind+":secret"))
	if err != nil {
		return "", errCredential
	}
	return string(plain), nil
}
func configMap(c ProviderConfig, kind string) map[string]string {
	switch kind {
	case "ldap":
		return map[string]string{"url": c.LDAP.URL, "bind_dn": c.LDAP.BindDN, "base_dn": c.LDAP.BaseDN, "filter": c.LDAP.Filter, "subject_attribute": c.LDAP.SubjectAttribute, "instance": c.LDAP.Instance}
	case "oidc":
		return map[string]string{"issuer": c.OIDC.Issuer, "client_id": c.OIDC.ClientID, "redirect_url": c.OIDC.RedirectURL}
	case "feishu":
		return map[string]string{"client_id": c.Feishu.ClientID, "redirect_url": c.Feishu.RedirectURL}
	}
	return nil
}
func providerFromMap(kind string, m map[string]string, secret string) ProviderConfig {
	var c ProviderConfig
	switch kind {
	case "ldap":
		c.LDAP = LDAPConfig{URL: m["url"], BindDN: m["bind_dn"], BaseDN: m["base_dn"], Filter: m["filter"], SubjectAttribute: m["subject_attribute"], Instance: m["instance"], BindPassword: secret}
	case "oidc":
		c.OIDC = OAuthProviderConfig{Issuer: m["issuer"], ClientID: m["client_id"], RedirectURL: m["redirect_url"], ClientSecret: secret}
	case "feishu":
		c.Feishu = OAuthProviderConfig{ClientID: m["client_id"], RedirectURL: m["redirect_url"], ClientSecret: secret}
	}
	return c
}
func mergeProvider(dst *ProviderConfig, part ProviderConfig, kind string, enabled bool) {
	switch kind {
	case "ldap":
		if enabled {
			dst.LDAP = part.LDAP
		} else {
			dst.LDAP = LDAPConfig{}
		}
	case "oidc":
		if enabled {
			dst.OIDC = part.OIDC
		} else {
			dst.OIDC = OAuthProviderConfig{}
		}
	case "feishu":
		if enabled {
			dst.Feishu = part.Feishu
		} else {
			dst.Feishu = OAuthProviderConfig{}
		}
	}
}
func (p *ProviderRegistry) load(ctx context.Context) (ProviderConfig, error) {
	c := p.environment
	rows, err := p.db.QueryContext(ctx, "SELECT kind,enabled,config,secret FROM auth_provider_settings")
	if err != nil {
		return c, err
	}
	defer rows.Close()
	for rows.Next() {
		var kind, raw, secret string
		var enabled bool
		if err = rows.Scan(&kind, &enabled, &raw, &secret); err != nil {
			return c, err
		}
		if p.owned[kind] || !enabled {
			continue
		}
		var m map[string]string
		if json.Unmarshal([]byte(raw), &m) != nil {
			return c, auth.ErrInvalid
		}
		clear, openErr := p.open(kind, secret)
		if openErr != nil {
			// 保留本地恢复入口；该源不会启用，管理页给出重新填写提示。
			continue
		}
		part := providerFromMap(kind, m, clear)
		if part.Validate() != nil {
			return c, auth.ErrInvalid
		}
		mergeProvider(&c, part, kind, true)
	}
	return c, rows.Err()
}
func (p *ProviderRegistry) list(ctx context.Context) ([]storedProvider, error) {
	out := make([]storedProvider, 0, 3)
	for _, kind := range providerKinds() {
		item := storedProvider{Kind: kind, Source: "page", Config: map[string]string{}}
		var raw, secret string
		var enabled bool
		err := p.db.QueryRowContext(ctx, "SELECT enabled,config,secret FROM auth_provider_settings WHERE kind=?", kind).Scan(&enabled, &raw, &secret)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		if err == nil {
			item.Shadowed = p.owned[kind]
			item.SecretConfigured = secret != ""
			if !p.owned[kind] {
				_ = json.Unmarshal([]byte(raw), &item.Config)
			}
			if enabled && !p.owned[kind] {
				if _, err = p.open(kind, secret); err != nil {
					item.Error = "已保存凭据无法解密；请用原 pepper 恢复，或重新填写凭据"
				}
			}
		}
		if p.owned[kind] {
			item.Source = "environment"
			item.Enabled = p.environment.Enabled(kind)
			item.Config = configMap(p.environment, kind)
			switch kind {
			case "ldap":
				item.SecretConfigured = p.environment.LDAP.BindPassword != ""
			case "oidc":
				item.SecretConfigured = p.environment.OIDC.ClientSecret != ""
			case "feishu":
				item.SecretConfigured = p.environment.Feishu.ClientSecret != ""
			}
		} else {
			item.Enabled = enabled && item.Error == ""
		}
		out = append(out, item)
	}
	return out, nil
}
func (p *ProviderRegistry) save(ctx context.Context, kind string, input providerUpdate) error {
	if p.owned[kind] {
		return auth.ErrForbidden
	}
	if kind != "ldap" && kind != "oidc" && kind != "feishu" {
		return auth.ErrInvalid
	}
	if input.SecretAction != "keep" && input.SecretAction != "replace" && input.SecretAction != "clear" {
		if kind == "feishu" {
			return providerField("secret_action", "凭据动作无效")
		}
		return auth.ErrInvalid
	}
	allowed := map[string]map[string]bool{
		"ldap":   {"url": true, "bind_dn": true, "base_dn": true, "filter": true, "subject_attribute": true, "instance": true},
		"oidc":   {"issuer": true, "client_id": true, "redirect_url": true},
		"feishu": {"client_id": true, "redirect_url": true},
	}
	for key, value := range input.Config {
		if !allowed[kind][key] || len(value) > 2048 {
			if kind == "feishu" {
				return providerField("config", "配置字段无效")
			}
			return auth.ErrInvalid
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var raw, secret string
	var wasEnabled bool
	err = tx.QueryRowContext(ctx, "SELECT enabled,config,secret FROM auth_provider_settings WHERE kind=?", kind).Scan(&wasEnabled, &raw, &secret)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil && input.SecretAction == "keep" {
		if _, err = p.open(kind, secret); err != nil {
			return errCredential
		}
	}
	switch input.SecretAction {
	case "replace":
		if input.Secret == "" {
			if kind == "feishu" {
				return providerField("secret_action", "覆盖凭据时须填写新凭据")
			}
			return auth.ErrInvalid
		}
		secret, err = p.seal(kind, input.Secret)
		if err != nil {
			return err
		}
	case "clear":
		secret = ""
	}
	part := providerFromMap(kind, input.Config, "")
	if input.Enabled {
		if kind == "feishu" {
			if strings.TrimSpace(input.Config["client_id"]) == "" {
				return providerField("client_id", "App ID 必填")
			}
			if secret == "" {
				return providerField("secret_action", "启用前须覆盖凭据")
			}
		}
		clear, err := p.open(kind, secret)
		if err != nil {
			return errCredential
		}
		part = providerFromMap(kind, input.Config, clear)
		if part.Validate() != nil || !part.Enabled(kind) {
			if kind == "feishu" {
				return providerField("redirect_url", "回调地址无效")
			}
			return auth.ErrInvalid
		}
	}
	next := p.current
	mergeProvider(&next, part, kind, input.Enabled)
	bytes, err := json.Marshal(input.Config)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO auth_provider_settings(kind,enabled,config,secret) VALUES(?,?,?,?) ON CONFLICT(kind) DO UPDATE SET enabled=excluded.enabled,config=excluded.config,secret=excluded.secret", kind, input.Enabled, string(bytes), secret)
	if err != nil {
		return err
	}
	oldInstance := p.current.instance(kind)
	if oldInstance != "" {
		if _, err = tx.ExecContext(ctx, "DELETE FROM sessions WHERE provider=? AND instance=?", kind, oldInstance); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "UPDATE identity_bindings SET verified_at=NULL WHERE provider=? AND instance=?", kind, oldInstance); err != nil {
			return err
		}
	}
	if err = ensureProviderPath(ctx, tx, next); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO auth_audit(occurred_at,action,target) VALUES(?,?,?)", time.Now().UTC().Format(time.RFC3339Nano), "provider_update", kind); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	p.current = next
	p.version, _ = randomToken()
	return nil
}

func providerField(field, reason string) error {
	return &catalog.ValidationError{Fields: []catalog.FieldError{{Field: field, Reason: reason}}}
}

type providerUpdate struct {
	Enabled      bool              `json:"enabled"`
	Config       map[string]string `json:"config"`
	SecretAction string            `json:"secret_action"`
	Secret       string            `json:"secret"`
}

func ensureProviderPath(ctx context.Context, tx *sql.Tx, c ProviderConfig) error {
	var local string
	if err := tx.QueryRowContext(ctx, "SELECT value FROM auth_settings WHERE name='local_login_enabled'").Scan(&local); err != nil {
		return err
	}
	if local == "1" {
		return nil
	}
	var count int
	err := tx.QueryRowContext(ctx, `SELECT count(*) FROM identity_bindings b JOIN users u ON u.id=b.user_id WHERE b.allowed=1 AND u.enabled=1 AND b.verified_at>=? AND ((b.provider='ldap' AND b.instance=? AND ?<>'') OR (b.provider='oidc' AND b.instance=? AND ?<>'') OR (b.provider='feishu' AND b.instance=? AND ?<>''))`, time.Now().UTC().Add(-time.Hour).Format(time.RFC3339Nano), c.LDAP.Instance, c.LDAP.URL, c.OIDC.Issuer, c.OIDC.Issuer, c.Feishu.ClientID, c.Feishu.ClientID).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		return auth.ErrForbidden
	}
	return nil
}
func (e Engine) providerSettings(w http.ResponseWriter, r *http.Request) {
	if !e.sameOrigin(r) {
		WriteJSON(w, 403, APIError{Code: "public_origin_mismatch", Message: "公开地址与请求来源不符；请检查可信代理转发的 Host、Proto 和 Port"})
		return
	}
	if _, err := e.Principal(r); err != nil {
		WriteError(w, err)
		return
	}
	if r.Header.Get("Authorization") != "" {
		WriteError(w, auth.ErrForbidden)
		return
	}
	p := e.ProviderRegistry
	if p == nil {
		WriteJSON(w, 503, APIError{Code: "unavailable", Message: "认证设置不可用"})
		return
	}
	path := r.URL.Path
	switch {
	case path == "/api/auth/settings" && r.Method == http.MethodGet:
		items, err := p.list(r.Context())
		if err != nil {
			WriteError(w, err)
			return
		}
		local, err := e.Auth.LocalEnabled(r.Context())
		if err != nil {
			WriteError(w, err)
			return
		}
		origin, _ := e.publicOrigin(r)
		WriteJSON(w, 200, map[string]any{"providers": items, "local_enabled": local, "public_origin": origin})
	case (path == "/api/auth/settings/ldap" || path == "/api/auth/settings/oidc" || path == "/api/auth/settings/feishu") && r.Method == http.MethodPut:
		kind := strings.TrimPrefix(path, "/api/auth/settings/")
		var input providerUpdate
		if err := decodeBody(w, r, &input); err != nil {
			WriteError(w, err)
			return
		}
		origin, validOrigin := e.publicOrigin(r)
		if kind != "ldap" && (!validOrigin || input.Config["redirect_url"] != origin+"/api/auth/"+kind+"/callback" || !strings.HasPrefix(origin, "https://")) {
			if kind == "feishu" {
				WriteError(w, providerField("redirect_url", "回调地址与公开入口不符；检查可信代理 Host、Proto 和 Port"))
			} else {
				WriteError(w, auth.ErrInvalid)
			}
			return
		}
		if len(input.Config) > 12 {
			WriteError(w, catalog.ErrInvalid)
			return
		}
		if err := p.save(r.Context(), kind, input); err != nil {
			if errors.Is(err, errCredential) {
				WriteJSON(w, 409, APIError{Code: "credential_unavailable", Message: "已保存凭据无法解密；请恢复原 pepper 或重新填写凭据"})
				return
			}
			WriteError(w, err)
			return
		}
		WriteJSON(w, 200, map[string]bool{"saved": true})
	case path == "/api/auth/settings/identities" && r.Method == http.MethodGet:
		users, bindings, err := e.identityLists(r.Context())
		if err != nil {
			WriteError(w, err)
			return
		}
		WriteJSON(w, 200, map[string]any{"users": users, "bindings": bindings})
	case (path == "/api/auth/settings/ldap/test" || path == "/api/auth/settings/oidc/test" || path == "/api/auth/settings/feishu/test") && r.Method == http.MethodPost:
		kind := strings.TrimSuffix(strings.TrimPrefix(path, "/api/auth/settings/"), "/test")
		if err := p.test(r.Context(), kind); err != nil {
			WriteJSON(w, 422, APIError{Code: "connectivity_failed", Message: "身份源连通或配置校验失败；请检查地址、证书与凭据"})
			return
		}
		WriteJSON(w, 200, map[string]bool{"reachable": true})
	case path == "/api/auth/settings/audit" && r.Method == http.MethodGet:
		page := 1
		_, _ = fmt.Sscan(r.URL.Query().Get("page"), &page)
		if page < 1 || page > 100000 {
			WriteError(w, auth.ErrInvalid)
			return
		}
		rows, err := e.Auth.DB.QueryContext(r.Context(), "SELECT id,occurred_at,action,target FROM auth_audit ORDER BY id DESC LIMIT 20 OFFSET ?", (page-1)*20)
		if err != nil {
			WriteError(w, err)
			return
		}
		defer rows.Close()
		result := []map[string]any{}
		for rows.Next() {
			var id int64
			var at, action, target string
			if err = rows.Scan(&id, &at, &action, &target); err != nil {
				WriteError(w, err)
				return
			}
			result = append(result, map[string]any{"id": id, "occurred_at": at, "action": action, "target": target})
		}
		WriteJSON(w, 200, map[string]any{"page": page, "entries": result})
	case path == "/api/auth/settings/users" && r.Method == http.MethodPost:
		var input struct {
			Login string `json:"login"`
		}
		if err := decodeBody(w, r, &input); err != nil {
			WriteError(w, err)
			return
		}
		if err := e.Auth.CreateExternalUser(r.Context(), input.Login); err != nil {
			WriteError(w, err)
			return
		}
		WriteJSON(w, 200, map[string]bool{"saved": true})
	case path == "/api/auth/settings/users" && r.Method == http.MethodPut:
		var input struct {
			Login   string `json:"login"`
			Enabled bool   `json:"enabled"`
		}
		if err := decodeBody(w, r, &input); err != nil {
			WriteError(w, err)
			return
		}
		if err := e.Auth.SetUserEnabled(r.Context(), input.Login, input.Enabled); err != nil {
			WriteError(w, err)
			return
		}
		WriteJSON(w, 200, map[string]bool{"saved": true})
	case path == "/api/auth/settings/users/email" && r.Method == http.MethodPut:
		var input struct {
			Login string `json:"login"`
			Email string `json:"email"`
		}
		if err := decodeBody(w, r, &input); err != nil {
			WriteError(w, err)
			return
		}
		var userID string
		if err := e.Auth.DB.QueryRowContext(r.Context(), "SELECT id FROM users WHERE lower(login)=lower(?)", input.Login).Scan(&userID); err != nil {
			WriteError(w, err)
			return
		}
		if err := e.Auth.SetEmail(r.Context(), userID, input.Email); err != nil {
			WriteError(w, err)
			return
		}
		WriteJSON(w, 200, map[string]bool{"saved": true})
	case path == "/api/auth/settings/bindings" && r.Method == http.MethodPut:
		var input struct {
			Provider string `json:"provider"`
			Instance string `json:"instance"`
			Subject  string `json:"subject"`
			Login    string `json:"login"`
			Allowed  bool   `json:"allowed"`
		}
		if err := decodeBody(w, r, &input); err != nil {
			WriteError(w, err)
			return
		}
		if err := e.Auth.ManageBinding(r.Context(), input.Provider, input.Instance, input.Subject, input.Login, input.Allowed); err != nil {
			WriteError(w, err)
			return
		}
		WriteJSON(w, 200, map[string]bool{"saved": true})
	case path == "/api/auth/settings/local" && r.Method == http.MethodPut:
		var input struct {
			Enabled bool `json:"enabled"`
		}
		if err := decodeBody(w, r, &input); err != nil {
			WriteError(w, err)
			return
		}
		if err := e.Auth.SetLocalEnabled(r.Context(), input.Enabled); err != nil {
			WriteError(w, err)
			return
		}
		WriteJSON(w, 200, map[string]bool{"saved": true})
	default:
		WriteJSON(w, 404, APIError{Code: "not_found", Message: "Not found"})
	}
}
func (e Engine) identityLists(ctx context.Context) ([]map[string]any, []map[string]any, error) {
	users := []map[string]any{}
	bindings := []map[string]any{}
	rows, err := e.Auth.DB.QueryContext(ctx, "SELECT login,enabled,(password_hash IS NOT NULL AND password_hash<>''),coalesce(email,'') FROM users ORDER BY login LIMIT 500")
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var login, email string
		var enabled, hasLocalPassword bool
		if err = rows.Scan(&login, &enabled, &hasLocalPassword, &email); err != nil {
			rows.Close()
			return nil, nil, err
		}
		users = append(users, map[string]any{"login": login, "enabled": enabled, "has_local_password": hasLocalPassword, "email": email})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, nil, err
	}
	rows, err = e.Auth.DB.QueryContext(ctx, "SELECT b.provider,b.instance,b.subject,u.login,b.allowed,b.verified_at FROM identity_bindings b JOIN users u ON u.id=b.user_id ORDER BY b.provider,b.instance,b.subject LIMIT 500")
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var kind, instance, subject, login string
		var allowed bool
		var verified sql.NullString
		if err = rows.Scan(&kind, &instance, &subject, &login, &allowed, &verified); err != nil {
			rows.Close()
			return nil, nil, err
		}
		bindings = append(bindings, map[string]any{"provider": kind, "instance": instance, "subject": subject, "login": login, "allowed": allowed, "verified_at": verified.String})
	}
	err = rows.Err()
	rows.Close()
	return users, bindings, err
}

// 连通测试只使用当前或已保存配置，不改变身份绑定的验通时间。
func (p *ProviderRegistry) test(ctx context.Context, kind string) error {
	p.mu.RLock()
	c := p.current
	owned := p.owned[kind]
	p.mu.RUnlock()
	if !owned && !c.Enabled(kind) {
		var raw, encoded string
		if err := p.db.QueryRowContext(ctx, "SELECT config,secret FROM auth_provider_settings WHERE kind=?", kind).Scan(&raw, &encoded); err != nil {
			return auth.ErrInvalid
		}
		var m map[string]string
		if json.Unmarshal([]byte(raw), &m) != nil {
			return auth.ErrInvalid
		}
		clear, err := p.open(kind, encoded)
		if err != nil {
			return errCredential
		}
		mergeProvider(&c, providerFromMap(kind, m, clear), kind, true)
	}
	if !c.Enabled(kind) || c.Validate() != nil {
		return auth.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	switch kind {
	case "ldap":
		cfg := c.LDAP
		u, err := url.Parse(cfg.URL)
		if err != nil {
			return err
		}
		conn, err := ldap.DialURL(cfg.URL, ldap.DialWithDialer(&net.Dialer{Timeout: 5 * time.Second}), ldap.DialWithTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12, ServerName: u.Hostname()}))
		if err != nil {
			return err
		}
		defer conn.Close()
		conn.SetTimeout(7 * time.Second)
		if u.Scheme == "ldap" {
			if err = conn.StartTLS(&tls.Config{MinVersion: tls.VersionTLS12, ServerName: u.Hostname()}); err != nil {
				return err
			}
		}
		if err = conn.Bind(cfg.BindDN, cfg.BindPassword); err != nil {
			return err
		}
		filter, err := safeLDAPFilter(cfg.Filter, "rollbook-connectivity-check")
		if err != nil {
			return err
		}
		_, err = conn.Search(ldap.NewSearchRequest(cfg.BaseDN, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 1, 5, false, filter, []string{cfg.SubjectAttribute}, nil))
		return err
	case "oidc":
		client := &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(c.OIDC.Issuer, "/")+"/.well-known/openid-configuration", nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return auth.ErrInvalid
		}
		var doc struct {
			Issuer string `json:"issuer"`
		}
		if err = json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&doc); err != nil {
			return err
		}
		if doc.Issuer != c.OIDC.Issuer {
			return auth.ErrInvalid
		}
		return nil
	case "feishu":
		body, _ := json.Marshal(map[string]string{"app_id": c.Feishu.ClientID, "app_secret": c.Feishu.ClientSecret})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://open.feishu.cn/open-apis/auth/v3/app_access_token/internal", strings.NewReader(string(body)))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return auth.ErrInvalid
		}
		var result struct {
			Code  int    `json:"code"`
			Token string `json:"app_access_token"`
		}
		if err = json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&result); err != nil {
			return err
		}
		if result.Code != 0 || result.Token == "" {
			return auth.ErrInvalid
		}
		return nil
	}
	return auth.ErrInvalid
}
