# Rollbook 第一版技术方案

## 范围与部署

单容器、Go 单二进制，静态网页通过 `embed` 打包；SQLite 使用 WAL、外键和 FTS5，优先采用无 CGO 的 `modernc.org/sqlite`。仅监听可配置的内网地址，经 Zoraxy 反代；应用校验可信反代头，默认不信任任意 `X-Forwarded-*`。GitHub Actions 运行测试并构建、推送 GHCR 镜像，不部署。自动发现、监控、远程执行、GHCR 版本源均不在第一版。

建议目录：`cmd/rollbook`（`serve/search/get/update/export/import`）、`internal/config`、`internal/db/migrations`、`internal/catalog`（实体、关系、校验）、`internal/search`、`internal/auth`、`internal/audit`、`internal/httpapi`、`internal/mcp`、`internal/versioncheck`、`internal/backup`、`web/`（构建产物由 Go embed）、`testdata/`、`docs/`。HTTP、MCP、CLI 共用应用服务与权限检查，不各自改数据库。

## 数据库与一致性

所有业务对象统一存 `objects(id TEXT PRIMARY KEY, type TEXT, name TEXT, aliases_json TEXT, tags_json TEXT, status TEXT, source TEXT, confidence TEXT, verified_at, verified_by, rev INTEGER, created_at, created_by, updated_at, updated_by)`，再以 `id` 为主外键存以下类型表。时间统一 UTC RFC3339，JSON 字段校验为数组/对象，枚举按 issue 固定。`rev` 从 1 开始；修改时事务内执行 `UPDATE ... WHERE id=? AND rev=?`，影响 0 行返回 409，成功后加 1 并同事务写变更。创建需要唯一 ID，不接受客户端伪造审计字段。删除仅由网页用户批准提议后执行；外键采用 RESTRICT，先显式处理关联对象，避免级联误删。

| 表 | 主要字段及约束 |
| --- | --- |
| `locations` | `id, kind, parent_id, city, region, address, timezone, description`；父级不得成环 |
| `uplinks` | `id, location_id, isp, type, bandwidth, public_ipv4, public_ipv4_addr, public_ipv6, ddns, inbound_ports_json, edge_device_id, account_ref` |
| `hosts` | `id, kind, net_role, uplink_id, parent_host_id, location_id, model, os, os_version, resources_json, mgmt_url`；宿主不得成环 |
| `networks` | `id, cidr, location_id, router_id, uplink_id, vlan_id, gateway, dhcp_range, purpose`；CIDR 规范化、唯一 |
| `ips` | `id, address, network_id, host_id, interface, mac, assign`；地址唯一，`network_id` 由同站点最长前缀匹配自动推导，匹配不唯一则拒绝 |
| `services` | `id, host_id, kind, deploy_method, upstream_json, owner, config_path, docs_url` |
| `ports` | `id, service_id, bind, port, protocol, purpose`；端口 1–65535 |
| `endpoints` | `id, service_id, kind, url, via_json, auth, exposure, nav_card_json`；`via` 为有序对象 ID 数组 |
| `dependencies` | `id, from_id, to_id, kind, note, generated`；唯一 `(from_id,to_id,kind)`；宿主与入口反代链生成边，重建时仅替换 generated 边 |
| `versions` | `id, service_id UNIQUE, current_version, current_source, latest_version, latest_source, latest_url, eol_date, pinned, pinned_reason, checked_at` |
| `credentials` | `id, target_id, vault, item_ref, purpose`；只存引用，不存值 |
| `notes` | `id, target_id, kind, body_md, author`；只允许 INSERT |
| `contexts` | `id, scope, project, key, body_md, owner_key_id, related_ids_json, expires_at, archived_at`；过期归档，不删除 |
| `changes` | `id, object_type, object_id, action, diff_json, actor_type, actor_key_id, actor_key_name, actor_issuer_id, actor_issuer_name, via, reason, state, base_rev, reviewed_by, reviewed_at, created_at`；保留原始提议者快照 |
| `users` | `id, login UNIQUE, password_hash, enabled, created_at`；初始仅一人，结构支持多人 |
| `api_keys` | `id, name, token_hash UNIQUE, level, issuer_user_id, created_at, expires_at, note, last_used_at, revoked_at`；不存明文 |
| `sessions` | `id_hash, user_id, expires_at, csrf_hash`；网页专用 |

