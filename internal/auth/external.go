package auth

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// LocalEnabled 从数据库读取开关，本机恢复命令无需重启即可生效。
func (s Service) LocalEnabled(ctx context.Context) (bool, error) {
	if s.DB == nil {
		return false, errors.New("auth requires database")
	}
	var v string
	err := s.DB.QueryRowContext(ctx, "SELECT value FROM auth_settings WHERE name='local_login_enabled'").Scan(&v)
	return v == "1", err
}

// Identity 返回当前网页用户可用的登录凭据类型。
func (s Service) Identity(ctx context.Context, p Principal) (string, bool, error) {
	var hash sql.NullString
	var bindings int
	if err := s.DB.QueryRowContext(ctx, "SELECT password_hash FROM users WHERE id=?", p.UserID).Scan(&hash); err != nil {
		return "", false, err
	}
	if err := s.DB.QueryRowContext(ctx, "SELECT count(*) FROM identity_bindings WHERE user_id=? AND allowed=1", p.UserID).Scan(&bindings); err != nil {
		return "", false, err
	}
	enabled, err := s.LocalEnabled(ctx)
	if err != nil {
		return "", false, err
	}
	local := hash.Valid && hash.String != ""
	kind := "external"
	if local {
		kind = "local"
	}
	if local && bindings > 0 {
		kind = "mixed"
	}
	return kind, local && enabled, nil
}

