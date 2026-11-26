# Rollbook 清册

## v0.14.0 飞书邮箱与授权绑定

`v0.14.0` 在「个人设置」（`/settings/personal`）新增邮箱与飞书绑定入口、「设置 → 认证设置 → 账号与绑定」（`/settings/auth?section=accounts`）邮箱编辑，以及按飞书邮箱自动绑定登录。邮箱可空；非空时去首尾空格并按 ASCII 大小写折叠，全局唯一。飞书登录优先沿用已允许的稳定主体绑定；若返回的邮箱唯一对应另一用户则拒绝。尚无主体绑定时，仅当飞书返回邮箱且唯一匹配启用的现有清册用户，才原子绑定并登录；缺失或不匹配时，请先本地登录并在个人设置绑定。禁用绑定、停用用户不能绕过，不按姓名匹配，不自动开户。本地登录保留。飞书应用须启用 `auth:user.id:read` 与 `contact:user.email:readonly` 并发布对应权限；授权请求包含两者。邮箱权限未开通或上游未返回 `data.email` 时，不进行邮箱匹配；无唯一匹配不发 session，可先本地登录后绑定，权限缺失不阻塞正式发版。绑定入口沿用同一应用和回调，公开 HTTPS 地址须精确包含非标准端口，例如 `:5443`。详见[外部身份登录](docs/external-auth.md)。