在迁移中建立外键、常用关系索引、`objects(type,status,verified_at)` 和 `changes(object_id,created_at)`。FTS5 索引 `name/aliases/tags/description/URL/域名`，另建 IP、端口精确索引；对象与关系变更同事务刷新搜索文档。检索结果由服务层补齐站点、网段路由器、宿主服务和入口。FTS 无法处理的 IP 片段（如 `126`）走规范化地址索引与受控前缀/片段查询；测试数据规模下目标 p95 <1 秒。关系查询和写入都验证引用存在，`get_site` 聚合位置树、网段、接入、路由器。待核实为 `verified_at IS NULL OR verified_at < now-30d`。

## 身份、权限与审计

网页用用户登录、HttpOnly/Secure/SameSite Cookie 与 CSRF token；首次启动通过本机一次性初始化流程设置管理员密码，不接受网络默认密码。REST、MCP 和 CLI 使用 `Authorization: Bearer <key>`。key 用系统安全随机数生成，生成时仅显示一次，数据库只保存带服务端 pepper 的 HMAC-SHA256 摘要；固定时间比较，按摘要索引查询，每次请求重新读取级别、到期和吊销状态，吊销立即生效。pepper 是运行环境配置，不能提交；备份恢复到新环境时需保留该配置，否则旧 key 失效。日志不记录 Bearer 值。

| 操作 | 只读 | 提议 | 直接写 | 网页用户 |
| --- | --- | --- | --- | --- |
| 查询、导出清册 | 是 | 是 | 是 | 是 |
| 写/续期自己创建的 context | 是 | 是 | 是 | 是 |
| 追加 note、提交变更提议 | 否 | 是 | 是 | 是 |
| update、mark_verified、check_versions | 否 | 否 | 是 | 是 |
| 批准/驳回提议、删除、管理 key、执行整库恢复 | 否 | 否 | 否 | 是 |

所有写操作含 `rev`（新增对象用 `base_rev: 0`），包括笔记和上下文写入时对目标/原记录的版本前提。拒绝缺少 reason 的变更，审计记录保留 key 名称和下发人快照；批准提议时保存批准者与时间，应用时重查 `base_rev`，冲突则 409，不覆盖新值。秘密拦截在所有自由文本、JSON、Markdown、导入数据进入持久层之前统一执行：至少识别私钥 PEM 头、`password=`、常见 token/key 赋值和高置信度令牌；返回 422，不把命中值写入日志或错误正文。仅引用字段允许 `vault/item_ref`，不允许凭据值。对误报可改写内容为引用，不能绕过校验。

CLI `import` 使用 Bearer key 提交整库导入提议并校验文件，真正的覆盖恢复仍由网页用户批准；这样不会让 key 获得删除或批准权。CLI `export` 仅导出清册数据；含账号与 key 哈希的完整备份只供网页登录或本机定时任务。此处的导入语义及备份范围如需改动权限模型，必须先由产品负责人确认。

## REST 与 MCP

统一 JSON 错误 `{code,message,details}`，401 无效/吊销/到期，403 权限不足，409 rev 冲突，422 内容校验。读请求可以用 key 或网页会话；管理类接口只接受网页会话，即使同时带 Bearer 也返回 403。分页用 `limit/cursor`，默认 50、上限 200。写请求含 `rev`、`reason`，PATCH 使用结构化字段，禁止任意 SQL/JSON Patch 路径。MCP `/mcp` 为 Streamable HTTP，工具调用与 REST 调用同一服务方法和授权表；错误映射为工具错误，不暴露内部堆栈。会话状态仅在必要时使用，跨进程不依赖 MCP 会话保存授权。

| MCP 工具 | REST | 说明 |
| --- | --- | --- |
| `search` | `GET /api/search?q=` | IP、域名、端口、名称、别名 |
| `get_asset` | `GET /api/assets/{id}` | 通用对象及关系 |
| `get_service` | `GET /api/services/{id-or-name}` | 宿主 IP、端口、入口、版本 |
| `list_by_host` | `GET /api/hosts/{id}/inventory` | 主机及其服务 |
| `list_by_network` | `GET /api/networks/{id}/inventory` | 网段 IP、设备、路由器 |
| `get_site` | `GET /api/sites/{id-or-name}` | 站点树、接入、网段 |
| `topology` | `GET /api/services/{id}/topology?depth=2` | 服务中心最多两层 |
| `list_outdated` | `GET /api/versions/outdated` | 版本落后列表 |
| `get_changes` | `GET /api/changes` | 变更历史，支持过滤 |
| `get_context` | `GET /api/contexts` | 默认不含过期，`archived=true` 查看归档 |
| `put_context` | `PUT /api/contexts/{id}` | 自己创建项；scope 默认到期策略 |
| `record_note` | `POST /api/notes` | 只追加 |
| `propose_change` | `POST /api/changes/proposals` | 待确认，含目标、diff、base_rev、reason |
| `update` | `PATCH /api/assets/{id}` | 直接写现有对象 |
| `mark_verified` | `POST /api/assets/{id}/verify` | 更新时间和操作者 |
| `check_versions` | `POST /api/versions/check` | 触发检查，不改 current 字段 |

