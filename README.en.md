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

The default address is `127.0.0.1:8080`; the database is `./data/rollbook.db`. The first user can be created interactively or at startup with `ROLLBOOK_BOOTSTRAP_USER` and either `ROLLBOOK_BOOTSTRAP_PASSWORD` or `ROLLBOOK_BOOTSTRAP_PASSWORD_FILE`. New passwords must have at least 8 characters and use at least three of uppercase, lowercase, digits and symbols. They must not contain the username (case insensitive) or match a common password, and must not exceed the bcrypt limit of 72 bytes. Existing passwords continue to work. The same rule applies to interactive setup, password reset and changing a password in the web interface. For Compose, use the optional `compose.bootstrap.yaml` override and a `secrets/bootstrap_password` file; the default Compose setup requires no bootstrap secret. Remove the bootstrap settings and file after the first login. Existing users and passwords are never changed by startup bootstrap. Set up HTTPS before signing in from another machine. The session cookie is `Secure` by default. Set `ROLLBOOK_LISTEN_ADDR` and `ROLLBOOK_DB_PATH` to change the address and database path. Keep the pepper outside the repository; `ROLLBOOK_AUTH_PEPPER_FILE` is also supported.

## API, MCP, and CLI

REST is served under `/api/` and Streamable HTTP MCP under `/mcp`. Both accept `Authorization: Bearer <key>`; the web interface uses a user session. Create a key in the web interface and copy it immediately, because it is shown once. `read` keys can query; `propose` keys submit changes for human approval; `write` keys apply changes directly. Never store passwords or tokens in catalog notes or reasons. Credential objects store references to your own vault.

Install the CLI from a release or build it with `go build ./cmd/rollbook`. Run `rollbook login` to save a server address and key, or set `ROLLBOOK_URL` and `ROLLBOOK_KEY` for the current process. The CLI uses the same REST API. MCP tools include `search`, `get_asset`, `get_service`, `create`, and `update`; inspect `tools/list` for full schemas. See [`skills/rollbook/SKILL.md`](skills/rollbook/SKILL.md) for agent workflows.

The sign-in page and signed-in footer display the same build version from `GET /api/version`. Tag builds show `Rollbook vX.Y.Z`; main builds show `Rollbook main` followed by the seven-character commit ID.

Create and update requests accept English enum aliases. For example, `active` maps to `在用` for `status`, and `physical` maps to `物理机` for a host's `kind`. Responses and stored values stay in Chinese for compatibility. MCP schemas list accepted aliases. REST errors use English when you send `Accept-Language: en` or `X-Rollbook-Language: en`; codes and HTTP statuses stay the same. Chinese requests and older clients continue to work.

## Security and deployment

Use HTTPS, a trusted reverse proxy, and least-privilege keys. For TLS termination, set `ROLLBOOK_TRUSTED_PROXY_CIDRS` to the immediate proxy's narrow TCP address range (for example `127.0.0.1/32`) and configure Zoraxy to overwrite `X-Forwarded-Proto`, or Nginx with `proxy_set_header X-Forwarded-Proto $scheme;`. Remove or normalize client-supplied `X-Forwarded-Proto` and `Forwarded` headers at the proxy. Only headers from trusted TCP peers affect the protocol decision; direct TLS is always secure. The warning and session/CSRF cookie `Secure` flags follow each request. Direct HTTP login requires `ROLLBOOK_ALLOW_INSECURE_HTTP=true` and exposes credentials on the network. The server rejects non-HTTP(S) URL fields and filters URL links in the web interface. Backups can contain account and key hashes; protect them accordingly. Read the [Chinese README](README.md) for the complete configuration, backup, restore, and migration procedures.

Licensed under Apache-2.0.

## Web development

The web source lives in `web/src/` and uses React, TypeScript, and Vite. Run `npm ci --prefix web` once, then `npm run typecheck --prefix web` and `npm run build --prefix web` after changes. Commit the generated `web/dist/` assets. Go embeds them, so `go build ./cmd/rollbook` works without Node. Rebuild before committing to keep `web/dist/` in sync.
