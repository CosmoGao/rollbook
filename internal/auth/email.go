package auth

import (
	"context"
	"database/sql"
	"errors"
	"net/mail"
	"strings"
)

var ErrEmailInvalid = errors.New("email_invalid")
var ErrEmailTaken = errors.New("email_taken")
var ErrBindingConflict = errors.New("binding_conflict")
var ErrEmailUnmatched = errors.New("email_unmatched")
var ErrBindingDisabled = errors.New("binding_disabled")

func NormalizeEmail(value string) (string, error) {
	value = strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + ('a' - 'A')
		}
		return r
	}, strings.TrimSpace(value))
	if value == "" {
		return "", nil
	}
	if len(value) > 254 || strings.ContainsAny(value, " \t\r\n") {
		return "", ErrEmailInvalid
	}
	a, err := mail.ParseAddress(value)
	if err != nil || a.Address != value || strings.Count(value, "@") != 1 {
		return "", ErrEmailInvalid
	}
	return value, nil
}
func (s Service) SetEmail(ctx context.Context, userID, email string) error {
	email, err := NormalizeEmail(email)
	if err != nil {
		return err
	}
	var value any = email
	if email == "" {
		value = nil
	}
	result, err := s.DB.ExecContext(ctx, "UPDATE users SET email=? WHERE id=?", value, userID)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return ErrEmailTaken
		}
		return err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return ErrUnauthorized
	}
	return nil
}
func (s Service) UserEmail(ctx context.Context, userID string) (string, error) {
	var email sql.NullString
	err := s.DB.QueryRowContext(ctx, "SELECT email FROM users WHERE id=?", userID).Scan(&email)
	return email.String, err
}
func (s Service) FeishuBinding(ctx context.Context, userID, instance string) (bool, error) {
	var count int
	err := s.DB.QueryRowContext(ctx, "SELECT count(*) FROM identity_bindings WHERE provider='feishu' AND instance=? AND user_id=?", instance, userID).Scan(&count)
	return count > 0, err
}
func (s Service) BindFeishu(ctx context.Context, userID, instance, subject string) error {
	if userID == "" || instance == "" || subject == "" {
		return ErrInvalid
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var enabled int
	if err = tx.QueryRowContext(ctx, "SELECT enabled FROM users WHERE id=?", userID).Scan(&enabled); err != nil || enabled != 1 {
		return ErrUnauthorized
	}
	var existing string
	var allowed int
	err = tx.QueryRowContext(ctx, "SELECT user_id,allowed FROM identity_bindings WHERE provider='feishu' AND instance=? AND subject=?", instance, subject).Scan(&existing, &allowed)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if existing != "" {
		if existing != userID {
			return ErrBindingConflict
		}
		if allowed != 1 {
			return ErrBindingDisabled
		}
		return tx.Commit()
	}
	var other int
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM identity_bindings WHERE provider='feishu' AND instance=? AND user_id=?", instance, userID).Scan(&other); err != nil {
		return err
	}
	if other > 0 {
		return ErrBindingConflict
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO identity_bindings(provider,instance,subject,user_id,allowed) VALUES('feishu',?,?,?,1)", instance, subject, userID)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return ErrBindingConflict
		}
		return err
	}
	return tx.Commit()
}
func (s Service) FeishuSession(ctx context.Context, instance, subject, email string) (string, string, Principal, error) {
	if instance == "" || subject == "" {
		return "", "", Principal{}, ErrUnauthorized
	}
	email, _ = NormalizeEmail(email)
	var bound, matched string
	var allowed, enabled int
	err := s.DB.QueryRowContext(ctx, "SELECT user_id,allowed FROM identity_bindings WHERE provider='feishu' AND instance=? AND subject=?", instance, subject).Scan(&bound, &allowed)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", "", Principal{}, err
	}
	if bound != "" && allowed != 1 {
		return "", "", Principal{}, ErrBindingDisabled
	}
	if email != "" {
		rows, e := s.DB.QueryContext(ctx, "SELECT id FROM users WHERE lower(trim(email))=?", email)
		if e != nil {
			return "", "", Principal{}, e
		}
		count := 0
		for rows.Next() {
			var id string
			if e = rows.Scan(&id); e != nil {
				break
			}
			count++
			if count == 1 {
				matched = id
			}
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return "", "", Principal{}, e
		}
		if count != 1 {
			matched = ""
		}
	}
	if bound != "" {
		if matched != "" && matched != bound {
			return "", "", Principal{}, ErrBindingConflict
		}
		return s.ExternalSession(ctx, "feishu", instance, subject)
	}
	if matched == "" {
		return "", "", Principal{}, ErrEmailUnmatched
	}
	if err = s.DB.QueryRowContext(ctx, "SELECT enabled FROM users WHERE id=?", matched).Scan(&enabled); err != nil || enabled != 1 {
		return "", "", Principal{}, ErrUnauthorized
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return "", "", Principal{}, err
	}
	defer tx.Rollback()
	var other int
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM identity_bindings WHERE provider='feishu' AND instance=? AND user_id=?", instance, matched).Scan(&other); err != nil {
		return "", "", Principal{}, err
	}
	if other > 0 {
		return "", "", Principal{}, ErrBindingConflict
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO identity_bindings(provider,instance,subject,user_id,allowed) SELECT 'feishu',?,?,u.id,1 FROM users u WHERE u.id=? AND u.enabled=1 AND lower(trim(u.email))=?`, instance, subject, matched, email)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return "", "", Principal{}, ErrBindingConflict
		}
		return "", "", Principal{}, err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return "", "", Principal{}, ErrEmailUnmatched
	}
	if err = tx.Commit(); err != nil {
		return "", "", Principal{}, err
	}
	return s.ExternalSession(ctx, "feishu", instance, subject)
}
