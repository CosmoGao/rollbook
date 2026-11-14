# Rollbook

Rollbook is a self-hosted service and asset catalog for homes and small businesses. Run it on a trusted network. The UI follows the browser language until you select English or Chinese; that choice is saved in the browser. See the [Chinese README](README.md), [changelog](CHANGELOG.md), and [release checklist](docs/releasing.md).

## Run locally

Go 1.24 or newer is required. From the repository root:

```sh
export ROLLBOOK_AUTH_PEPPER="$(openssl rand -hex 32)"
go run ./cmd/rollbook bootstrap-user owner
go run ./cmd/rollbook serve
```

The first command creates a pepper for this local shell. Keep the same pepper across restarts in a real deployment; a changed pepper can invalidate credentials. `bootstrap-user` reads the password interactively. The default address is `127.0.0.1:8080`, the database is `./data/rollbook.db`, `/healthz` returns `ok`, and `/` serves the UI. `ROLLBOOK_LISTEN_ADDR` and `ROLLBOOK_DB_PATH` override the defaults. Startup applies forward-only database migrations. SQLite uses WAL, foreign keys, and FTS5.

The first account can also be created on the first `serve` by setting `ROLLBOOK_BOOTSTRAP_USER` and exactly one of `ROLLBOOK_BOOTSTRAP_PASSWORD` or `ROLLBOOK_BOOTSTRAP_PASSWORD_FILE`. Remove these settings after setup. Existing accounts and passwords are never replaced by startup bootstrap. `ROLLBOOK_AUTH_PEPPER` must be at least 32 bytes, or use `ROLLBOOK_AUTH_PEPPER_FILE`; do not set both. The file is read as raw bytes, including a trailing newline. Never commit the pepper.

New and reset passwords need at least eight characters and at least three of uppercase, lowercase, digits, and symbols. They cannot contain the username (case insensitive), match common passwords, or exceed bcrypt's 72-byte limit. Existing passwords remain valid. Changing a password in the UI revokes all sessions. A local administrator can run `rollbook reset-password owner`; it reads the new password interactively, operates on the existing `ROLLBOOK_DB_PATH`, and revokes that user's sessions. The login limiter counts by normalized account and client IP; by default five failures lock login for 15 minutes, with exponential backoff up to an hour. `ROLLBOOK_LOGIN_THRESHOLD`, `ROLLBOOK_LOGIN_LOCK_DURATION`, and `ROLLBOOK_LOGIN_MAX_BACKOFF` adjust this. Only configure `ROLLBOOK_TRUSTED_PROXY_CIDRS` for a trusted single-hop proxy; otherwise the server uses the TCP peer IP and ignores `X-Forwarded-For`.

REST is under `/api/`, Streamable HTTP MCP under `/mcp`; both use `Authorization: Bearer <key>`. The UI uses a session cookie that is `Secure` by default. Key values are displayed only once. A `read` key queries; `propose` submits changes for web approval; `write` applies changes directly. Never store passwords or tokens in catalog notes or reasons. Credential objects should refer to a separate vault. Pending imports are limited to ten per submitter and 100 globally and expire after seven days; startup and hourly cleanup remove the source file while retaining audit metadata.

## Web UI (v0.6.0)

The six main menu items are **Overview, Assets, Topology, To do, Agent access, Settings**. Global search sits in the top bar; mobile navigation uses a drawer. Overview surfaces proposals, new upstream versions, and assets needing verification, then shows hosts, services, and access points by site. Search accepts IPs, domains, ports, and service names, and shows related hosts, services, endpoints, and access paths. Only safe HTTP(S) URLs become links.

Assets lets you create and browse sites, hosts, networks, services, and other records. Hosts have list and tree views; site browsing shows uplinks, networks, and routers. Details include deployment, dependencies, endpoints, versions, notes, and changes. Service and host details show recursive impact up front. Deactivating a host first previews affected children, services, and endpoints; the options apply only to direct objects of that host. Topology has layered graph, access path, and impact views, site filtering, and focus links from details. To do groups proposals with diffs, assets to verify, and new versions; bulk rejection requires confirmation. Agent access holds keys, MCP configuration, Skill, and CLI links. Settings holds account, language, and backup and restore.

## Install and version

Tagged releases provide a multi-architecture GHCR image, Linux/macOS/Windows amd64 and arm64 CLI archives, `SHA256SUMS`, and `install.sh`. Linux/macOS users can download and run the release's `install.sh` to install under `~/.local/bin`; set `ROLLBOOK_VERSION=v0.6.0` to pin or `ROLLBOOK_INSTALL_DIR` to choose a destination. Windows users should choose an archive and verify its checksum. The installer only replaces the local CLI. `rollbook version`, the sign-in screen, footer, `GET /api/version`, and MCP `initialize.serverInfo` report the same build version.

