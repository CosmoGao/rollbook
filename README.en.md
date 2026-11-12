# Rollbook

Rollbook is a self-hosted service and asset catalog for a home or small business. Run it only on a trusted network. The web interface supports English and Chinese. It follows your browser language until you choose a language, then remembers your choice on that browser.

## Quick start

Go 1.24 or newer is required.

```sh
go test ./...
export ROLLBOOK_AUTH_PEPPER='<set at least 32 random bytes in the deployment environment>'
go run ./cmd/rollbook bootstrap-user owner
go run ./cmd/rollbook serve
```

The default address is `127.0.0.1:8080`; the database is `./data/rollbook.db`. The first user can be created interactively or at startup with `ROLLBOOK_BOOTSTRAP_USER` and either `ROLLBOOK_BOOTSTRAP_PASSWORD` or `ROLLBOOK_BOOTSTRAP_PASSWORD_FILE`. The password must be at least 12 characters. For Compose, use the optional `compose.bootstrap.yaml` override and a `secrets/bootstrap_password` file; the default Compose setup requires no bootstrap secret. Remove the bootstrap settings and file after the first login. Existing users and passwords are never changed by startup bootstrap. Set up HTTPS before signing in from another machine. The session cookie is `Secure` by default. Set `ROLLBOOK_LISTEN_ADDR` and `ROLLBOOK_DB_PATH` to change the address and database path. Keep the pepper outside the repository; `ROLLBOOK_AUTH_PEPPER_FILE` is also supported.

## API, MCP, and CLI

REST is served under `/api/` and Streamable HTTP MCP under `/mcp`. Both accept `Authorization: Bearer <key>`; the web interface uses a user session. Create a key in the web interface and copy it immediately, because it is shown once. `read` keys can query; `propose` keys submit changes for human approval; `write` keys apply changes directly. Never store passwords or tokens in catalog notes or reasons. Credential objects store references to your own vault.

Install the CLI from a release or build it with `go build ./cmd/rollbook`. Run `rollbook login` to save a server address and key, or set `ROLLBOOK_URL` and `ROLLBOOK_KEY` for the current process. The CLI uses the same REST API. MCP tools include `search`, `get_asset`, `get_service`, `create`, and `update`; inspect `tools/list` for full schemas. See [`skills/rollbook/SKILL.md`](skills/rollbook/SKILL.md) for agent workflows.

Create and update requests accept English enum aliases. For example, `active` maps to `在用` for `status`, and `physical` maps to `物理机` for a host's `kind`. Responses and stored values stay in Chinese for compatibility. MCP schemas list accepted aliases. REST errors use English when you send `Accept-Language: en` or `X-Rollbook-Language: en`; codes and HTTP statuses stay the same. Chinese requests and older clients continue to work.

## Security and deployment

Use HTTPS, a trusted reverse proxy, and least-privilege keys. The server rejects non-HTTP(S) URL fields and filters URL links in the web interface. Backups can contain account and key hashes; protect them accordingly. Read the [Chinese README](README.md) for the complete configuration, backup, restore, and migration procedures.

Licensed under Apache-2.0.
