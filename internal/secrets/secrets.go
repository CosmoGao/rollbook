package secrets

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
)

var ErrSecret = errors.New("content contains a suspected secret")
var assignment = regexp.MustCompile(`(?i)(?:password|passwd|pwd|api[_-]?key|access[_-]?token|secret|client[_-]?secret)\s*[:=]`)
var privateKey = regexp.MustCompile(`(?i)-----BEGIN (?:[A-Z0-9 ]+ )?PRIVATE KEY-----`)
var bearerToken = regexp.MustCompile(`(?i)bearer\s+[a-z0-9._~+/-]{16,}`)
var knownToken = regexp.MustCompile(`(?:gh[pousr]_[A-Za-z0-9_]{20,}|github_pat_[A-Za-z0-9_]{20,}|xox[baprs]-[A-Za-z0-9-]{20,}|AKIA[0-9A-Z]{16}|eyJ[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{10,})`)

// Check recursively checks every user-controlled text value before persistence.
// The error deliberately omits the matched text.
func Check(v any) error {
	switch x := v.(type) {
	case string:
		if privateKey.MatchString(x) || assignment.MatchString(x) || bearerToken.MatchString(x) || knownToken.MatchString(x) {
			return ErrSecret
		}
		if strings.HasPrefix(strings.TrimSpace(x), "sk-") && len(strings.TrimSpace(x)) > 20 {
			return ErrSecret
		}
		if json.Valid([]byte(x)) && (strings.HasPrefix(strings.TrimSpace(x), "{") || strings.HasPrefix(strings.TrimSpace(x), "[")) {
			var decoded any
			if json.Unmarshal([]byte(x), &decoded) == nil {
				return Check(decoded)
			}
		}
	case map[string]any:
		for k, val := range x {
			lower := strings.ToLower(k)
			if lower == "password" || lower == "token" || lower == "secret" || lower == "private_key" || lower == "api_key" {
				return ErrSecret
			}
			if err := Check(val); err != nil {
				return err
			}
		}
	case []any:
		for _, val := range x {
			if err := Check(val); err != nil {
				return err
			}
		}
	case []string:
		for _, val := range x {
			if err := Check(val); err != nil {
				return err
			}
		}
	}
	return nil
}
