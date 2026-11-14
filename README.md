# Rollbook 清册

Rollbook 是面向家庭与小企业的自建服务和资产清册。请只在可信内网运行。英文说明见 [README.en.md](README.en.md)，版本变化见 [CHANGELOG.md](CHANGELOG.md)，发版检查见 [docs/releasing.md](docs/releasing.md)。

## 本地运行

需要 Go 1.24 或更新版本。

```sh
go test ./...
export ROLLBOOK_AUTH_PEPPER="$(openssl rand -hex 32)"
go run ./cmd/rollbook bootstrap-user owner
go run ./cmd/rollbook serve
```

上述临时密钥只供本地试用；正式部署须长期保存同一密钥。默认监听 `127.0.0.1:8080`，数据库写入 `./data/rollbook.db`。`bootstrap-user` 只允许创建首个用户，密码从终端读取；也可在首次 `serve` 前配置 `ROLLBOOK_BOOTSTRAP_USER` 与 `ROLLBOOK_BOOTSTRAP_PASSWORD` 或 `ROLLBOOK_BOOTSTRAP_PASSWORD_FILE` 自动创建。访问 `/` 可打开清册网页，`/healthz` 返回 `ok`。网页登录的会话 Cookie 默认带 `Secure`，实际访问需要 HTTPS 反向代理。环境变量 `ROLLBOOK_LISTEN_ADDR` 和 `ROLLBOOK_DB_PATH` 可修改监听地址与数据库路径。首次启动自动迁移；再次启动不会重复执行已应用的迁移。数据库使用 WAL、外键约束和 FTS5。

新设或重设密码不得包含用户名（不区分大小写）、不得使用常见弱口令，且不超过 bcrypt 的 72 字节上限；已有密码无需修改。网页侧边栏可修改密码（须输入旧密码）或退出所有设备；两种操作都撤销当前及其他登录会话，随后重新登录。本机管理员可运行 `rollbook reset-password <login>`，在终端交互输入至少 8 字符，且大写、小写、数字、符号四类至少三类的新密码。此命令只操作 `ROLLBOOK_DB_PATH` 指向的已有数据库文件，不接受命令行明文密码，成功后撤销该账号的所有会话。重置后该账号可从重置前被锁定的 IP 使用新密码立即登录；其他账号的 IP 限速不受重置影响。

登录失败按规范化账号和客户端 IP 分别计数，默认第 5 次失败锁定 15 分钟，指数退避上限 1 小时；可通过 `ROLLBOOK_LOGIN_THRESHOLD`、`ROLLBOOK_LOGIN_LOCK_DURATION`、`ROLLBOOK_LOGIN_MAX_BACKOFF` 调整。日志记录失败及锁定类别，不记录口令、Cookie 或 Key。默认只使用 TCP 对端 IP，忽略 `X-Forwarded-For`。仅在单跳可信反向代理部署时，才将代理地址段填入 `ROLLBOOK_TRUSTED_PROXY_CIDRS`（逗号分隔的 CIDR）；届时采用代理追加的最后一个 `X-Forwarded-For` 地址。多跳代理应先在入口规范化转发头。清册 URL 字段仅接受绝对 HTTP/HTTPS 地址，页面展示链接时再次检查协议。页面响应带固定 CSP、防嵌入、防 MIME 嗅探及无来源引用头。

待处理导入最多每提交者 10 份、全局 100 份；提交 7 天后自动过期并清除原始 YAML，服务启动及每小时清理一次。已批准、驳回和过期的记录保留状态、提交者、时间及审批人等元数据，原始文件不再保留。

REST `/api/` 与 Streamable HTTP `/mcp` 使用 `Authorization: Bearer <key>`；网页使用用户会话。启动服务或创建首用户须设置至少 32 字节的 `ROLLBOOK_AUTH_PEPPER`，或使用 `ROLLBOOK_AUTH_PEPPER_FILE` 指向密钥文件；两者不能同时设置。文件内容按原始字节读取，不裁剪换行。该值应保存在部署环境中，不要提交到仓库。网页可搜索和维护资产、查看分层拓扑与递归影响面、审核提议并管理 Key。key 生成后只显示一次。MCP 支持 `initialize`、`tools/list`、`tools/call` 的 JSON 响应模式，工具调用与 REST 共用应用服务和权限检查。`check_versions` 由直接写 key 或网页用户手动触发；服务启动后补查过期项，并每小时扫描一次，确保成功记录不超过 24 小时未检查。

