package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/CosmoGao/rollbook/internal/secrets"
	"golang.org/x/crypto/bcrypt"
)

var ErrUnauthorized = errors.New("unauthorized")
var ErrForbidden = errors.New("forbidden")
var ErrInvalid = errors.New("invalid input")

type Level string

const (
	Read    Level = "read"
	Propose Level = "propose"
	Write   Level = "write"
)

type Principal struct {
	UserID, Login, KeyID, KeyName, IssuerID, IssuerName string
	Level                                               Level
}

func (p Principal) IsUser() bool { return p.UserID != "" && p.KeyID == "" }

type Service struct {
	DB          *sql.DB
	Pepper      []byte
	LoginPolicy LoginPolicy
}

func (s Service) valid() error {
	if s.DB == nil || len(s.Pepper) < 32 {
		return errors.New("auth requires database and at least 32 bytes of server pepper")
	}
	return nil
}
func random() (string, error) {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func (s Service) digest(domain, value string) []byte {
	h := hmac.New(sha256.New, s.Pepper)
	h.Write([]byte(domain))
	h.Write([]byte{0})
	h.Write([]byte(value))
	return h.Sum(nil)
}

// BootstrapUser is for a local, one-time setup command; it refuses a second account.
func (s Service) BootstrapUser(ctx context.Context, login, password string) (string, error) {
	if e := s.valid(); e != nil {
		return "", e
	}
	if strings.TrimSpace(login) == "" || len(password) < 12 {
		return "", ErrInvalid
	}
	hash, e := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if e != nil {
		return "", e
	}
	id, e := random()
	if e != nil {
		return "", e
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	var n int
	if e = tx.QueryRowContext(ctx, "SELECT count(*) FROM users").Scan(&n); e != nil {
		return "", e
	}
	if n != 0 {
		return "", ErrForbidden
	}
	_, e = tx.ExecContext(ctx, "INSERT INTO users(id,login,password_hash,created_at) VALUES(?,?,?,?)", id, login, string(hash), time.Now().UTC().Format(time.RFC3339Nano))
	if e != nil {
		return "", e
	}
	return id, tx.Commit()
}

func (s Service) Login(ctx context.Context, login, password string) (session, csrf string, p Principal, err error) {
	return s.LoginWithIP(ctx, login, password, "")
}

// Session verifies the cookie and, for mutating methods, the double-submit CSRF token.
func (s Service) Session(ctx context.Context, session, csrf string, mutating bool) (Principal, error) {
	var p Principal
	if e := s.valid(); e != nil {
		return p, e
	}
	if session == "" {
		return p, ErrUnauthorized
	}
	var expires string
	var stored []byte
	var enabled int
	e := s.DB.QueryRowContext(ctx, `SELECT u.id,u.login,u.enabled,s.expires_at,s.csrf_hash FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.id_hash=?`, s.digest("session", session)).Scan(&p.UserID, &p.Login, &enabled, &expires, &stored)
	deadline, parseErr := time.Parse(time.RFC3339Nano, expires)
	if e != nil || enabled != 1 || parseErr != nil || !deadline.After(time.Now()) {
		return Principal{}, ErrUnauthorized
	}
	if mutating && (csrf == "" || !hmac.Equal(stored, s.digest("csrf", csrf))) {
		return Principal{}, ErrForbidden
	}
	return p, nil
}
func (s Service) Logout(ctx context.Context, session string) error {
	if e := s.valid(); e != nil {
		return e
	}
	if session == "" {
		return ErrUnauthorized
	}
	_, e := s.DB.ExecContext(ctx, "DELETE FROM sessions WHERE id_hash=?", s.digest("session", session))
	return e
}
func SessionCookie(value string, allowInsecureHTTP bool) *http.Cookie {
	return &http.Cookie{Name: "rollbook_session", Value: value, Path: "/", HttpOnly: true, Secure: !allowInsecureHTTP, SameSite: http.SameSiteStrictMode, MaxAge: 12 * 3600}
}
func ClearSessionCookie(allowInsecureHTTP bool) *http.Cookie {
	c := SessionCookie("", allowInsecureHTTP)
	c.MaxAge = -1
	return c
}

type Key struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	IssuerID   string `json:"issuer_id"`
	IssuerName string `json:"issuer_name"`
	CreatedAt  string `json:"created_at"`
	ExpiresAt  string `json:"expires_at"`
	Note       string `json:"note"`
	LastUsedAt string `json:"last_used_at"`
	RevokedAt  string `json:"revoked_at"`
	Level      Level  `json:"level"`
}

func (s Service) IssueKey(ctx context.Context, p Principal, name string, level Level, expires *time.Time, note string) (Key, string, error) {
	var k Key
	if e := s.valid(); e != nil {
		return k, "", e
	}
	if !p.IsUser() {
		return k, "", ErrForbidden
	}
	if strings.TrimSpace(name) == "" || !validLevel(level) {
		return k, "", ErrInvalid
	}
	if err := secrets.Check([]string{name, note}); err != nil {
		return k, "", err
	}
	raw, e := random()
	if e != nil {
		return k, "", e
	}
	k.ID, e = random()
	if e != nil {
		return k, "", e
	}
	k.Name = name
	k.Level = level
	k.IssuerID = p.UserID
	k.IssuerName = p.Login
	k.Note = note
	k.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if expires != nil {
		if !expires.After(time.Now()) {
			return Key{}, "", ErrInvalid
		}
		k.ExpiresAt = expires.UTC().Format(time.RFC3339Nano)
	}
	_, e = s.DB.ExecContext(ctx, `INSERT INTO api_keys(id,name,token_hash,level,issuer_user_id,created_at,expires_at,note) VALUES(?,?,?,?,?,?,?,?)`, k.ID, k.Name, s.digest("key", raw), k.Level, k.IssuerID, k.CreatedAt, null(k.ExpiresAt), k.Note)
	if e != nil {
		return Key{}, "", e
	}
	return k, "rbk_" + raw, nil
}
func validLevel(l Level) bool { return l == Read || l == Propose || l == Write }
func null(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func (s Service) Authenticate(ctx context.Context, bearer string) (Principal, error) {
	var p Principal
	if e := s.valid(); e != nil {
		return p, e
	}
	if !strings.HasPrefix(bearer, "rbk_") {
		return p, ErrUnauthorized
	}
	var expires, revoked sql.NullString
	var enabled int
	e := s.DB.QueryRowContext(ctx, `SELECT k.id,k.name,k.level,k.issuer_user_id,u.login,k.expires_at,k.revoked_at,u.enabled FROM api_keys k JOIN users u ON u.id=k.issuer_user_id WHERE k.token_hash=?`, s.digest("key", strings.TrimPrefix(bearer, "rbk_"))).Scan(&p.KeyID, &p.KeyName, &p.Level, &p.IssuerID, &p.IssuerName, &expires, &revoked, &enabled)
	expired := false
	if expires.Valid {
		deadline, parseErr := time.Parse(time.RFC3339Nano, expires.String)
		expired = parseErr != nil || !deadline.After(time.Now())
	}
	if e != nil || enabled != 1 || revoked.Valid || expired {
		return Principal{}, ErrUnauthorized
	}
	_, _ = s.DB.ExecContext(ctx, "UPDATE api_keys SET last_used_at=? WHERE id=?", time.Now().UTC().Format(time.RFC3339Nano), p.KeyID)
	return p, nil
}
func (s Service) SetLevel(ctx context.Context, p Principal, id string, level Level) error {
	if !p.IsUser() {
		return ErrForbidden
	}
	if !validLevel(level) {
		return ErrInvalid
	}
	r, e := s.DB.ExecContext(ctx, "UPDATE api_keys SET level=? WHERE id=? AND revoked_at IS NULL", level, id)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
func (s Service) Revoke(ctx context.Context, p Principal, id string) error {
	if !p.IsUser() {
		return ErrForbidden
	}
	r, e := s.DB.ExecContext(ctx, "UPDATE api_keys SET revoked_at=? WHERE id=? AND revoked_at IS NULL", time.Now().UTC().Format(time.RFC3339Nano), id)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
func (s Service) Keys(ctx context.Context, p Principal) ([]Key, error) {
	if !p.IsUser() {
		return nil, ErrForbidden
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT k.id,k.name,k.level,k.issuer_user_id,u.login,k.created_at,coalesce(k.expires_at,''),coalesce(k.note,''),coalesce(k.last_used_at,''),coalesce(k.revoked_at,'') FROM api_keys k JOIN users u ON u.id=k.issuer_user_id ORDER BY k.created_at`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Key{}
	for rows.Next() {
		var k Key
		if e = rows.Scan(&k.ID, &k.Name, &k.Level, &k.IssuerID, &k.IssuerName, &k.CreatedAt, &k.ExpiresAt, &k.Note, &k.LastUsedAt, &k.RevokedAt); e != nil {
			return nil, e
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

type Operation string

const (
	View          Operation = "view"
	PutContext    Operation = "put_context"
	RecordNote    Operation = "record_note"
	ProposeChange Operation = "propose_change"
	Update        Operation = "update"
	MarkVerified  Operation = "mark_verified"
	CheckVersions Operation = "check_versions"
	Approve       Operation = "approve"
	Reject        Operation = "reject"
	Delete        Operation = "delete"
	ManageKeys    Operation = "manage_keys"
	Restore       Operation = "restore"
)

func Authorize(p Principal, op Operation, ownerKeyID string) error {
	if p.IsUser() {
		return nil
	}
	if p.KeyID == "" {
		return ErrUnauthorized
	}
	switch op {
	case View:
		return nil
	case PutContext:
		if ownerKeyID == p.KeyID || ownerKeyID == "" {
			return nil
		}
	case RecordNote, ProposeChange:
		if p.Level == Propose || p.Level == Write {
			return nil
		}
	case Update, MarkVerified, CheckVersions:
		if p.Level == Write {
			return nil
		}
	}
	return ErrForbidden
}
