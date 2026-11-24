package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/CosmoGao/rollbook/internal/auth"
	"github.com/CosmoGao/rollbook/internal/db"
	"github.com/CosmoGao/rollbook/internal/httpapi"
)

func envSecret(name string) (string, error) {
	value, hasValue := os.LookupEnv(name)
	path, hasFile := os.LookupEnv(name + "_FILE")
	if hasValue && hasFile {
		return "", fmt.Errorf("%s and %s_FILE cannot both be set", name, name)
	}
	if hasFile {
		content, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read %s_FILE: %w", name, err)
		}
		return strings.TrimSuffix(strings.TrimSuffix(string(content), "\n"), "\r"), nil
	}
	return value, nil
}
func providerConfig() (httpapi.ProviderConfig, error) {
	var c httpapi.ProviderConfig
	var err error
	c.LDAP = httpapi.LDAPConfig{URL: os.Getenv("ROLLBOOK_LDAP_URL"), BindDN: os.Getenv("ROLLBOOK_LDAP_BIND_DN"), BaseDN: os.Getenv("ROLLBOOK_LDAP_BASE_DN"), Filter: os.Getenv("ROLLBOOK_LDAP_FILTER"), SubjectAttribute: os.Getenv("ROLLBOOK_LDAP_SUBJECT_ATTRIBUTE"), Instance: os.Getenv("ROLLBOOK_LDAP_INSTANCE")}
	c.LDAP.BindPassword, err = envSecret("ROLLBOOK_LDAP_BIND_PASSWORD")
	if err != nil {
		return c, err
	}
	c.OIDC = httpapi.OAuthProviderConfig{Issuer: os.Getenv("ROLLBOOK_OIDC_ISSUER"), ClientID: os.Getenv("ROLLBOOK_OIDC_CLIENT_ID"), RedirectURL: os.Getenv("ROLLBOOK_OIDC_REDIRECT_URL")}
	c.OIDC.ClientSecret, err = envSecret("ROLLBOOK_OIDC_CLIENT_SECRET")
	if err != nil {
		return c, err
	}
	c.Feishu = httpapi.OAuthProviderConfig{ClientID: os.Getenv("ROLLBOOK_FEISHU_CLIENT_ID"), RedirectURL: os.Getenv("ROLLBOOK_FEISHU_REDIRECT_URL")}
	c.Feishu.ClientSecret, err = envSecret("ROLLBOOK_FEISHU_CLIENT_SECRET")
	if err != nil {
		return c, err
	}
	return c, c.Validate()
}
func environmentProviderOwnership() map[string]bool {
	owned := map[string]bool{}
	for _, kind := range []string{"ldap", "oidc", "feishu"} {
		prefix := "ROLLBOOK_" + strings.ToUpper(kind) + "_"
		for _, entry := range os.Environ() {
			if strings.HasPrefix(entry, prefix) {
				owned[kind] = true
				break
			}
		}
	}
	return owned
}
func authCommand(args []string) error {
	usage := errors.New("usage: rollbook auth user-create <login> | user-enable <login> <true|false> | bind <ldap|oidc|feishu> <instance> <subject> <login> | allow <ldap|oidc|feishu> <instance> <subject> <login> <true|false> | local-login <enable|disable>")
	if len(args) == 0 {
		return usage
	}
	path := os.Getenv("ROLLBOOK_DB_PATH")
	if path == "" {
		path = "./data/rollbook.db"
	}
	if _, err := os.Stat(path); err != nil {
		return err
	}
	database, err := db.Open(context.Background(), path)
	if err != nil {
		return err
	}
	defer database.Close()
	service := auth.Service{DB: database}
	ctx := context.Background()
	if !(args[0] == "local-login" && len(args) == 2 && args[1] == "enable") {
		c, err := providerConfig()
		if err != nil {
			return err
		}
		pepper, err := authPepper()
		if err != nil {
			return err
		}
		registry, err := httpapi.NewProviderRegistry(ctx, database, pepper, c, environmentProviderOwnership())
		if err != nil {
			return err
		}
		service.EnabledProvider = registry.Enabled
	}
	switch args[0] {
	case "user-create":
		if len(args) != 2 {
			return usage
		}
		return service.CreateExternalUser(ctx, args[1])
	case "user-enable":
		if len(args) != 3 {
			return usage
		}
		if args[2] != "true" && args[2] != "false" {
			return usage
		}
		return service.SetUserEnabled(ctx, args[1], args[2] == "true")
	case "bind":
		if len(args) != 5 {
			return usage
		}
		return service.ManageBinding(ctx, args[1], args[2], args[3], args[4], true)
	case "allow":
		if len(args) != 6 {
			return usage
		}
		if args[5] != "true" && args[5] != "false" {
			return usage
		}
		return service.ManageBinding(ctx, args[1], args[2], args[3], args[4], args[5] == "true")
	case "local-login":
		if len(args) != 2 {
			return usage
		}
		if args[1] != "enable" && args[1] != "disable" {
			return usage
		}
		return service.SetLocalEnabled(ctx, args[1] == "enable")
	}
	return usage
}