网页专用：`POST /api/auth/login`、`POST /api/auth/logout`、`GET/POST/PATCH/DELETE /api/keys`、`POST /api/changes/{id}/approve`、`POST /api/changes/{id}/reject`、`DELETE /api/assets/{id}`（仅通过已批准删除提议）、`GET /api/backup/export`、`POST /api/backup/restore`。清册新增可由网页直接创建，或由 key 通过提议创建；直接写 key 若需新增，也必须携带 `base_rev:0` 并走同一审计路径。MCP 工具名保持 issue 清单，不另加管理工具。CLI `search/get/update/export/import` 分别调用对应 REST；import 为待确认提议。

## 页面

全局搜索、按主机/站点/网段浏览、服务详情、以服务为中心的两层拓扑、有新版本/待确认/待核实列表、key 管理与导入导出。待确认逐条展示字段差异、原值、新值、提议者、依据和当前 rev；批准前再次校验冲突。所有网页操作走同一 REST 应用服务。仅渲染 Markdown 安全子集，URL 协议仅允许 http/https 等明确白名单，避免脚本注入。

## 版本检查

每个 service 的 `upstream_json` 明确 `provider`（`github_releases`、`docker_hub`、`endoflife_date`）与仓库/镜像/产品标识。每日定时一次，启动时对过期或未检查项补查；单实例 SQLite 租约避免重复任务。并发、超时、重试和速率限制有界；上游失败保留上次最新值，记录错误供页面显示。只更新 `latest_version/latest_source/latest_url/eol_date/checked_at`，不改 current、pinned 或其他清册字段。版本比较保留原始字符串，并针对各源按发布日期/规范版本排序；无法可靠比较时显示“需人工确认”，不误报落后。配置上游的服务在成功响应后 24 小时内有 `checked_at`，失败明确标示并可手动重试。GHCR 后续再做。

## YAML 备份与恢复

格式固定 `schema_version: 1`、`exported_at`、`objects`（按 type/id 排序）、`relations`（依赖及有序入口链）、`notes`、`contexts`、`changes`；完整备份增加 `users`、`api_keys` 的哈希和元数据，**从不含明文 key、密码或 session**。凭据仅导出 `vault/item_ref`。文件采用 UTF-8 YAML，显式 null，稳定 ID，保留 rev、审计时间；示意：

```yaml
schema_version: 1
exported_at: "2026-11-11T00:00:00Z"
objects:
  - id: "host:multica-server"
    type: host
    name: "Multica 服务端"
    rev: 1
    host: {location_id: "location:beijing", kind: "物理机"}
relations: []
notes: []
contexts: []
changes: []
```

定时备份以临时文件、fsync、原子 rename 写入数据目录，权限 0600，保留份数可配置。恢复先解析 schema_version、秘密检测、引用与 CIDR 校验、计算对象/关系计数并展示预览；网页用户批准后在独占写事务内导入到临时数据库，完整校验通过再原子替换并重建 FTS、清除 session。任何失败保留旧库及备份。恢复后核对对象和关系计数、外键、搜索结果。格式演进只添加明确的版本迁移，不静默丢字段。

## 测试与交付

单元测试覆盖 CIDR 最长前缀、关系环、版本比较、秘密检测；SQLite 集成测试覆盖迁移、FTS 与 IP 搜索、rev 冲突、审计事务、context 过期归档、YAML 往返与失败回滚。HTTP/MCP/CLI 端到端测试使用 issue 示例数据逐条验证 16 条验收标准，尤其是 key 级别、即时吊销、管理员接口 403、`get_service("Overleaf")` 和搜索 `126`。CI 执行 `go test ./...`、前端构建与镜像构建/推送 GHCR；不含部署。每个实施 PR 交测试审查独立验收，CI 通过后由测试审查 squash 合并 main；失败退回工程师。任何人不得直接推 main。
