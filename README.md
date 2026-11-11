# Rollbook 清册

Rollbook 是面向家庭与小企业的自建服务和资产清册。请只在可信内网运行。

## 本地运行

需要 Go 1.24 或更新版本。

```sh
go test ./...
export ROLLBOOK_AUTH_PEPPER='<在部署环境中设置至少 32 字节的随机值>'
go run ./cmd/rollbook bootstrap-user owner
go run ./cmd/rollbook serve
```

默认监听 `127.0.0.1:8080`，数据库写入 `./data/rollbook.db`。`bootstrap-user` 只允许创建首个用户，密码从终端读取。访问 `/` 可打开清册网页，`/healthz` 返回 `ok`。网页登录的会话 Cookie 默认带 `Secure`，实际访问需要 HTTPS 反向代理。环境变量 `ROLLBOOK_LISTEN_ADDR` 和 `ROLLBOOK_DB_PATH` 可修改监听地址与数据库路径。首次启动自动迁移；再次启动不会重复执行已应用的迁移。数据库使用 WAL、外键约束和 FTS5。

REST `/api/` 与 Streamable HTTP `/mcp` 使用 `Authorization: Bearer <key>`；网页使用用户会话。启动服务或创建首用户须设置至少 32 字节的 `ROLLBOOK_AUTH_PEPPER`，或使用 `ROLLBOOK_AUTH_PEPPER_FILE` 指向密钥文件；两者不能同时设置。文件内容按原始字节读取，不裁剪换行。该值应保存在部署环境中，不要提交到仓库。网页可搜索、浏览站点/主机/网段、查看服务和两层拓扑、审核提议，并管理 key。key 生成后只显示一次。MCP 支持 `initialize`、`tools/list`、`tools/call` 的 JSON 响应模式，工具调用与 REST 共用应用服务和权限检查。`check_versions` 由直接写 key 或网页用户手动触发；服务启动后补查过期项，并每小时扫描一次，确保成功记录不超过 24 小时未检查。

CLI 通过 `ROLLBOOK_URL`（默认 `http://127.0.0.1:8080`）和 `ROLLBOOK_KEY` 连接同一 REST API：

```sh
rollbook search Overleaf
rollbook get service:overleaf
rollbook update service:overleaf 1 '{"name":"Overleaf 2"}' '修正名称'
```

## 上游版本检查

在服务的 `upstream_json` 中配置一个上游，并为服务建立版本记录（首次检查也会自动创建空白版本记录）：

```json
{"provider":"github_releases","repository":"owner/repo"}
{"provider":"docker_hub","image":"namespace/image"}
{"provider":"endoflife_date","product":"go"}
```

只使用固定公开 API 域名，标识符不接受 URL。Docker Hub 从最多 100 个标签中选择最高的纯数字版本，忽略 `latest` 等别名；GitHub 取最新正式 Release；endoflife.date 使用 v1 API 的最新 release。版本比较只接受 `v1.2.3` 这类数字格式，无法判断的版本显示“待人工确认”。上游错误和最近尝试时间另存，不覆盖上次成功值，也不改当前版本、固定版本理由或清册 rev。网页“有新版本”页提供手动重查；REST 为 `POST /api/versions/check`，MCP 为 `check_versions`。GHCR 暂不支持。

## Docker Compose 部署

宿主机需要 Docker Engine 和 Compose 插件。默认镜像是 `ghcr.io/cosmogao/rollbook:${ROLLBOOK_VERSION}`；试运行可用 `latest`，正式部署请在 `.env` 固定已验证的 `vX.Y.Z`。在仓库目录运行：

```sh
cp .env.example .env
mkdir -p secrets
umask 077
openssl rand -hex 32 | tr -d '\n' > secrets/auth_pepper
docker compose up -d
docker compose ps
curl -fsS http://127.0.0.1:8080/healthz
```

`docker compose ps` 的 `rollbook` 应变为 `healthy`，健康接口返回 `ok`。`secret-init` 一次性读取本地 `secrets/auth_pepper`，把密钥复制到仅 UID 10001 可读的 `rollbook-secret` 卷，服务从 `/run/secrets/auth_pepper` 读取；原文件保持 `0600`。此文件与 `.env` 均被 Git 和 Docker 构建忽略。密钥至少 32 字节，必须长期保存，重建容器或升级时继续使用同一文件；换密钥可能使现有凭据失效。请勿在 `.env` 中另设 `ROLLBOOK_AUTH_PEPPER`。

创建首个用户，按提示在终端输入至少 12 字符的密码：

```sh
docker compose run --rm -it rollbook bootstrap-user owner
# 已运行的服务也可使用：
docker compose exec rollbook rollbook bootstrap-user owner
```