YAML backups use `schema_version: 1`; unsupported versions are rejected. Newer databases may not be readable by older binaries. Make a full file-level snapshot before upgrading. The CLI uses REST. Run `rollbook login` to enter a server URL and key without echoing the key; the profile is stored in the user config directory with mode `0600`. `rollbook --profile home login` creates another profile. `ROLLBOOK_URL` and `ROLLBOOK_KEY` override the chosen profile for a process. With no configured URL, it defaults to `http://127.0.0.1:8080`. `--output json|yaml|table` controls REST output; `export` always writes YAML. `rollbook completion bash|zsh|fish` generates shell completion.

```sh
rollbook search Overleaf
rollbook get service:overleaf
rollbook --output table site location:home
rollbook topology service:overleaf --depth 2
rollbook topology service:overleaf --view access
rollbook versions --outdated
rollbook changes --pending
rollbook context get
```

For writes, use `rollbook create @asset.json 'reason'`, `rollbook update <id> --set name=value --reason 'reason'`, or `rollbook propose <id> --set name=value --reason 'reason'`. `rollbook note` and `rollbook verify` are also available. When a command omits `--rev`, the CLI reads the current revision first; retry after a 409 conflict by fetching the object again. REST and MCP create/update accept English enum aliases such as `active` and `physical`, but responses and stored values remain Chinese for compatibility. REST errors honor `Accept-Language: en` or `X-Rollbook-Language: en` without changing codes or status. MCP also supports resources and prompts; inspect `tools/list` for schemas.

## From an empty catalog to the first query

1. Start the server, create the first user, and sign in. The empty catalog offers a first site or clearly fictional sample data using documentation IPs and `.invalid` domains. You can also create a site in Assets.
2. In **Agent access**, create a `read` key and copy it immediately. The page shows HTTP MCP configuration for the current site URL, stdio configuration, a Skill zip download, and a link to the matching CLI Release. Configuration snippets use a placeholder by default; the one-time key appears only if explicitly selected in its creation dialog. Keep snippets containing keys in protected local files.
3. Download `rollbook-skill.zip` while signed in, extract `rollbook/SKILL.md` into the client's skills directory, and configure MCP. For example, Codex can use `~/.agents/skills/rollbook/`, Claude Code `~/.claude/skills/rollbook/`, and Cursor `.cursor/skills/rollbook/`. The zip requires a web session. The source Skill is also at [`skills/rollbook/SKILL.md`](skills/rollbook/SKILL.md).

For an HTTP MCP client, point its Streamable HTTP connection at `https://your-rollbook-host/mcp` and send `Authorization: Bearer <key>`. For a stdio-only client, install the CLI, run `rollbook login` as the same user, then configure `{"mcpServers":{"rollbook":{"command":"rollbook","args":["mcp"]}}}`. Alternatively set `ROLLBOOK_URL` and `ROLLBOOK_KEY` for that client process. The bridge reads newline-delimited JSON-RPC and writes only protocol messages to stdout. A `propose` key permits agent-submitted changes for human review; grant a `write` key only to a trusted agent.

## Upstream version checks

Set a service's `upstream_json` to a supported public provider, for example `{"provider":"github_releases","repository":"owner/repo"}`, `{"provider":"docker_hub","image":"namespace/image"}`, or `{"provider":"endoflife_date","product":"go"}`. Identifiers are not arbitrary URLs. GitHub uses the latest stable Release, Docker Hub compares suitable tags among at most 100 tags, and endoflife.date uses its v1 API. `tag_prefix` and `tag_regex` can narrow tags. Automatic checks run at startup and hourly; successfully checked records are rechecked within 24 hours. A web user or direct-write key can trigger `POST /api/versions/check` or MCP `check_versions`. Failed checks preserve the last successful value. Optional Feishu/WeCom webhook alerts are off by default, send at most a daily summary for updates or approaching end of support, and do not export webhook URLs in YAML backups.

## Docker Compose deployment

Requires Docker Engine and Compose. In the repository root, pin `ROLLBOOK_VERSION=v0.6.0` in `.env` for production; the example defaults to `latest` for a trial. Run:

```sh
cp .env.example .env
mkdir -p secrets
umask 077
openssl rand -hex 32 | tr -d '\n' > secrets/auth_pepper
docker compose up -d
docker compose ps
curl -fsS http://127.0.0.1:8080/healthz
```

The `rollbook` container should become healthy and `/healthz` return `ok`. `secret-init` copies the pepper into a volume readable only by UID 10001; `rollbook` mounts it read-only. Keep the original file, permissions, and pepper stable across rebuilds. The default Compose binds port 8080 to loopback and persists `/data` and `/backups`. `docker compose down` preserves volumes; never use `down -v` for an upgrade.

To create the first admin with a Docker secret, set `ROLLBOOK_BOOTSTRAP_USER=owner` in `.env`, then run:

```sh
umask 077
read -rsp 'New admin password: ' bootstrap_password; printf '\n'
printf '%s' "$bootstrap_password" > secrets/bootstrap_password
unset bootstrap_password
docker compose -f compose.yaml -f compose.bootstrap.yaml up -d
```