CLI 使用与服务端相同的 REST API。首次运行 `rollbook login`，依次输入服务地址和网页生成的 Key；交互输入 Key 时终端不回显。配置写入 `~/.config/rollbook/config.toml`（或系统的用户配置目录），文件权限为 `0600`。用 `rollbook --profile home login` 保存另一套配置；最近登录的 profile 是默认值。每次调用可用 `--profile` 选择。环境变量 `ROLLBOOK_URL`、`ROLLBOOK_KEY` 分别覆盖选中 profile 的地址和 Key；地址都未设置时默认 `http://127.0.0.1:8080`。环境变量只对当前进程生效。无 Key 时 `--help`、`version` 和 `completion` 仍可运行。

网页登录后可从「Agent 接入」页复制基于当前访问地址的 HTTP/stdio MCP 配置、下载 `rollbook-skill.zip`，或前往当前版本的 CLI Release。Skill zip 仅允许网页登录会话下载，内容固定为 `rollbook/SKILL.md`；Key 占位符需在本机替换。新 Key 的明文只显示一次，配置片段默认保留占位符，选择临时带入后关闭对话框即清除。

仅支持 stdio 的 MCP 客户端可配置 `{"mcpServers":{"rollbook":{"command":"rollbook","args":["mcp"]}}}`；先在同一用户下执行 `rollbook login`，或设置 `ROLLBOOK_URL` 和 `ROLLBOOK_KEY`。多套清册可在 `args` 中写入 `["--profile","home","mcp"]`。桥接按 JSON-RPC 每行一条消息读写，标准输出只包含协议响应，错误写入标准错误；客户端关闭 stdin 后退出。远端须提供 Rollbook 的 `/mcp` JSON 响应模式。

## 网页使用（v0.6.0）

主菜单六项为「总览、资产、拓扑、待处理、Agent 接入、设置」；全局搜索在顶栏，手机端由抽屉菜单进入。总览先列出待确认提议、有新版本和待核实资产的任务入口，再按站点显示主机、服务及入口，可直接跳转拓扑或搜索设备与服务。搜索支持 IP、域名、端口、服务名等，结果展示相关主机、服务、入口和访问方式；只把安全的 HTTP/HTTPS URL 渲染为链接。

「资产」可按类型录入并浏览站点、主机、网段、服务等；主机可切换列表与树，站点视图可查看接入、网段和路由设备。资产详情展示部署与依赖、访问入口、版本、笔记和变更记录；服务与主机可在详情第一屏查看影响面。主机停用时先预览子主机、服务和入口影响；停用选项只作用于当前主机的直接对象。

「拓扑」提供分层拓扑、访问链和影响面三视图，可按站点筛选并从详情聚焦对象。「待处理」集中待确认提议、待核实资产和有新版本；提议显示差异，批量驳回须确认。「Agent 接入」管理 Key、MCP、Skill 和 CLI。「设置」提供账号、语言及备份恢复入口。浏览器语言决定初始界面语言，手动切换后在该浏览器记住选择。

## 安装与版本

正式 tag `vX.Y.Z` 会在同一构建流程发布 GHCR 镜像、Linux/macOS/Windows 的 amd64/arm64 压缩包、`SHA256SUMS`、`install.sh` 和自动生成的 Release notes。Linux/macOS 可以下载 Release 中的 `install.sh` 后运行，默认安装到 `~/.local/bin`；可通过 `ROLLBOOK_VERSION=v0.6.0` 固定版本，`ROLLBOOK_INSTALL_DIR` 调整安装目录。Windows 请从 Release 下载对应压缩包并核对 SHA256。安装程序只替换本机 CLI 二进制，不更改数据库。`rollbook version`、网页登录页与登录后的页脚、`GET /api/version` 与 MCP `initialize.serverInfo` 使用同一构建版本；网页中 tag 显示版本号，main 显示版本号和短提交号。