两条命令二选一。再次执行会报告用户已存在，不会覆盖密码。容器监听 8080，Compose 默认只把宿主机 `127.0.0.1:8080` 暴露给本机反向代理；在代理上配置 HTTPS 后访问网页。网页会话与 CSRF Cookie 默认带 `Secure`。若必须在可信内网直接用 HTTP 访问，把 `.env` 中 `ROLLBOOK_BIND_ADDR` 设为内网接口地址，并把 `ROLLBOOK_ALLOW_INSECURE_HTTP` 设为 `true` 后运行 `docker compose up -d`。此时页面显示 HTTP 模式提示、日志发出警告，Cookie 不带 `Secure`。**HTTP 模式下口令和 key 会在网络中明文传输，只能在可信内网使用。**不要把此端口直接暴露到公网。

数据库、WAL 与默认的 `rollbook-latest.yaml` 均位于 `/data`，由 Compose 的 `rollbook-data` 卷持久化。`docker compose down && docker compose up -d` 不会删除卷；不要使用 `down -v`。启动后及每 24 小时写一次完整 YAML 备份。需要把 YAML 备份放在独立卷时，在 `.env` 设 `ROLLBOOK_BACKUP_DIR=/backups`；Compose 已为 `/backups` 提供持久卷。自行替换成宿主机绑定目录时，数据库目录与独立备份目录都要持久化，且应由容器内固定的 `10001:10001` 用户读写，例如：

```sh
sudo mkdir -p /srv/rollbook/data /srv/rollbook/backups
sudo chown -R 10001:10001 /srv/rollbook/data /srv/rollbook/backups
```

然后在 `compose.yaml` 将相应卷映射改为 `/srv/rollbook/data:/data` 和 `/srv/rollbook/backups:/backups`。密钥文件保持仅部署管理员可读；初始化后的密钥卷在服务中只读挂载。独立备份卷仅保存最新一份 YAML，应定期复制到其他安全位置。

### 升级和回退

升级前，先备份 `.env`、密钥文件、数据库整个 `/data` 卷，以及启用独立备份目录时的 `/backups` 卷。完整 YAML 含密码与 key 的哈希，也要按敏感数据保存。SQLite 使用 WAL；做文件级快照时先 `docker compose stop rollbook`，把数据库文件及 `-wal`、`-shm` 一同复制，再启动。可用宿主机卷快照工具，或停机后用以下命令复制容器数据：

```sh
docker compose stop rollbook
mkdir -p snapshot-data snapshot-backups
docker compose cp rollbook:/data/. ./snapshot-data/
docker compose cp rollbook:/backups/. ./snapshot-backups/
docker compose start rollbook
```

确认备份可读后，编辑 `.env` 的 `ROLLBOOK_VERSION` 为新版本（如 `v1.2.3`），执行 `docker compose pull && docker compose up -d`，再检查 `docker compose ps`、`/healthz` 和登录。启动会自动运行尚未应用的数据库迁移；每个版本只执行一次，已记录的迁移不会在重启时重复执行。不要在同一数据库上同时运行新旧版本。

**数据库迁移可能不可逆。**如果新版本无法使用，先停服务，把 `/data` 恢复成升级前的完整文件级快照，再把 `.env` 的版本改回旧版并 `docker compose up -d`；不能只改镜像 tag 后让旧程序直接读取已迁移的数据库。若只用 YAML 备份恢复，还须确认旧版本支持其 `schema_version`，且应保留原有用户凭据与密钥。完整卷快照是回退的首选。

GitHub Actions 在 PR 上运行测试、vet 和镜像构建；main 推送发布 `main` 与 `sha-<短提交号>`，语义版本 tag 推送发布 `vX.Y.Z`、`X.Y`、`latest`，均含 amd64 和 arm64。发布工作流不部署服务。

## 备份与恢复

服务启动时及每 24 小时在 `ROLLBOOK_BACKUP_DIR` 指定目录原子写入 `rollbook-latest.yaml`（0600）；未设置时使用数据库目录。网页的「备份与恢复」可下载清册或完整备份、提交 YAML 并查看预览计数；批准后恢复会清除所有会话，需重新登录。完整备份含密码与 key 的哈希，请按敏感文件管理。

CLI 使用 `ROLLBOOK_KEY`：`rollbook export > catalog.yaml` 下载清册；`rollbook import catalog.yaml` 校验并提交待确认导入，网页用户批准后才会恢复。只读 key 可以导出，提议/直接写 key 可以提交导入，任何 key 都无法下载完整备份或批准恢复。未知 `schema_version` 和校验不通过的文件会被拒绝，旧数据库保持不变。