此变更含 `018_feishu_email.sql` 前向迁移：新增 `users.email` 及非空唯一索引、`auth_pending.purpose`、`user_id`、`session_hash`；认证后端与绑定流程有变更，未新增 pepper 或密钥方案。[v0.14.0 正式 Release](https://github.com/CosmoGao/rollbook/releases/tag/v0.14.0)、镜像与 CLI 已核验；正式镜像针为 `ghcr.io/cosmogao/rollbook:v0.14.0@sha256:5b116dabebd401bcda38197db329680f7ca05fc2c277fd9f4ecb6998f39a01bc`。发版本身不部署，也不改变现网登录策略。升级前先停服，保存完整 `/data` 文件级快照（SQLite、`-wal`、`-shm`）及独立 `/backups` 快照，保留当时的环境配置、pepper 或其 `_FILE`、身份源与认证加密材料，并在隔离环境验证恢复。完整 YAML 不能代替快照，也不含页面凭据。回退时先停服，恢复升级前完整快照及当时密钥配置，再启动上一正式镜像；不能只换旧镜像读取迁移后的数据库。


Rollbook 是面向家庭与小企业的自建服务和资产清册。请只在可信内网运行。英文说明见 [README.en.md](README.en.md)，版本变化见 [CHANGELOG.md](CHANGELOG.md)，发版检查见 [docs/releasing.md](docs/releasing.md)。

## v0.13.0 飞书设置校验与公开回调修复

本版已正式发布：[v0.13.0 Release](https://github.com/CosmoGao/rollbook/releases/tag/v0.13.0)。tag 解引用提交为 `8b478b4c8ab4199baf68ffc33411e3cdac23c92f`；正式镜像 `ghcr.io/cosmogao/rollbook:v0.13.0` 及六平台 CLI、`install.sh` 已核验，详情见[本版说明](docs/release-notes-v0.13.0.md)。此处发版后文档补正只进入后续 `main`，不在不可变 tag 内。正式发布不会自动部署现有实例；升级仍须按下文备份和验证恢复。

系统「设置 → 认证设置 → 飞书」（`/settings/auth?section=feishu`）显示的非 443 HTTPS 公开回调现在可以保存。飞书 App ID 缺失、凭据动作无效或缺少新凭据、回调地址与公开入口不符时，服务端返回中英文字段级 `422 validation`（`client_id`、`secret_action`、`redirect_url`）。公开 origin 取自直连 Host/TLS，或仅采信可信直接代理的单值转发头；伪造、冲突、多值转发头会被拒绝。未启用飞书时登录页不显示入口。代理配置和错误处理见[公开地址与校验](docs/public-auth-origin.md)、[外部身份登录说明](docs/external-auth.md)。LDAP/OIDC 的 HTTPS、固定回调、显式绑定与安全失败约束保持不变。

搜索快捷键、主题、认证凭据只写不读、防锁死恢复路径、现有 Key 权限和已登录网页用户权限保持不变。真实 LDAP、OIDC、飞书 IdP 尚未联调；没有新增全站 RBAC 或群组映射。本地登录开关和生产身份源配置保持不变，不要为本次升级启用外部身份源。

相对 `v0.12.0` 无新增数据库迁移、无密钥方案变更；HTTP API 公开 origin 与设置校验确有变化。升级前停服并保存完整 `/data` 文件级快照（含 SQLite 数据库、`-wal`、`-shm`）及独立使用的 `/backups` 卷；保留原环境变量、`ROLLBOOK_AUTH_PEPPER` 或其 `_FILE` 文件、身份源环境变量或 `_FILE` 文件及认证配置加密所需密钥材料，在隔离环境验证恢复。完整 YAML 不能代替快照或恢复页面凭据。回退先停服，恢复升级前完整快照、原配置和密钥，再运行 `v0.12.0` 正式镜像；不能只换旧镜像读取升级后的数据。原 pepper 丢失或更换时，先在本机运行 `rollbook auth local-login enable` 保留本地恢复入口，再重新填写页面凭据。详见[升级和回退](#升级和回退)中的停服与快照示例；该节的 `v0.12.0` 升级步骤是上一版历史记录。

## v0.12.0 全站文本控件焦点修复

本版已正式发布：[v0.12.0 Release](https://github.com/CosmoGao/rollbook/releases/tag/v0.12.0)。tag 指向 `47f96d343eb91c06c6ceb23122de7a1bd058a3af`；正式镜像 `ghcr.io/cosmogao/rollbook:v0.12.0` 和六平台 CLI、`install.sh` 已核验。此处发版后文档补正只进入后续 `main`，不在已发布的 tag 内。正式发布不等于生产部署，现有实例不会自动更新；升级前仍须按下文核对产物并备份。

全站独立文本 `input`、`textarea`、`select` 在自身 `border-box` 内显示与外框重合的焦点色和可辨识强度；「待处理 → 筛选记录」（`/pending`）等实际路由与弹窗已覆盖。搜索页 `form.search-box` 及 HEADER 的桌面、窄屏搜索与主题组合控件由外层绘制唯一外框。错误边框与 forced-colors 焦点反馈保留；原生 select 的系统展开层不受页面 CSS 控制。路由矩阵、脱敏截图和复现命令见[文本控件验证记录](docs/control-geometry.md)。搜索快捷键、主题选择、认证凭据只写不读及原有登录策略保持不变；真实 LDAP、OIDC、飞书 IdP 尚未联调，没有新增全站 RBAC 或群组映射。

相对 `v0.11.0` 无新增数据库迁移、无密钥方案变更。升级前停服，保存完整 `/data` 文件级快照（SQLite 数据库、`-wal`、`-shm`）及独立使用的 `/backups` 卷；保留原环境变量、`ROLLBOOK_AUTH_PEPPER` 或其 `_FILE` 文件、身份源环境变量或 `_FILE` 文件及认证配置加密所需密钥材料，并在隔离环境验证恢复。完整 YAML 不能代替快照，也不能恢复页面身份源凭据。回退先停服，恢复升级前完整快照、原配置与密钥，再启动 `v0.11.0` 正式镜像，不能只换旧镜像读取升级后的数据。原 pepper 丢失或更换时，先在本机运行 `rollbook auth local-login enable` 保留本地恢复入口，再重新填写页面凭据。具体步骤见[升级和回退](#升级和回退)。

## v0.11.0 HEADER 控件与设置导航

本版已正式发布：[v0.11.0 Release](https://github.com/CosmoGao/rollbook/releases/tag/v0.11.0)。发布镜像和 CLI 不会自动更新现有实例。

HEADER 桌面 `form.top-search`、窄屏 `form.mobile-search` 和 `header.topbar label.theme-control` 由组合控件绘制单层边框与贴合边界的焦点反馈，内部输入、按钮和 select 不再出现完整重复边框或偏移焦点环；Ctrl/⌘+K、搜索提交和主题选择与浏览器持久化保持原有行为。准确控件与截图见 [HEADER 控件验证记录](docs/HEADER-CONTROLS-VERIFICATION.md)。

系统「设置 → 认证设置」（`/settings/auth`，默认本地登录）现分为本地登录、LDAP、OIDC、飞书、账号与绑定、认证审计六组；`/settings/auth?section=ldap|oidc|feishu|accounts|audit` 可直达对应分组并支持刷新与历史导航。「备份与恢复」（`/settings/backup`）和「个人设置」（`/settings/personal`）保持独立。切换分组或离开页面有未保存草稿保护，按身份源独立保存反馈。页面凭据只写不读；任一同源环境变量或 `_FILE` 存在时，启动环境接管整个身份源并使页面配置只读，移除后须重启才能切源。连通测试不等于真实外部身份试登录；关闭本地登录前须按[外部身份登录说明](docs/external-auth.md)核对真实试登录与本机恢复通道。真实 LDAP、OIDC、飞书 IdP 尚未联调；所有已登录网页用户仍有完整管理权，没有新增全站 RBAC 或群组映射。

相对 `v0.10.0` 无新增数据库迁移、无密钥方案变更。升级前须停服，保存完整 `/data` 文件级快照（SQLite 数据库、`-wal`、`-shm`），独立使用 `/backups` 时也保存该卷；保留原环境变量、`ROLLBOOK_AUTH_PEPPER` 或其 `_FILE` 文件、身份源环境变量或 `_FILE` 文件及认证配置加密所需密钥材料，并在隔离环境验证恢复。完整 YAML 不能代替快照，也不含页面身份源凭据。回退时先停服，恢复升级前完整快照、配置和密钥，再启动 `v0.10.0` 正式镜像；不能只换旧镜像读取升级后的数据。页面密文只能用原 pepper 解密；丢失或更换时须先在本机运行 `rollbook auth local-login enable` 保留恢复入口，再重新填写页面凭据。

## v0.10.0 认证设置与搜索框修复

本版已正式发布：[v0.10.0 Release](https://github.com/CosmoGao/rollbook/releases/tag/v0.10.0)。发布镜像与 CLI 不会自动更新现有实例。

系统「设置 → 认证设置」（`/settings/auth`）可配置并启停 LDAP/LDAPS、通用 OIDC、飞书身份源，管理用户、显式身份绑定、允许登录、本地入口与只读认证审计；「个人设置」（`/settings/personal`）处理当前账号，「备份与恢复」（`/settings/backup`）处理 YAML。所有已登录网页用户均有完整管理权，没有全站 RBAC 或群组映射；Bearer API Key 不能调用这些管理接口。页面凭据只写不读，以现有 pepper 派生密钥在服务端加密保存；完整与定时 YAML 不含页面凭据，也不能恢复它们。任一同源环境变量或 `_FILE` 存在时，启动环境接管整个身份源，页面只读并提示来源，已保存配置被遮蔽；移除后须重启才能切源。无环境接管时，页面保存与启停热生效。配置步骤和回调登记见[外部身份登录说明](docs/external-auth.md)。

从 `v0.9.0` 升级会执行 `017_auth_providers.sql`。升级前停服务，保存完整 `/data` 文件级快照（包括 SQLite 数据库、`-wal`、`-shm`）及独立使用的 `/backups`，保留当时的 `ROLLBOOK_AUTH_PEPPER` 或 `_FILE` 文件、身份源环境变量或 `_FILE` 文件和必要密钥材料。完整 YAML 不能代替快照；全量 YAML 导入会清除目标库页面身份源配置，且拒绝本地入口已关闭的文件。密文只能用原 pepper 解密；丢失或更换后须先用本机 `rollbook auth local-login enable` 保留恢复路径，再重新填写页面凭据。回退须恢复升级前完整快照和配置，然后启动 `v0.9.0` 正式镜像；旧镜像不能直接读迁移后数据库。真实 LDAP、OIDC、飞书 IdP 联调尚未进行。

网页首次进入空库会显示引导弹层；按 Escape 关闭后可继续使用当前导航。资产列表读取权限不足时显示「权限不足」及重试入口；离线读取会显示加载失败，清册不提供离线编辑。

全局搜索框在深浅主题的选中、悬停和聚焦状态沿真实框线高亮，不再出现偏离边框的外圈。`v0.10.0` 已有正式 tag、Release 与核验通过的镜像产物；实例升级仍须按下文备份和回退步骤办理。

## v0.9.0 身份登录与界面更新

可选启用 LDAP/LDAPS、通用 OIDC、飞书独立 OAuth 登录，三者默认关闭。LDAP 明文连接必须升级到 StartTLS；OIDC 与飞书回调使用固定 HTTPS 地址。本地登录默认保留；LDAP/OIDC 仍须显式绑定稳定主体；飞书可显式绑定，也可在邮箱唯一匹配已启用用户时自动绑定，不按姓名绑定。每个网页用户仍有清册全部管理能力，没有 RBAC 或群组映射。受控试登录后才可通过本机命令关闭本地入口；本机可运行 `rollbook auth local-login enable` 恢复入口，必要时运行 `rollbook reset-password <已有清册登录名>`。配置、完整命令与会话失效边界见[外部身份登录说明](docs/external-auth.md)。真实 LDAP、OIDC、飞书 IdP 联调尚未进行。

桌面与窄屏共用右上角头像账号入口；个人设置按会话能力显示本地改密或上游密码指引，并提供退出当前会话和退出所有设备；系统设置保留备份恢复。退出操作只撤销清册浏览器会话，不退出上游 SSO 或撤销 API Key。

全局搜索使用单层组合控件边框与统一焦点环，输入、清空与提交均可用键盘操作；搜索结果和接口行为不变。主题可在登录页与顶栏单击选择浅色、深色或跟随系统并即时生效。资产维护的站点位置使用卡片导航，正文打开详情、独立按钮展开下级；拓扑站点卡片接收卡片内滚轮，主内容使用贴近视口右缘的原生滚动条。

## 前端规范

现有业务页采用固定版前端与 VI 规范 `v1.0.0`：浅深主题统一语义颜色与控件轮廓，长内容按主内容区域滚动。登录页与顶栏均可显式选择浅色、深色或跟随系统；选择保存在当前浏览器中，刷新后继续生效。资产表单的 JSON 错误关联对应字段并保留已输入内容；待处理批量驳回弹层管理键盘焦点。拓扑名称、侧栏账号操作和窄屏导航采用更易读的字号与触控尺寸；首次引导关闭后焦点返回导航入口，资产读取权限不足时显示明确提示。业务、API 和授权行为保持原样。实现与截图见[前端实施记录](docs/frontend-implementation.md)；真人读屏和系统软键盘专项尚未验证。

## v0.7.0 设备与主机

「资产 → 设备与主机」现在可登记智能电视、游戏机、机顶盒与播放器、音箱与音频设备、摄像头与门铃、打印与扫描设备、智能家居中枢与网关、家电、传感器与控制器、其他设备；原有物理机、NAS、虚拟机、LXC、容器、网络设备保持原值。新建设备须主动选择类别、名称与位置。型号可填厂商和型号，别名与标签可分别添加多个值。电视、传感器等可以不登记 OS、IP、管理 URL 或服务；「未登记 IP」仅表示清册没有地址记录。

示例：在「客厅」登记「电视」为智能电视，型号填「Example 55」，不填 OS、IP、URL；登记「游戏机」为游戏机，型号填「Example Console 5」，别名填「PS5」；登记「门磁」为传感器与控制器，标签填「Zigbee」，不填 IP。若确知门磁依赖网关，可手工建立门磁→网关的依赖，不能把网关填为部署宿主。路由器 DNS、NAS 媒体服务、打印机打印服务均可在对应设备详情点击「加服务」；服务仍须指向一台真实设备或实例。纯外部/SaaS 服务当前不能作为无宿主 service 登记，也不要创建假主机。

全局搜索及设备列表可按名称、ID、别名、标签、型号、类别查找；列表可与站点、类别精确筛选和树视图组合。拓扑默认显示实体设备与无部署父的独立实例，按需展开部署实例、服务和入口。API 分层参数 physical/virtual/all 保留，分别表示默认层、实例层、全部层。

升级前请保留完整数据库卷快照。v0.7.0 会向前迁移 SQLite 的 hosts.kind 约束并重建搜索索引，旧记录不自动改类；YAML 仍为 schema_version: 1，CSV 旧列名和 REST/MCP/CLI 的 host 契约保持。含新类别的备份不能直接导入 v0.6.0；不能直接用旧镜像打开迁移后的数据库，回退须恢复升级前的完整快照。

## 本地运行

需要 Go 1.24 或更新版本。

```sh
go test ./...
export ROLLBOOK_AUTH_PEPPER="$(openssl rand -hex 32)"
go run ./cmd/rollbook bootstrap-user owner
go run ./cmd/rollbook serve
```

上述临时密钥只供本地试用；正式部署须长期保存同一密钥。默认监听 `127.0.0.1:8080`，数据库写入 `./data/rollbook.db`。`bootstrap-user` 只允许创建首个用户，密码从终端读取；也可在首次 `serve` 前配置 `ROLLBOOK_BOOTSTRAP_USER` 与 `ROLLBOOK_BOOTSTRAP_PASSWORD` 或 `ROLLBOOK_BOOTSTRAP_PASSWORD_FILE` 自动创建。访问 `/` 可打开清册网页，`/healthz` 返回 `ok`。网页登录的会话 Cookie 默认带 `Secure`，实际访问需要 HTTPS 反向代理。环境变量 `ROLLBOOK_LISTEN_ADDR` 和 `ROLLBOOK_DB_PATH` 可修改监听地址与数据库路径。首次启动自动迁移；再次启动不会重复执行已应用的迁移。数据库使用 WAL、外键约束和 FTS5。

新设或重设密码不得包含用户名（不区分大小写）、不得使用常见弱口令，且不超过 bcrypt 的 72 字节上限；已有密码无需修改。网页右上角头像提供个人设置、改密与退出操作；只有具备可用本地口令的账号可修改密码（须输入旧密码）。改密或退出所有设备会撤销当前及其他清册浏览器会话，退出登录仅撤销当前会话；两种退出都不会退出上游 SSO 或撤销 API Key。本机管理员可运行 `rollbook reset-password <login>`，在终端交互输入至少 8 字符，且大写、小写、数字、符号四类至少三类的新密码。此命令只操作 `ROLLBOOK_DB_PATH` 指向的已有数据库文件，不接受命令行明文密码，成功后撤销该账号的所有会话。重置后该账号可从重置前被锁定的 IP 使用新密码立即登录；其他账号的 IP 限速不受重置影响。

登录失败按规范化账号和客户端 IP 分别计数，默认第 5 次失败锁定 15 分钟，指数退避上限 1 小时；可通过 `ROLLBOOK_LOGIN_THRESHOLD`、`ROLLBOOK_LOGIN_LOCK_DURATION`、`ROLLBOOK_LOGIN_MAX_BACKOFF` 调整。日志记录失败及锁定类别，不记录口令、Cookie 或 Key。默认只使用 TCP 对端 IP，忽略 `X-Forwarded-For`。仅在单跳可信反向代理部署时，才将代理地址段填入 `ROLLBOOK_TRUSTED_PROXY_CIDRS`（逗号分隔的 CIDR）；届时采用代理追加的最后一个 `X-Forwarded-For` 地址。多跳代理应先在入口规范化转发头。清册 URL 字段仅接受绝对 HTTP/HTTPS 地址，页面展示链接时再次检查协议。页面响应带固定 CSP、防嵌入、防 MIME 嗅探及无来源引用头。

待处理导入最多每提交者 10 份、全局 100 份；提交 7 天后自动过期并清除原始 YAML，服务启动及每小时清理一次。已批准、驳回和过期的记录保留状态、提交者、时间及审批人等元数据，原始文件不再保留。

REST `/api/` 与 Streamable HTTP `/mcp` 使用 `Authorization: Bearer <key>`；网页使用用户会话。启动服务或创建首用户须设置至少 32 字节的 `ROLLBOOK_AUTH_PEPPER`，或使用 `ROLLBOOK_AUTH_PEPPER_FILE` 指向密钥文件；两者不能同时设置。文件内容按原始字节读取，不裁剪换行。该值应保存在部署环境中，不要提交到仓库。网页可搜索和维护资产、查看分层拓扑与递归影响面、审核提议并管理 Key。key 生成后只显示一次。MCP 支持 `initialize`、`tools/list`、`tools/call` 的 JSON 响应模式，工具调用与 REST 共用应用服务和权限检查。`check_versions` 由直接写 key 或网页用户手动触发；服务启动后补查过期项，并每小时扫描一次，确保成功记录不超过 24 小时未检查。

CLI 使用与服务端相同的 REST API。首次运行 `rollbook login`，依次输入服务地址和网页生成的 Key；交互输入 Key 时终端不回显。配置写入 `~/.config/rollbook/config.toml`（或系统的用户配置目录），文件权限为 `0600`。用 `rollbook --profile home login` 保存另一套配置；最近登录的 profile 是默认值。每次调用可用 `--profile` 选择。环境变量 `ROLLBOOK_URL`、`ROLLBOOK_KEY` 分别覆盖选中 profile 的地址和 Key；地址都未设置时默认 `http://127.0.0.1:8080`。环境变量只对当前进程生效。无 Key 时 `--help`、`version` 和 `completion` 仍可运行。

网页登录后可从「Agent 接入」页复制基于当前访问地址的 HTTP/stdio MCP 配置、下载 `rollbook-skill.zip`，或前往当前版本的 CLI Release。Skill zip 仅允许网页登录会话下载，内容固定为 `rollbook/SKILL.md`；Key 占位符需在本机替换。新 Key 的明文只显示一次，配置片段默认保留占位符，选择临时带入后关闭对话框即清除。

仅支持 stdio 的 MCP 客户端可配置 `{"mcpServers":{"rollbook":{"command":"rollbook","args":["mcp"]}}}`；先在同一用户下执行 `rollbook login`，或设置 `ROLLBOOK_URL` 和 `ROLLBOOK_KEY`。多套清册可在 `args` 中写入 `["--profile","home","mcp"]`。桥接按 JSON-RPC 每行一条消息读写，标准输出只包含协议响应，错误写入标准错误；客户端关闭 stdin 后退出。远端须提供 Rollbook 的 `/mcp` JSON 响应模式。

## 网页使用

主菜单六项为「总览、资产、拓扑、待处理、Agent 接入、设置」；全局搜索在顶栏，手机端由抽屉菜单进入。总览先列出待确认提议、有新版本和待核实资产的任务入口，再按站点显示主机、服务及入口，可直接跳转拓扑或搜索设备与服务。搜索支持 IP、域名、端口、服务名等，结果展示相关主机、服务、入口和访问方式；只把安全的 HTTP/HTTPS URL 渲染为链接。

「资产」可按类型录入并浏览站点、主机、网段、服务等；主机可切换列表与树，站点视图可查看接入、网段和路由设备。资产详情展示部署与依赖、访问入口、版本、笔记和变更记录；服务与主机可在详情第一屏查看影响面。主机停用时先预览子主机、服务和入口影响；停用选项只作用于当前主机的直接对象。

资产维护的「站点」页按位置层级显示卡片。点击卡片正文打开详情；有下级位置时，右侧独立按钮负责展开或收起。

「拓扑」提供分层拓扑、访问链和影响面三视图，可按站点筛选并从详情聚焦对象。站点卡片内容溢出时可在卡片内滚动；主内容滚动条位于视口右缘，侧栏保持固定。「待处理」集中待确认提议、待核实资产和有新版本；提议显示差异，批量驳回须确认。「Agent 接入」管理 Key、MCP、Skill 和 CLI。「个人设置」显示账号信息与清册会话操作；「设置」提供备份恢复及系统配置入口。语言选择位于顶栏。浏览器语言决定初始界面语言，手动切换后在该浏览器记住选择。

## 安装与版本

正式 tag `vX.Y.Z` 会在同一构建流程发布 GHCR 镜像、Linux/macOS/Windows 的 amd64/arm64 压缩包、`SHA256SUMS`、`install.sh` 和自动生成的 Release notes。Linux/macOS 可以下载 Release 中的 `install.sh` 后运行，默认安装到 `~/.local/bin`；可通过 `ROLLBOOK_VERSION=v0.13.0` 固定已核验版本，`ROLLBOOK_INSTALL_DIR` 调整安装目录。Windows 请从 Release 下载对应压缩包并核对 SHA256。安装程序只替换本机 CLI 二进制，不更改数据库。`rollbook version`、网页登录页与登录后的页脚、`GET /api/version` 与 MCP `initialize.serverInfo` 使用同一构建版本；网页中 tag 显示版本号，main 显示版本号和短提交号。

YAML 备份当前使用 `schema_version: 1`。导入仅接受程序支持的 schema 版本；新版若提高 schema 版本，会先提供升级路径，不承诺新版 YAML 能由旧程序导入。数据库在启动时执行向前迁移；不承诺降级读取已迁移数据库，升级前须保留完整文件级快照。完整回退流程见下文“升级和回退”。

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

宿主机需要 Docker Engine 和 Compose 插件。默认镜像是 `ghcr.io/cosmogao/rollbook:${ROLLBOOK_VERSION}`；试运行可用 `latest`，正式部署请在 `.env` 固定已验证的 `vX.Y.Z`（当前可固定已核验的 `v0.13.0`）。在仓库目录运行：

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

从 `v0.13.0` 升级到已发布的 `v0.14.0` 前，使用已核验的正式镜像 `ghcr.io/cosmogao/rollbook:v0.14.0@sha256:5b116dabebd401bcda38197db329680f7ca05fc2c277fd9f4ecb6998f39a01bc`，不使用 `main`、`latest` 或 `sha-*` 作为正式版。停服后按下方命令保存完整 `/data`（含 SQLite、`-wal`、`-shm`）及独立 `/backups` 文件级快照，保留原配置、pepper 与身份源密钥，并在隔离环境验证恢复。`018_feishu_email.sql` 是前向迁移；回退先恢复升级前快照与当时配置密钥，再用 `ghcr.io/cosmogao/rollbook:v0.13.0@sha256:9946132f927915687972e257b27bca8e74505d526427e0aa5cc3003f5be18e5c`，不可让旧镜像读取新库。完整 YAML 不含页面凭据，也不能代替快照。以下 `v0.13.0` 升级段是上一版历史步骤。

从 `v0.12.0` 升级到已发布的 [v0.13.0 Release](https://github.com/CosmoGao/rollbook/releases/tag/v0.13.0) 前，须按上方 `v0.13.0` 章节保存完整 `/data`、独立 `/backups`、原配置与密钥材料，并在隔离环境验证恢复。下方命令演示停服与文件级快照；确认快照可恢复后，将 `.env` 的 `ROLLBOOK_VERSION` 设为 `v0.13.0`，运行 `docker compose pull && docker compose up -d`，检查 `docker compose ps`、`/healthz` 和网页登录。回退须先停服，恢复升级前完整快照与原配置密钥，再启动 `v0.12.0` 正式镜像，不能让旧镜像读取升级后的数据。下方 `ROLLBOOK_VERSION=v0.12.0` 是上一版历史升级步骤。

从 `v0.11.0` 升级到已发布的 `v0.12.0` 前，先保存 `.env`、原 `ROLLBOOK_AUTH_PEPPER` 或 `ROLLBOOK_AUTH_PEPPER_FILE` 文件、身份源环境变量或其 `_FILE` 文件，以及认证配置加密所需密钥材料。先停服务，再复制完整 `/data` 数据卷；SQLite 数据库及 `-wal`、`-shm` 文件须一起保存，独立使用 `/backups` 时也保存该卷。完整 YAML 含账号及 Key 哈希，应作为敏感数据保护，但它不含页面身份源凭据，不能代替文件级快照。Compose 示例（目录须自行妥善保护）：

```sh
docker compose stop rollbook
mkdir -p snapshot-data snapshot-backups
docker compose cp rollbook:/data/. ./snapshot-data/
docker compose cp rollbook:/backups/. ./snapshot-backups/
```

在隔离环境验证快照可恢复，且原 pepper 和身份源配置已安全保留。相对 `v0.11.0`，本版无新增数据库迁移、无密钥方案变更；不要重复执行或编造 `017_auth_providers.sql`。核对正式 [v0.12.0 Release](https://github.com/CosmoGao/rollbook/releases/tag/v0.12.0) 和镜像后，将 `.env` 的 `ROLLBOOK_VERSION` 改为 `v0.12.0`，执行 `docker compose pull && docker compose up -d`，检查 `docker compose ps`、`/healthz` 和网页登录。不要让新旧版本同时使用同一数据库。页面凭据由原 pepper 派生密钥解密；原 pepper 遗失或更换时，先在本机运行 `rollbook auth local-login enable` 保留恢复入口，再重新填写页面凭据，必要时用 `rollbook reset-password <已有清册登录名>` 恢复本地口令。全量 YAML 导入会清除目标库页面身份源配置，并拒绝本地入口已关闭的文件。

**数据库只保证向前迁移。**回退时先停服务，恢复升级前完整 `/data` 与独立使用的 `/backups` 快照、原 `.env`、pepper、身份源环境配置及密钥材料，再启动 `v0.11.0` 正式镜像。不能只切换旧镜像读取升级后的数据库。完整 YAML 不能恢复页面凭据；清册 YAML 导出只含资产。GitHub Actions 的 tag 流水线发布正式镜像与 CLI，不会部署你的实例。

## 备份与恢复

服务启动时及每 24 小时在 `ROLLBOOK_BACKUP_DIR` 指定目录原子写入带唯一时间戳的日备份（0600）；未设置时使用数据库目录。首次启动及每 7 天另写周备份。默认保留最近 7 份日备份、4 份周备份，仅轮转 `rollbook-daily-*` 和 `rollbook-weekly-*` 受管文件。`ROLLBOOK_BACKUP_INTERVAL` 与 `ROLLBOOK_BACKUP_WEEKLY_INTERVAL` 使用 Go duration 格式，例如 `1h`、`168h`；`ROLLBOOK_BACKUP_DAILY_KEEP` 与 `ROLLBOOK_BACKUP_WEEKLY_KEEP` 是正整数。周期或保留数无效时使用默认值。网页的「备份与恢复」可列出受管文件的时间和大小、下载或选择一份提交恢复，也可上传 YAML。恢复预览列出每类对象的新增、修改、删除及将丢失的变更；审批时若数据已变化会返回冲突，需重新提交。恢复前必须成功保存当前库快照；批准后会清除所有会话，需重新登录。完整备份含密码与 key 的哈希，请按敏感文件管理。

CLI 使用 `ROLLBOOK_KEY`：`rollbook export > catalog.yaml` 下载清册；`rollbook import catalog.yaml` 校验并提交待确认导入，网页用户批准后才会恢复。只读 key 可以导出，提议/直接写 key 可以提交导入，任何 key 都无法下载完整备份或批准恢复。未知 `schema_version` 和校验不通过的文件会被拒绝，旧数据库保持不变。

CSV 合并导入支持 `host`、`network`、`ip`、`service` 四类对象。网页「备份与恢复」可下载各类模板并提交 CSV；CLI 用 `rollbook template csv host > hosts.csv` 下载模板，用 `rollbook import csv host hosts.csv` 提交，其他类型替换 `host` 即可。CLI 使用所选 profile 的服务地址与 Key。文件提交后进入与 YAML 合并导入相同的预览和审批队列；即使使用提议级 Key，也须由网页用户批准后才能写入。导入文件上限 16 MiB；待处理数量与 7 天过期规则同 YAML。

CSV 使用 UTF-8 编码（可带 BOM），首行是模板列名，列名区分大小写；未知、重复、缺少必填列会返回行列错误。每行 `id` 必须带类型前缀（如 `host:`），文件内重复 ID 会被拒绝。新增对象不填 `rev`；更新现有对象必须填写当前正整数 `rev`，不一致会在预览中标为冲突。空单元格表示不提供该字段，不会把现有值清空；必填列不能留空。JSON 列填写合法 JSON；URL 列只接受绝对 HTTP/HTTPS 地址。关联列填对象 ID，可引用库中已有对象或同批文件中的对象；不存在的引用会显示定位错误。CSV 会映射到原合并模型，原 YAML 导入格式与行为不变。

网页业务页使用固定版前端与 VI 规范 v1.0.0 的同名浅深语义颜色、统一控件轮廓与列表/详情/表单/总览布局；资产表单的 JSON 输入错误会在字段旁说明，修正时保留已填内容。各路由与验收截图见 [前端实施记录](docs/frontend-implementation.md)。

## 前端开发

网页源码位于 `web/src/`，使用 React、TypeScript 和 Vite。首次开发运行 `npm ci --prefix web`，修改后运行 `npm run typecheck --prefix web` 与 `npm run build --prefix web`。构建产物 `web/dist/` 随仓库提交，并由 Go 内嵌，因此直接执行 `go build ./cmd/rollbook` 无需 Node。提交前确认重新构建后的 `web/dist/` 与源码一致。