YAML 备份当前使用 `schema_version: 1`。导入仅接受程序支持的 schema 版本；新版若提高 schema 版本，会先提供升级路径，不承诺新版 YAML 能由旧程序导入。数据库在启动时执行向前迁移；不承诺降级读取已迁移数据库，升级前须保留完整文件级快照。完整回退流程见下文“数据库迁移可能不可逆”。

`--output json|yaml|table` 适用于 REST JSON 响应，默认 JSON；`export` 始终原样输出 YAML，适合重定向到文件。表格会展平首层字段，嵌套字段以紧凑 JSON 显示。输出中的 Key、token、密码等敏感字段会隐藏。可用 `rollbook completion bash|zsh|fish` 生成相应的补全脚本，例如 `rollbook completion bash > ~/.local/share/bash-completion/completions/rollbook`。

常用命令：

```sh
rollbook search Overleaf
rollbook get service:overleaf
rollbook --output table site location:home
rollbook create '{"id":"host:demo","type":"host","name":"演示主机","fields":{}}' '新增资产'
rollbook update host:demo --set name=新名称 --set fields.kind=物理机 --reason '更正'
rollbook propose host:demo --set fields.kind=虚拟机 --reason '待人工审批'
rollbook propose create @new-service.json '建议新增'
rollbook note host:demo --body '已巡检' --kind 运维笔记 --reason '巡检'
rollbook verify host:demo --reason '现场核实'
rollbook changes --pending
rollbook topology service:overleaf --depth 2
rollbook topology service:overleaf --view access
rollbook topology host:server --depth 2
rollbook network network:example
rollbook versions --outdated
rollbook versions --check
rollbook context get
rollbook context put @context.json
```

`update`、`propose`、`note`、`verify` 省略 `--rev` 时会先读取对象当前 rev；若其他人并发修改导致 409，请重新 `get` 后重试。修改字段用 `--set field=value`，普通字段如 `name` 直接写，业务字段可写 `fields.kind`；含空格的值请按 shell 规则加引号。传入有效 JSON 数字、布尔值或对象时按 JSON 解析，其余按字符串。若要提议删除，使用 `propose <id> --action delete --reason 文本`。上下文新增文件是含 `id` 的对象 JSON；更新时包含 `id`、`rev`、`body_md` 和 `reason`。401、403、409、422 会给出中文原因和下一步建议。

拓扑也可通过 `GET /api/services/{id}/topology` 和 `GET /api/hosts/{id}/topology` 查询。`view=access` 返回入口经反代、服务和宿主到父主机的访问链；默认返回拓扑。响应保留原有 `nodes`（ID 数组）与 `edges`，另有 `details`（名称、类型、状态）供绘图使用。MCP `topology` 工具接受同样的 `id`、`depth` 和 `view` 参数。

## 从空库到第一次查询

1. 按「本地运行」或 Compose 部署章节创建首用户并启动服务，然后登录网页。空库会提示建立首个站点，或导入使用 `192.0.2.0/24`、`.invalid` 域名的**虚构示例**。也可以到「资产」手工新建站点、网段、主机等。
2. 在「Agent 接入」创建只读 Key。Token 只显示一次；页面提供复制按钮、CLI 环境变量，以及 Claude Code、Codex、Cursor 的 Streamable HTTP 配置片段。将对应片段放入客户端配置，替换其中的地址为实际可访问的 HTTPS 地址。配置包含 Key，不要提交到版本库。
3. 将 [`skills/rollbook/SKILL.md`](skills/rollbook/SKILL.md) 复制到客户端的 `skills/rollbook/SKILL.md`（Codex 可放在 `~/.agents/skills/rollbook/SKILL.md`；Claude Code 放在 `~/.claude/skills/rollbook/SKILL.md`；Cursor 放在项目 `.cursor/skills/rollbook/SKILL.md`），重启客户端后让 agent “搜索示例站点并读取详情”。也可用 CLI 验证：

```sh
export ROLLBOOK_URL='https://rollbook.example.invalid'
export ROLLBOOK_KEY='<网页显示的一次性 token>'
rollbook search 示例
```

