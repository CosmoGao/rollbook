package catalog

import "github.com/CosmoGao/rollbook/internal/secrets"

var ErrSecret = secrets.ErrSecret

func CheckSecrets(v any) error { return secrets.Check(v) }