After the first sign-in, remove the bootstrap setting and password file and run `docker compose up -d`. The optional override is not needed for normal startup. Alternatively, run `docker compose run --rm -it rollbook bootstrap-user owner` and enter a new password interactively. Do not set both bootstrap password sources. Existing users are never overwritten.

Put HTTPS in front of the service before signing in from another machine. With TLS termination at a reverse proxy, add `ROLLBOOK_TRUSTED_PROXY_CIDRS: ${ROLLBOOK_TRUSTED_PROXY_CIDRS:-}` to `rollbook.environment` in `compose.yaml`, and set the variable in `.env` to the proxy's *immediate TCP source* CIDR, such as `127.0.0.1/32` for a same-host proxy. The shipped Compose file does not pass this variable through automatically. Configure the proxy to overwrite `X-Forwarded-Proto` and remove or normalize client-supplied `X-Forwarded-Proto` and `Forwarded` headers. For Nginx, use `proxy_set_header X-Forwarded-Proto $scheme;`. The server only trusts headers from configured TCP peers; direct TLS remains secure. Protocol warnings and session/CSRF cookie `Secure` flags follow each request. Do not expose the backend publicly.

For direct HTTP testing only on a trusted network, set `ROLLBOOK_BIND_ADDR` to the desired host interface and `ROLLBOOK_ALLOW_INSECURE_HTTP=true` in `.env`, then run `docker compose up -d`. Otherwise HTTP login is rejected. HTTP sends passwords and keys in plaintext.

### Upgrade and rollback

Before upgrading, save `.env`, the pepper file, the full `/data` volume, and `/backups` if used separately. SQLite uses WAL, so stop Rollbook before a file-level snapshot and preserve the database together with its `-wal` and `-shm` files. For example:

```sh
docker compose stop rollbook
mkdir -p snapshot-data snapshot-backups
docker compose cp rollbook:/data/. ./snapshot-data/
docker compose cp rollbook:/backups/. ./snapshot-backups/
docker compose start rollbook
```

Check that the snapshot is readable. Change `ROLLBOOK_VERSION` in `.env` to the intended tag, run `docker compose pull && docker compose up -d`, then check `docker compose ps`, `/healthz`, and web sign-in. Do not run two versions against one database. Migrations are forward-only; for rollback, stop the service, restore the complete pre-upgrade `/data` snapshot, set the old image tag, then start it. Merely switching tags against a migrated database is unsafe. If using YAML instead, first verify that the old binary supports its `schema_version` and preserve the original credentials and pepper. A full volume snapshot is preferred. GitHub Actions tests PRs and publishes images and archives on tags; the workflow does not deploy your instance.

## Backup and restore

Startup and daily backups write timestamped full YAML files (mode `0600`) to `ROLLBOOK_BACKUP_DIR` or the database directory. A weekly backup is also written on first startup and every seven days. Defaults retain seven daily and four weekly backups. `ROLLBOOK_BACKUP_INTERVAL` and `ROLLBOOK_BACKUP_WEEKLY_INTERVAL` accept Go durations; `ROLLBOOK_BACKUP_DAILY_KEEP` and `ROLLBOOK_BACKUP_WEEKLY_KEEP` set positive retention counts. The UI's **Settings → Backup and restore** can download catalog or full YAML, inspect managed backups, submit YAML for merge/catalog/full restore, and review previews. Full YAML includes account and key hashes and is available only to signed-in web users. Approval checks revisions again, saves a pre-restore snapshot, and revokes all sessions after restore.

A catalog export is for asset migration. CLI examples: `rollbook export > catalog.yaml` downloads a catalog; `rollbook import catalog.yaml` validates and submits it for web approval. A read key can export; propose/write keys can submit, but no key can download a full backup or approve a restore. Unsupported `schema_version` is rejected. A YAML merge adds or updates assets while retaining others and history; catalog restore replaces the catalog; full restore also replaces accounts and keys.

CSV merge import supports `host`, `network`, `ip`, and `service`. Download a UI template or run `rollbook template csv host > hosts.csv`, fill it, then run `rollbook import csv host hosts.csv`; substitute another supported type as needed. Submission creates a preview and approval item. Files are limited to 16 MiB and follow the same pending/expiry limits as YAML. CSV is UTF-8 (optional BOM) with exact, case-sensitive template headers. IDs require type prefixes such as `host:`. For updates supply the current positive `rev`; leave it empty for new objects. An empty cell leaves an existing field unchanged, except required fields cannot be empty. JSON cells must be valid JSON; URL fields must be absolute HTTP(S) URLs; references may name existing objects or objects in the same import. Duplicate IDs and invalid columns or references produce located errors.

## Web development

The UI lives in `web/src/` and uses React, TypeScript, and Vite. Run `npm ci --prefix web`, then `npm run typecheck --prefix web` and `npm run build --prefix web` after changes. Commit generated `web/dist/` assets with UI changes; Go embeds them, so `go build ./cmd/rollbook` does not need Node.

Licensed under Apache-2.0.