若要让 agent 提议新增或修改，另建 `propose` 级 Key，按 Skill 的查询、读取 rev、提交提议流程操作，最后在网页「待处理」审批。`write` 级 Key 会直接生效，只授予可信 agent。

## 上游版本检查

在服务的 `upstream_json` 中配置一个上游，并为服务建立版本记录（首次检查也会自动创建空白版本记录）：

```json
{"provider":"github_releases","repository":"owner/repo"}
{"provider":"docker_hub","image":"namespace/image"}
{"provider":"endoflife_date","product":"go"}
```

只使用固定公开 API 域名，标识符不接受 URL。Docker Hub 从最多 100 个标签中选择匹配且可比较的最高版本，忽略 `latest` 等别名；GitHub 默认取最新正式 Release；endoflife.date 使用 v1 API 的最新 release。`tag_prefix` 可限制前缀，`tag_regex` 可限制正则；Docker Hub 的正则若含第一个捕获组，会把该组当作数字版本比较，同时保留完整原始 tag。例如 `{"provider":"docker_hub","image":"namespace/image","tag_regex":"^([0-9]+\\.[0-9]+)-alpine$"}`。非法或过宽正则在保存服务时拒绝。网页显示上游来源和原始 tag。

版本状态分为“已锁定”“90 天内停止支持”“有新版本”“上游数据可疑，需人工确认”“无法判断”“已是最新”。锁定版本不会出现在“有新版本”列表或提醒中；当前版本高于上游值时须人工确认。手动重查仍是 `POST /api/versions/check`，MCP 为 `check_versions`；自动检查在启动及每小时扫描，成功记录每 24 小时重查。上游错误不覆盖上次成功值，也不改当前版本、固定理由或清册 rev。GHCR 暂不支持。

网页登录后可在版本页填写机器人 Webhook URL、选择飞书或企业微信格式并启用提醒；默认关闭，未启用时不会发送外部请求。发现新版本或 90 天内停止支持时每天最多发送一次摘要；失败记录在数据库中，最多重试 3 次，每次至少间隔 1 小时。Webhook 请求 5 秒超时、拒绝重定向、响应最多读取 4 KiB，失败记录不含 URL 或响应正文。每天生成一条全局共享 context（key 为 `version-alert:YYYY-MM-DD`），agent 可以通过 `get_context` 读取。YAML 全量和清册备份包含共享 context 与版本数据；Webhook 地址及投递记录仅保存在本机数据库，不进入 YAML 备份。恢复 YAML 时保留本机提醒配置与投递记录；文件级数据库快照包含两者。

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

推荐用可选的 Docker secret 自动创建首位管理员。先将用户名写入 `.env`（例如 `ROLLBOOK_BOOTSTRAP_USER=owner`），再创建仅部署管理员可读的口令文件，首次启动时带上覆盖文件：

```sh
umask 077
read -rsp '首次管理员口令（至少8字符，且四类字符至少三类）: ' bootstrap_password; printf '\n'
printf '%s' "$bootstrap_password" > secrets/bootstrap_password
unset bootstrap_password
docker compose -f compose.yaml -f compose.bootstrap.yaml up -d
```

`compose.bootstrap.yaml` 仅在显式指定时加载；默认 `docker compose config` 和 `docker compose up -d` 都不需要 `secrets/bootstrap_password`。口令文件内容可带一个结尾换行，程序会去掉。也可在 `compose.yaml` 的 `rollbook.environment` 中设置 `ROLLBOOK_BOOTSTRAP_USER: ${ROLLBOOK_BOOTSTRAP_USER}` 和 `ROLLBOOK_BOOTSTRAP_PASSWORD: ${ROLLBOOK_BOOTSTRAP_PASSWORD}`，在 `.env` 中提供对应值；这种方式会使口令出现在 Compose 配置和容器环境中，建议只用于本地测试。两种口令来源不可同时设置，口令至少 8 字符，且大写、小写、数字、符号四类至少三类。

首次登录后移除引导环境变量、覆盖文件配置及口令文件，再按默认命令重建服务；已有用户时自动引导只记录一次忽略提示，不会更改原密码。也可继续使用交互式命令创建首位用户，按提示输入至少 8 字符，且大写、小写、数字、符号四类至少三类的密码：