// ExternalSession 只依据显式绑定签发外部登录会话，不使用邮箱或显示名自动关联。
func (s Service) ExternalSession(ctx context.Context, provider, instance, subject string) (session, csrf string, p Principal, err error) {
	if err = s.valid(); err != nil {
		return
	}
	if provider == "" || instance == "" || subject == "" {
		err = ErrUnauthorized
		return
	}
	var allowed, enabled int
	err = s.DB.QueryRowContext(ctx, `SELECT u.id,u.login,u.enabled,b.allowed FROM identity_bindings b JOIN users u ON u.id=b.user_id WHERE b.provider=? AND b.instance=? AND b.subject=?`, provider, instance, subject).Scan(&p.UserID, &p.Login, &enabled, &allowed)
	if err != nil || enabled != 1 || allowed != 1 {
		p = Principal{}
		err = ErrUnauthorized
		return
	}
	session, err = random()
	if err != nil {
		return
	}
	csrf, err = random()
	if err != nil {
		return
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		err = e
		return
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := tx.ExecContext(ctx, `INSERT INTO sessions(id_hash,user_id,expires_at,csrf_hash,provider,instance,subject) SELECT ?,u.id,?,?,b.provider,b.instance,b.subject FROM identity_bindings b JOIN users u ON u.id=b.user_id WHERE b.provider=? AND b.instance=? AND b.subject=? AND b.allowed=1 AND u.enabled=1 AND u.id=?`, s.digest("session", session), time.Now().UTC().Add(12*time.Hour).Format(time.RFC3339Nano), s.digest("csrf", csrf), provider, instance, subject, p.UserID)
	if err != nil {
		return
	}
	n, e := result.RowsAffected()
	if e != nil {
		err = e
		return
	}
	if n != 1 {
		err = ErrUnauthorized
		return
	}
	_, err = tx.ExecContext(ctx, "UPDATE identity_bindings SET verified_at=? WHERE provider=? AND instance=? AND subject=?", now, provider, instance, subject)
	if err != nil {
		return
	}
	err = tx.Commit()
	if err == nil {
		p.Provider = provider
		p.Instance = instance
		p.Subject = subject
	}
	return
}

func (s Service) ManageBinding(ctx context.Context, provider, instance, subject, login string, allowed bool) error {
	if provider != "ldap" && provider != "oidc" && provider != "feishu" {
		return ErrInvalid
	}
	if instance == "" || subject == "" || login == "" {
		return ErrInvalid
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var userID, oldUser string
	if err = tx.QueryRowContext(ctx, "SELECT id FROM users WHERE lower(login)=lower(?)", login).Scan(&userID); err != nil {
		return err
	}
	oldErr := tx.QueryRowContext(ctx, "SELECT user_id FROM identity_bindings WHERE provider=? AND instance=? AND subject=?", provider, instance, subject).Scan(&oldUser)
	if oldErr != nil && !errors.Is(oldErr, sql.ErrNoRows) {
		return oldErr
	}
	a := 0
	if allowed {
		a = 1
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO identity_bindings(provider,instance,subject,user_id,allowed) VALUES(?,?,?,?,?) ON CONFLICT(provider,instance,subject) DO UPDATE SET user_id=excluded.user_id,allowed=excluded.allowed,verified_at=NULL`, provider, instance, subject, userID, a)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM sessions WHERE user_id=? OR user_id=?", userID, oldUser); err != nil {
		return err
	}
	if err = s.ensureLoginPath(ctx, tx); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO auth_audit(occurred_at,action,target) VALUES(?,?,?)", time.Now().UTC().Format(time.RFC3339Nano), "binding_update", provider+":"+instance+":"+subject); err != nil {
		return err
	}
	return tx.Commit()
}

func (s Service) SetUserEnabled(ctx context.Context, login string, enabled bool) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	v := 0
	if enabled {
		v = 1
	}
	r, err := tx.ExecContext(ctx, "UPDATE users SET enabled=? WHERE lower(login)=lower(?)", v, login)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return sql.ErrNoRows
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM sessions WHERE user_id=(SELECT id FROM users WHERE lower(login)=lower(?))", login); err != nil {
		return err
	}
	if err = s.ensureLoginPath(ctx, tx); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO auth_audit(occurred_at,action,target) VALUES(?,?,?)", time.Now().UTC().Format(time.RFC3339Nano), "user_enabled_"+strings.ToLower(map[bool]string{true: "true", false: "false"}[enabled]), login); err != nil {
		return err
	}
	return tx.Commit()
}

// 本机恢复命令始终可重新开放入口；常规管理操作避免意外移除最后一个可用账号。
func (s Service) ensureLoginPath(ctx context.Context, tx *sql.Tx) error {
	var count int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM users WHERE enabled=1").Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		return ErrForbidden
	}
	var local string
	if err := tx.QueryRowContext(ctx, "SELECT value FROM auth_settings WHERE name='local_login_enabled'").Scan(&local); err != nil {
		return err
	}
	if local == "1" {
		return nil
	}
	return s.verifiedPath(ctx, tx)
}

func (s Service) verifiedPath(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, "SELECT b.provider,b.instance FROM identity_bindings b JOIN users u ON u.id=b.user_id WHERE b.allowed=1 AND b.verified_at>=? AND u.enabled=1", time.Now().UTC().Add(-time.Hour).Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var kind, instance string
		if err = rows.Scan(&kind, &instance); err != nil {
			return err
		}
		if s.EnabledProvider == nil || s.EnabledProvider(kind, instance) {
			return nil
		}
	}
	if err = rows.Err(); err != nil {
		return err
	}
	return ErrForbidden
}

func (s Service) CreateExternalUser(ctx context.Context, login string) error {
	login = strings.TrimSpace(login)
	if login == "" {
		return ErrInvalid
	}
	id, err := random()
	if err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err = tx.ExecContext(ctx, "INSERT INTO users(id,login,password_hash,created_at) VALUES(?,?,NULL,?)", id, login, now); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO auth_audit(occurred_at,action,target) VALUES(?,?,?)", now, "user_create", login); err != nil {
		return err
	}
	return tx.Commit()
}

func (s Service) SetLocalEnabled(ctx context.Context, enabled bool) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if !enabled {
		if err = s.verifiedPath(ctx, tx); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "DELETE FROM sessions WHERE provider='local'"); err != nil {
			return err
		}
	}
	value := "0"
	if enabled {
		value = "1"
	}
	if _, err = tx.ExecContext(ctx, "UPDATE auth_settings SET value=? WHERE name='local_login_enabled'", value); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO auth_audit(occurred_at,action,target) VALUES(?,?,?)", time.Now().UTC().Format(time.RFC3339Nano), "local_login", value); err != nil {
		return err
	}
	return tx.Commit()
}
