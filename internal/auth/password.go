package auth

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

type PasswordFailure string

const (
	PasswordTooShort      PasswordFailure = "too_short"
	PasswordTooLong       PasswordFailure = "too_long"
	PasswordTooSimple     PasswordFailure = "too_simple"
	PasswordContainsLogin PasswordFailure = "contains_login"
	PasswordCommon        PasswordFailure = "common"
)

// PasswordError 只携带失败代码，不保存口令或用户名。
type PasswordError struct{ Reason PasswordFailure }

func (e *PasswordError) Error() string {
	switch e.Reason {
	case PasswordTooShort:
		return "password must contain at least 8 characters"
	case PasswordTooLong:
		return "password must not exceed 72 bytes"
	case PasswordTooSimple:
		return "password must use at least three of uppercase, lowercase, digits and symbols"
	case PasswordContainsLogin:
		return "password must not contain the username"
	case PasswordCommon:
		return "password is too common"
	default:
		return fmt.Sprintf("invalid password: %s", e.Reason)
	}
}

func (e *PasswordError) Is(target error) bool { return target == ErrInvalid }

var commonPasswords = map[string]bool{"password1": true, "12345678": true, "qwerty123": true}

// ValidateNewPassword 校验新设口令；login 必须由服务端从账号取得或规范化。
func ValidateNewPassword(login, password string) error {
	if utf8.RuneCountInString(password) < 8 {
		return &PasswordError{PasswordTooShort}
	}
	if len([]byte(password)) > 72 {
		return &PasswordError{PasswordTooLong}
	}
	if normalized := strings.ToLower(strings.TrimSpace(login)); normalized != "" && strings.Contains(strings.ToLower(password), normalized) {
		return &PasswordError{PasswordContainsLogin}
	}
	if commonPasswords[strings.ToLower(password)] {
		return &PasswordError{PasswordCommon}
	}
	var upper, lower, digit, symbol bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsLower(r):
			lower = true
		case unicode.IsDigit(r):
			digit = true
		default:
			symbol = true
		}
	}
	classes := 0
	for _, present := range []bool{upper, lower, digit, symbol} {
		if present {
			classes++
		}
	}
	if classes < 3 {
		return &PasswordError{PasswordTooSimple}
	}
	return nil
}