```sh
docker compose run --rm -it rollbook bootstrap-user owner
# 已运行的服务也可使用：
docker compose exec rollbook rollbook bootstrap-user owner
```

两条命令二选一。再次执行会报告用户已存在，不会覆盖密码。容器监听 8080，Compose 默认只把宿主机 `127.0.0.1:8080` 暴露给本机反向代理。反代终结 TLS 时，须将其连接到清册的 TCP 地址段填入 `ROLLBOOK_TRUSTED_PROXY_CIDRS`，例如仅同机代理可用 `127.0.0.1/32`。Compose 部署需将该变量显式加入 `compose.yaml` 的 `rollbook.environment`，例如 `ROLLBOOK_TRUSTED_PROXY_CIDRS: ${ROLLBOOK_TRUSTED_PROXY_CIDRS:-}`，并在 `.env` 设置实际代理来源地址段；Zoraxy 应开启并覆盖上游 `X-Forwarded-Proto`，Nginx 可用 `proxy_set_header X-Forwarded-Proto $scheme;`。反代须先清除客户端传入的同名头和 `Forwarded` 头，或将它们规范化为单一值；不要信任整个内网或公网地址段，也不要把清册后端端口直接暴露到公网。只有 TCP 对端在可信地址段内，清册才采信协议头；直连 TLS 始终按 HTTPS 处理。页面风险提示和会话、CSRF Cookie 的 `Secure` 标记均按每次请求实际协议决定。

若必须在可信内网直接用 HTTP 访问，把 `.env` 中 `ROLLBOOK_BIND_ADDR` 设为内网接口地址，并把 `ROLLBOOK_ALLOW_INSECURE_HTTP` 设为 `true` 后运行 `docker compose up -d`。HTTP 请求会显示风险提示，Cookie 不带 `Secure`；未开启该选项时 HTTP 登录会被拒绝。启动日志会提示此选项允许 HTTP 登录。**HTTP 下口令和 key 会在网络中明文传输，只能在可信内网使用。**

数据库、WAL 与默认备份目录均位于 `/data`，由 Compose 的 `rollbook-data` 卷持久化。`docker compose down && docker compose up -d` 不会删除卷；不要使用 `down -v`。启动后及每 24 小时写一次完整 YAML 日备份，首次启动及每 7 天另写周备份。需要把 YAML 备份放在独立卷时，在 `.env` 设 `ROLLBOOK_BACKUP_DIR=/backups`；Compose 已为 `/backups` 提供持久卷。自行替换成宿主机绑定目录时，数据库目录与独立备份目录都要持久化，且应由容器内固定的 `10001:10001` 用户读写，例如：

```sh
sudo mkdir -p /srv/rollbook/data /srv/rollbook/backups
sudo chown -R 10001:10001 /srv/rollbook/data /srv/rollbook/backups
```

然后在 `compose.yaml` 将相应卷映射改为 `/srv/rollbook/data:/data` 和 `/srv/rollbook/backups:/backups`。密钥文件保持仅部署管理员可读；初始化后的密钥卷在服务中只读挂载。备份卷应定期复制到其他安全位置。

### 升级和回退

升级前，先备份 `.env`、密钥文件、数据库整个 `/data` 卷，以及启用独立备份目录时的 `/backups` 卷。完整 YAML 含密码与 key 的哈希，也要按敏感数据保存。SQLite 使用 WAL；做文件级快照时先 `docker compose stop rollbook`，把数据库文件及 `-wal`、`-shm` 一同复制，再启动。可用宿主机卷快照工具，或停机后用以下命令复制容器数据：

```sh
docker compose stop rollbook
mkdir -p snapshot-data snapshot-backups
docker compose cp rollbook:/data/. ./snapshot-data/
docker compose cp rollbook:/backups/. ./snapshot-backups/
docker compose start rollbook
```

确认备份可读后，编辑 `.env` 的 `ROLLBOOK_VERSION` 为新版本（如 `v0.6.0`），执行 `docker compose pull && docker compose up -d`，再检查 `docker compose ps`、`/healthz` 和登录。启动会自动运行尚未应用的数据库迁移；每个版本只执行一次，已记录的迁移不会在重启时重复执行。不要在同一数据库上同时运行新旧版本。

