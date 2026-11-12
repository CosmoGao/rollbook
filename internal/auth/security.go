package auth

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// LoginPolicy 控制账号和客户端 IP 的限速；零值使用默认配置。
type LoginPolicy struct {
	Threshold    int
	MaxBackoff   time.Duration
	LockDuration time.Duration
}

func (p LoginPolicy) defaults() LoginPolicy {
	if p.Threshold <= 0 {
		p.Threshold = 5
	}
	if p.MaxBackoff <= 0 {
		p.MaxBackoff = time.Hour
	}
	if p.LockDuration <= 0 {
		p.LockDuration = 15 * time.Minute
	}
	return p
}

// LoginWithIP 使用 HTTP 层提供的连接对端 IP；调用方不得传入未验证的代理头。
func (s Service) LoginWithIP(ctx context.Context, login, password, ip string) (session, csrf string, p Principal, err error) {
	if err = s.valid(); err != nil {
		return
	}
	login = strings.ToLower(strings.TrimSpace(login))
	policy := s.LoginPolicy.defaults()
	now := time.Now().UTC()
	if _, err = s.DB.ExecContext(ctx, "DELETE FROM login_limits WHERE updated_at<?", now.Add(-30*24*time.Hour).Format(time.RFC3339Nano)); err != nil {
		return
	}
	subjects := []struct{ kind, value string }{{"account", login}, {"ip", ip}}
	var ipBlocked, ipUpdated time.Time
	for _, subject := range subjects {
		if subject.value == "" {
			continue
		}
		var until, updated string
		e := s.DB.QueryRowContext(ctx, "SELECT blocked_until,updated_at FROM login_limits WHERE kind=? AND subject=?", subject.kind, s.digest("limit:"+subject.kind, subject.value)).Scan(&until, &updated)
		if e != nil && !errors.Is(e, sql.ErrNoRows) {
			err = e
			return
		}
		deadline, _ := time.Parse(time.RFC3339Nano, until)
		if deadline.After(now) {
			if subject.kind == "account" {
				log.Printf("登录锁定 kind=%s", subject.kind)
				err = ErrUnauthorized
				return
			}
			ipBlocked = deadline
			ipUpdated, _ = time.Parse(time.RFC3339Nano, updated)
		}
	}
	var hash, canonical string
	var resetAt sql.NullString
	var enabled int
	e := s.DB.QueryRowContext(ctx, "SELECT id,login,password_hash,enabled,password_reset_at FROM users WHERE lower(login)=?", login).Scan(&p.UserID, &canonical, &hash, &enabled, &resetAt)
	if e != nil && !errors.Is(e, sql.ErrNoRows) {
		err = e
		return
	}
	// 未知账号也执行相同成本的 bcrypt 校验，减少枚举账号的时间侧信道。
	if e != nil {
		hash = "$2a$10$7EqJtq98hPqEX7fNZaFWoOhi5DxjJbTbgv.UGzS5nBYhI8zCX2.sO"
	}
	// 本机重置后，只允许该账号用新密码越过重置前已有的 IP 锁定。
	// 其他账号和重置后新产生的 IP 锁定仍按原策略拒绝。
	if !ipBlocked.IsZero() {
		resetTime, _ := time.Parse(time.RFC3339Nano, resetAt.String)
		if e != nil || !resetTime.After(ipUpdated) {
			log.Printf("登录锁定 kind=ip")
			p = Principal{}
			return "", "", p, ErrUnauthorized
		}
	}
	good := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
	if e != nil || enabled != 1 || !good {
		for _, subject := range subjects {
			if subject.value == "" {
				continue
			}
			digest := s.digest("limit:"+subject.kind, subject.value)
			var failures int
			_ = s.DB.QueryRowContext(ctx, "SELECT failures FROM login_limits WHERE kind=? AND subject=?", subject.kind, digest).Scan(&failures)
			failures++
			delay := time.Duration(0)
			if failures >= policy.Threshold {
				delay = policy.LockDuration
				for i := policy.Threshold; i < failures && delay < policy.MaxBackoff; i++ {
					delay *= 2
				}
				if delay > policy.MaxBackoff {
					delay = policy.MaxBackoff
				}
			}
			_, err = s.DB.ExecContext(ctx, `INSERT INTO login_limits(kind,subject,failures,blocked_until,updated_at) VALUES(?,?,?,?,?) ON CONFLICT(kind,subject) DO UPDATE SET failures=excluded.failures,blocked_until=excluded.blocked_until,updated_at=excluded.updated_at`, subject.kind, digest, failures, now.Add(delay).Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
			if err != nil {
				return
			}
			if delay > 0 {
				log.Printf("登录锁定 kind=%s failures=%d", subject.kind, failures)
			}
		}
		log.Printf("登录失败")
		p = Principal{}
		err = ErrUnauthorized
		return
	}
	p.Login = canonical
	session, err = random()
	if err != nil {
		return
	}
	csrf, err = random()
	if err != nil {
		return
	}
	_, err = s.DB.ExecContext(ctx, "INSERT INTO sessions(id_hash,user_id,expires_at,csrf_hash) VALUES(?,?,?,?)", s.digest("session", session), p.UserID, now.Add(12*time.Hour).Format(time.RFC3339Nano), s.digest("csrf", csrf))
	if err == nil {
		for _, subject := range subjects {
			if subject.value != "" {
				_, _ = s.DB.ExecContext(ctx, "DELETE FROM login_limits WHERE kind=? AND subject=?", subject.kind, s.digest("limit:"+subject.kind, subject.value))
			}
		}
	}
	return
}

func (s Service) ChangePassword(ctx context.Context, p Principal, oldPassword, newPassword string) error {
	if !p.IsUser() {
		return ErrForbidden
	}
	if len(newPassword) < 12 {
		return ErrInvalid
	}
	var hash string
	if err := s.DB.QueryRowContext(ctx, "SELECT password_hash FROM users WHERE id=?", p.UserID).Scan(&hash); err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(oldPassword)) != nil {
		return ErrUnauthorized
	}
	return s.setPassword(ctx, p.UserID, newPassword, hash, "")
}

func (s Service) ResetPassword(ctx context.Context, login, newPassword string) error {
	if err := s.valid(); err != nil {
		return err
	}
	if len(newPassword) < 12 {
		return ErrInvalid
	}
	var id, canonical, hash string
	if err := s.DB.QueryRowContext(ctx, "SELECT id,login,password_hash FROM users WHERE lower(login)=?", strings.ToLower(strings.TrimSpace(login))).Scan(&id, &canonical, &hash); err != nil {
		return err
	}
	return s.setPassword(ctx, id, newPassword, hash, strings.ToLower(canonical))
}

func (s Service) setPassword(ctx context.Context, id, password, oldHash, resetLogin string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var resetAt any
	if resetLogin != "" {
		resetAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	result, err := tx.ExecContext(ctx, "UPDATE users SET password_hash=?,password_reset_at=? WHERE id=? AND password_hash=?", string(hash), resetAt, id, oldHash)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrUnauthorized
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM sessions WHERE user_id=?", id); err != nil {
		return err
	}
	if resetLogin != "" {
		if _, err = tx.ExecContext(ctx, "DELETE FROM login_limits WHERE kind='account' AND subject=?", s.digest("limit:account", resetLogin)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s Service) LogoutAll(ctx context.Context, p Principal) error {
	if !p.IsUser() {
		return ErrForbidden
	}
	_, err := s.DB.ExecContext(ctx, "DELETE FROM sessions WHERE user_id=?", p.UserID)
	return err
}
