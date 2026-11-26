package httpapi

import (
	"bytes"
	"context"
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
	"time"

	"github.com/CosmoGao/rollbook/internal/auth"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-ldap/ldap/v3"
	"golang.org/x/oauth2"
)

// ProviderConfig 仅在服务端配置；上游地址在启动时固定，不从请求获取。
type ProviderConfig struct {
	LDAP       LDAPConfig
	OIDC       OAuthProviderConfig
	Feishu     OAuthProviderConfig
	HTTPClient *http.Client
}
type LDAPConfig struct{ URL, BindDN, BindPassword, BaseDN, Filter, SubjectAttribute, Instance string }
type OAuthProviderConfig struct{ Issuer, ClientID, ClientSecret, RedirectURL string }

func (c ProviderConfig) Client() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 10 * time.Second}
}
func (c ProviderConfig) Enabled(kind string) bool {
	switch kind {
	case "ldap":
		return c.LDAP.URL != ""
	case "oidc":
		return c.OIDC.Issuer != ""
	case "feishu":
		return c.Feishu.ClientID != ""
	}
	return false
}
func (c ProviderConfig) Validate() error {
	if c.LDAP.URL != "" {
		u, e := url.Parse(c.LDAP.URL)
		if e != nil || (u.Scheme != "ldaps" && u.Scheme != "ldap") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || c.LDAP.BaseDN == "" || c.LDAP.BindDN == "" || c.LDAP.BindPassword == "" || c.LDAP.SubjectAttribute == "" || c.LDAP.Instance == "" || !strings.Contains(c.LDAP.Filter, "{login}") {
			return fmt.Errorf("invalid LDAP config")
		}
	}
	if c.OIDC.Issuer != "" {
		u, e := url.Parse(c.OIDC.Issuer)
		if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || c.OIDC.ClientID == "" || c.OIDC.ClientSecret == "" || !validRedirect(c.OIDC.RedirectURL, "/api/auth/oidc/callback") {
			return fmt.Errorf("invalid OIDC config")
		}
	}
	if c.Feishu.ClientID != "" {
		if c.Feishu.ClientSecret == "" || !validRedirect(c.Feishu.RedirectURL, "/api/auth/feishu/callback") {
			return fmt.Errorf("invalid Feishu config")
		}
	}
	return nil
}
func validRedirect(raw, path string) bool {
	u, e := url.Parse(raw)
	return e == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.Path == path && u.RawQuery == "" && u.Fragment == ""
}
func randomToken() (string, error) {
	b := make([]byte, 32)
	_, e := rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b), e
}
func stateHash(state string) []byte { sum := sha256.Sum256([]byte(state)); return sum[:] }
func (e Engine) providerStart(w http.ResponseWriter, r *http.Request, kind string) {
	purpose := "login"
	userID := ""
	var sessionHash []byte
	if kind == "feishu" && r.URL.Path == "/api/auth/feishu/bind/start" {
		purpose = "bind"
		principal, err := e.Principal(r)
		if err != nil || !principal.IsUser() || principal.Provider != "local" {
			WriteError(w, auth.ErrForbidden)
			return
		}
		cookie, err := r.Cookie("rollbook_session")
		if err != nil {
			WriteError(w, auth.ErrUnauthorized)
			return
		}
		userID = principal.UserID
		sessionHash = e.Auth.SessionHash(cookie.Value)
	}
	if r.Method != http.MethodGet || !e.Providers.Enabled(kind) {
		http.NotFound(w, r)
		return
	}
	if !e.secureRequest(r) && !e.AllowInsecureHTTP {
		WriteError(w, auth.ErrForbidden)
		return
	}
	if !e.callbackHost(r, kind) {
		WriteError(w, auth.ErrForbidden)
		return
	}
	state, err := randomToken()
	if err != nil {
		WriteError(w, err)
		return
	}
	nonce, err := randomToken()
	if err != nil {
		WriteError(w, err)
		return
	}
	verifier := oauth2.GenerateVerifier()
	var target string
	_, _ = e.Auth.DB.ExecContext(r.Context(), "DELETE FROM auth_pending WHERE expires_at<?", time.Now().UTC().Format(time.RFC3339Nano))
	switch kind {
	case "oidc":
		cfg := e.Providers.OIDC
		provider, ex := oidc.NewProvider(oidc.ClientContext(r.Context(), e.Providers.Client()), cfg.Issuer)
		if ex != nil {
			WriteError(w, auth.ErrUnauthorized)
			return
		}
		oc := oauth2.Config{ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret, RedirectURL: cfg.RedirectURL, Endpoint: provider.Endpoint(), Scopes: []string{oidc.ScopeOpenID}}
		target = oc.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier))
	case "feishu":
		cfg := e.Providers.Feishu
		u, _ := url.Parse("https://accounts.feishu.cn/open-apis/authen/v1/authorize")
		q := u.Query()
		q.Set("client_id", cfg.ClientID)
		q.Set("redirect_uri", cfg.RedirectURL)
		q.Set("state", state)
		q.Set("response_type", "code")
		q.Set("scope", "auth:user.id:read contact:user.email:readonly")
		u.RawQuery = q.Encode()
		target = u.String()
	}
	version := e.providerVersion
	if version == "" {
		version = "static"
	}
	_, err = e.Auth.DB.ExecContext(r.Context(), "INSERT INTO auth_pending(state_hash,provider,nonce,verifier,expires_at,config_version,purpose,user_id,session_hash) VALUES(?,?,?,?,?,?,?,?,?)", stateHash(state), kind, nonce, verifier, time.Now().UTC().Add(5*time.Minute).Format(time.RFC3339Nano), version, purpose, userID, sessionHash)
	if err != nil {
		WriteError(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "rollbook_auth_state", Value: state, Path: "/api/auth/", HttpOnly: true, Secure: e.secureRequest(r), SameSite: http.SameSiteLaxMode, MaxAge: 300})
	http.Redirect(w, r, target, http.StatusFound)
}
func (e Engine) providerCallback(w http.ResponseWriter, r *http.Request, kind string) {
	if r.Method != http.MethodGet || !e.Providers.Enabled(kind) {
		http.NotFound(w, r)
		return
	}
	if !e.callbackHost(r, kind) {
		WriteError(w, auth.ErrForbidden)
		return
	}
	fail := func() { http.Redirect(w, r, "/?auth_error=external_login_failed", http.StatusSeeOther) }
	cookie, err := r.Cookie("rollbook_auth_state")
	if err != nil || cookie.Value == "" || r.URL.Query().Get("state") != cookie.Value || r.URL.Query().Get("code") == "" {
		fail()
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "rollbook_auth_state", Path: "/api/auth/", MaxAge: -1, HttpOnly: true, Secure: e.secureRequest(r), SameSite: http.SameSiteLaxMode})
	if e.ProviderRegistry != nil {
		e.ProviderRegistry.mu.RLock()
		defer e.ProviderRegistry.mu.RUnlock()
	}
	var pendingKind, nonce, verifier, expiry, version, purpose string
	var userID sql.NullString
	var sessionHash []byte
	tx, err := e.Auth.DB.BeginTx(r.Context(), nil)
	if err != nil {
		fail()
		return
	}
	defer tx.Rollback()
	err = tx.QueryRowContext(r.Context(), "SELECT provider,nonce,verifier,expires_at,config_version,purpose,user_id,session_hash FROM auth_pending WHERE state_hash=?", stateHash(cookie.Value)).Scan(&pendingKind, &nonce, &verifier, &expiry, &version, &purpose, &userID, &sessionHash)
	if err != nil || pendingKind != kind {
		fail()
		return
	}
	if purpose == "bind" {
		fail = func() { http.Redirect(w, r, "/settings/personal?bind_error=failed", http.StatusSeeOther) }
	}
	currentVersion := e.providerVersion
	if currentVersion == "" {
		currentVersion = "static"
	}
	if e.ProviderRegistry != nil {
		currentVersion = e.ProviderRegistry.version
	}
	if version != currentVersion {
		fail()
		return
	}
	_, err = tx.ExecContext(r.Context(), "DELETE FROM auth_pending WHERE state_hash=?", stateHash(cookie.Value))
	if err != nil || tx.Commit() != nil {
		fail()
		return
	}
	until, err := time.Parse(time.RFC3339Nano, expiry)
	if err != nil || !until.After(time.Now()) {
		fail()
		return
	}
	if purpose == "bind" {
		cookie, ex := r.Cookie("rollbook_session")
		if ex != nil || !bytes.Equal(e.Auth.SessionHash(cookie.Value), sessionHash) {
			http.Redirect(w, r, "/settings/personal?bind_error=session", http.StatusSeeOther)
			return
		}
		principal, ex := e.Principal(r)
		if ex != nil || principal.Provider != "local" || principal.UserID != userID.String {
			http.Redirect(w, r, "/settings/personal?bind_error=session", http.StatusSeeOther)
			return
		}
	}
	var instance, subject, email string
	if kind == "oidc" {
		instance = e.Providers.OIDC.Issuer
		subject, err = e.oidcSubject(r.Context(), r.URL.Query().Get("code"), nonce, verifier)
	} else {
		instance = e.Providers.Feishu.ClientID
		subject, email, err = e.feishuSubject(r.Context(), r.URL.Query().Get("code"))
	}
	if err != nil || subject == "" {
		fail()
		return
	}
	if purpose == "bind" {
		err = e.Auth.BindFeishu(r.Context(), userID.String, instance, subject)
		target := "/settings/personal?bind=ok"
		if errors.Is(err, auth.ErrBindingDisabled) {
			target = "/settings/personal?bind_error=disabled"
		} else if err != nil {
			target = "/settings/personal?bind_error=conflict"
		}
		http.Redirect(w, r, target, http.StatusSeeOther)
		return
	}
	var session, csrf string
	if kind == "feishu" {
		session, csrf, _, err = e.Auth.FeishuSession(r.Context(), instance, subject, email)
	} else {
		session, csrf, _, err = e.Auth.ExternalSession(r.Context(), kind, instance, subject)
	}
	if err != nil {
		if kind == "feishu" {
			reason := "failed"
			if errors.Is(err, auth.ErrEmailUnmatched) {
				reason = "unmatched"
			} else if errors.Is(err, auth.ErrBindingConflict) {
				reason = "conflict"
			} else if errors.Is(err, auth.ErrBindingDisabled) {
				reason = "disabled"
			}
			http.Redirect(w, r, "/?auth_error="+reason, http.StatusSeeOther)
			return
		}
		fail()
		return
	}
	http.SetCookie(w, auth.SessionCookie(session, !e.secureRequest(r)))
	http.SetCookie(w, csrfCookie(csrf, 12*3600, !e.secureRequest(r)))
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
func (e Engine) callbackHost(r *http.Request, kind string) bool {
	raw := e.Providers.OIDC.RedirectURL
	if kind == "feishu" {
		raw = e.Providers.Feishu.RedirectURL
	}
	u, err := url.Parse(raw)
	origin, ok := e.publicOrigin(r)
	return err == nil && ok && origin == u.Scheme+"://"+u.Host
}
func (e Engine) oidcSubject(ctx context.Context, code, nonce, verifier string) (string, error) {
	cfg := e.Providers.OIDC
	ctx = oidc.ClientContext(ctx, e.Providers.Client())
	provider, err := oidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return "", err
	}
	oc := oauth2.Config{ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret, RedirectURL: cfg.RedirectURL, Endpoint: provider.Endpoint(), Scopes: []string{oidc.ScopeOpenID}}
	token, err := oc.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return "", err
	}
	raw, ok := token.Extra("id_token").(string)
	if !ok || raw == "" {
		return "", auth.ErrUnauthorized
	}
	id, err := provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}).Verify(ctx, raw)
	if err != nil {
		return "", err
	}
	var claims struct {
		Nonce string `json:"nonce"`
	}
	if err = id.Claims(&claims); err != nil {
		return "", err
	}
	if claims.Nonce != nonce {
		return "", auth.ErrUnauthorized
	}
	return id.Subject, nil
}
func (e Engine) feishuSubject(ctx context.Context, code string) (string, string, error) {
	cfg := e.Providers.Feishu
	body := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "client_id": {cfg.ClientID}, "client_secret": {cfg.ClientSecret}, "redirect_uri": {cfg.RedirectURL}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://accounts.feishu.cn/oauth/v3/token", strings.NewReader(body.Encode()))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := e.Providers.Client().Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", "", auth.ErrUnauthorized
	}
	var token struct {
		AccessToken string `json:"access_token"`
		Code        int    `json:"code"`
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&token); err != nil {
		return "", "", err
	}
	if token.Code != 0 || token.AccessToken == "" {
		return "", "", auth.ErrUnauthorized
	}
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, "https://open.feishu.cn/open-apis/authen/v1/user_info", nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	resp, err = e.Providers.Client().Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", "", auth.ErrUnauthorized
	}
	var user struct {
		Code int `json:"code"`
		Data struct {
			OpenID string `json:"open_id"`
			Email  string `json:"email"`
		} `json:"data"`
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&user); err != nil {
		return "", "", err
	}
	if user.Code != 0 || user.Data.OpenID == "" {
		return "", "", auth.ErrUnauthorized
	}
	return user.Data.OpenID, user.Data.Email, nil
}
func (e Engine) ldapLogin(w http.ResponseWriter, r *http.Request) {
	if !e.Providers.Enabled("ldap") || r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	if !e.sameOrigin(r) || (!e.secureRequest(r) && !e.AllowInsecureHTTP) {
		WriteError(w, auth.ErrForbidden)
		return
	}
	var input struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	if err := decodeBody(w, r, &input); err != nil {
		WriteError(w, err)
		return
	}
	subject, err := e.ldapSubject(input.Login, input.Password)
	if err != nil {
		WriteError(w, auth.ErrUnauthorized)
		return
	}
	session, csrf, p, err := e.Auth.ExternalSession(r.Context(), "ldap", e.Providers.LDAP.Instance, subject)
	if err != nil {
		WriteError(w, auth.ErrUnauthorized)
		return
	}
	http.SetCookie(w, auth.SessionCookie(session, !e.secureRequest(r)))
	http.SetCookie(w, csrfCookie(csrf, 12*3600, !e.secureRequest(r)))
	WriteJSON(w, 200, map[string]string{"login": p.Login})
}
func (e Engine) ldapSubject(login, password string) (string, error) {
	cfg := e.Providers.LDAP
	if strings.TrimSpace(login) == "" || password == "" {
		return "", auth.ErrUnauthorized
	}
	u, err := url.Parse(cfg.URL)
	if err != nil {
		return "", err
	}
	conn, err := ldap.DialURL(cfg.URL, ldap.DialWithDialer(&netDialer), ldap.DialWithTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12, ServerName: u.Hostname()}))
	if err != nil {
		return "", err
	}
	defer conn.Close()
	if u.Scheme == "ldap" {
		if err = conn.StartTLS(&tls.Config{MinVersion: tls.VersionTLS12, ServerName: u.Hostname()}); err != nil {
			return "", err
		}
	}
	if err = conn.Bind(cfg.BindDN, cfg.BindPassword); err != nil {
		return "", err
	}
	filter, err := safeLDAPFilter(cfg.Filter, login)
	if err != nil {
		return "", err
	}
	req := ldap.NewSearchRequest(cfg.BaseDN, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 2, 10, false, filter, []string{cfg.SubjectAttribute}, nil)
	result, err := conn.Search(req)
	if err != nil {
		return "", err
	}
	if len(result.Entries) != 1 {
		return "", auth.ErrUnauthorized
	}
	entry := result.Entries[0]
	subject := entry.GetAttributeValue(cfg.SubjectAttribute)
	if subject == "" {
		return "", auth.ErrUnauthorized
	}
	if err = conn.Bind(entry.DN, password); err != nil {
		return "", err
	}
	return subject, nil
}
func safeLDAPFilter(template, login string) (string, error) {
	filter := template
	if filter == "" {
		filter = "(uid={login})"
	}
	if !strings.Contains(filter, "{login}") {
		return "", auth.ErrInvalid
	}
	return strings.ReplaceAll(filter, "{login}", ldap.EscapeFilter(login)), nil
}

var netDialer = net.Dialer{Timeout: 10 * time.Second}