**数据库迁移可能不可逆。**如果新版本无法使用，先停服务，把 `/data` 恢复成升级前的完整文件级快照，再把 `.env` 的版本改回旧版并 `docker compose up -d`；不能只改镜像 tag 后让旧程序直接读取已迁移的数据库。若只用 YAML 备份恢复，还须确认旧版本支持其 `schema_version`，且应保留原有用户凭据与密钥。完整卷快照是回退的首选。

GitHub Actions 在 PR 上运行测试、vet 和镜像构建；main 推送发布 `main` 与 `sha-<短提交号>`，语义版本 tag 推送发布 `vX.Y.Z`、`X.Y`、`latest`，均含 amd64 和 arm64。发布工作流不部署服务。

## 备份与恢复

服务启动时及每 24 小时在 `ROLLBOOK_BACKUP_DIR` 指定目录原子写入带唯一时间戳的日备份（0600）；未设置时使用数据库目录。首次启动及每 7 天另写周备份。默认保留最近 7 份日备份、4 份周备份，仅轮转 `rollbook-daily-*` 和 `rollbook-weekly-*` 受管文件。`ROLLBOOK_BACKUP_INTERVAL` 与 `ROLLBOOK_BACKUP_WEEKLY_INTERVAL` 使用 Go duration 格式，例如 `1h`、`168h`；`ROLLBOOK_BACKUP_DAILY_KEEP` 与 `ROLLBOOK_BACKUP_WEEKLY_KEEP` 是正整数。周期或保留数无效时使用默认值。网页的「备份与恢复」可列出受管文件的时间和大小、下载或选择一份提交恢复，也可上传 YAML。恢复预览列出每类对象的新增、修改、删除及将丢失的变更；审批时若数据已变化会返回冲突，需重新提交。恢复前必须成功保存当前库快照；批准后会清除所有会话，需重新登录。完整备份含密码与 key 的哈希，请按敏感文件管理。

CLI 使用 `ROLLBOOK_KEY`：`rollbook export > catalog.yaml` 下载清册；`rollbook import catalog.yaml` 校验并提交待确认导入，网页用户批准后才会恢复。只读 key 可以导出，提议/直接写 key 可以提交导入，任何 key 都无法下载完整备份或批准恢复。未知 `schema_version` 和校验不通过的文件会被拒绝，旧数据库保持不变。

CSV 合并导入支持 `host`、`network`、`ip`、`service` 四类对象。网页「备份与恢复」可下载各类模板并提交 CSV；CLI 用 `rollbook template csv host > hosts.csv` 下载模板，用 `rollbook import csv host hosts.csv` 提交，其他类型替换 `host` 即可。CLI 使用所选 profile 的服务地址与 Key。文件提交后进入与 YAML 合并导入相同的预览和审批队列；即使使用提议级 Key，也须由网页用户批准后才能写入。导入文件上限 16 MiB；待处理数量与 7 天过期规则同 YAML。

CSV 使用 UTF-8 编码（可带 BOM），首行是模板列名，列名区分大小写；未知、重复、缺少必填列会返回行列错误。每行 `id` 必须带类型前缀（如 `host:`），文件内重复 ID 会被拒绝。新增对象不填 `rev`；更新现有对象必须填写当前正整数 `rev`，不一致会在预览中标为冲突。空单元格表示不提供该字段，不会把现有值清空；必填列不能留空。JSON 列填写合法 JSON；URL 列只接受绝对 HTTP/HTTPS 地址。关联列填对象 ID，可引用库中已有对象或同批文件中的对象；不存在的引用会显示定位错误。CSV 会映射到原合并模型，原 YAML 导入格式与行为不变。

## 前端开发

网页源码位于 `web/src/`，使用 React、TypeScript 和 Vite。首次开发运行 `npm ci --prefix web`，修改后运行 `npm run typecheck --prefix web` 与 `npm run build --prefix web`。构建产物 `web/dist/` 随仓库提交，并由 Go 内嵌，因此直接执行 `go build ./cmd/rollbook` 无需 Node。提交前确认重新构建后的 `web/dist/` 与源码一致。
