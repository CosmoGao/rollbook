package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateNewPassword(t *testing.T) {
	for _, tc := range []struct {
		name, login, password string
		want                  PasswordFailure
	}{
		{"seven", "owner", "Abcd1!x", PasswordTooShort},
		{"two classes", "owner", "abcdef12", PasswordTooSimple},
		{"eight, three classes", "owner", "Abcdef12", ""},
		{"contains canonical login", "owner", "xxOwNeR1!", PasswordContainsLogin},
		{"common mixed case", "owner", "PaSsWoRd1", PasswordCommon},
		{"common digits", "owner", "12345678", PasswordCommon},
		{"common keyboard", "owner", "QwErTy123", PasswordCommon},
		{"72 bytes", "owner", strings.Repeat("A", 69) + "a1!", ""},
		{"73 bytes", "owner", strings.Repeat("A", 70) + "a1!", PasswordTooLong},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateNewPassword(tc.login, tc.password)
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var policy *PasswordError
			if !errors.As(err, &policy) || policy.Reason != tc.want || !errors.Is(err, ErrInvalid) {
				t.Fatalf("got %v, want %s", err, tc.want)
			}
			if strings.Contains(err.Error(), tc.password) {
				t.Fatal("password leaked in error")
			}
		})
	}
}
