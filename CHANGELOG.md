# 更新记录

按 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/) 分类，依据各 Release、tag 区间和已合并 PR 编写。公开 Release 从 v0.3.0 开始；另有 `v0.0.1` tag，但无对应 Release。未发现 v0.1.0、v0.2.0 的发布证据，不为其编造条目。

## 未发布

暂无新条目。

## v0.13.0（待发布）

版本号按 [发版检查](docs/releasing.md) 及现有次版本归档方式选择。正式 tag、Release 和产物尚未创建；本节以 `v0.12.0..main` 的 [#73](https://github.com/CosmoGao/rollbook/pull/73)、[#74](https://github.com/CosmoGao/rollbook/pull/74) 为依据，其中 #73 仅补齐上一版文档与 Release 链接。发布后再补真实 Release 链接；合入 `main` 不等于发布或部署。

### 修复

- 飞书认证设置保存时，缺少 App ID、凭据动作无效或缺少新凭据、回调地址不符，返回中英文可定位的字段级 `422 validation`（`client_id`、`secret_action`、`redirect_url`）；凭据仍不回显。设置页展示的非 443 HTTPS 公开回调可保存（[#74](https://github.com/CosmoGao/rollbook/pull/74)）。
- HTTP API 公开 origin 使用直连 `Host`/TLS，或仅采信可信直接代理的单值转发头；伪造、冲突、多值或逗号链拒绝。LDAP/OIDC 同类安全约束保持。飞书未启用时登录页无飞书入口。详见[公开地址与校验](docs/public-auth-origin.md)。

### 升级注意事项

- 相对 `v0.12.0` 无新增数据库迁移、无密钥方案变更；本轮确有 HTTP API 公开 origin 与设置校验变化。真实 LDAP、OIDC、飞书 IdP 尚未联调，没有新增全站 RBAC 或群组映射。本地登录开关和生产身份源配置保持不变。
- 升级前停服，保存完整 `/data` 文件级快照（包括 SQLite 数据库、`-wal`、`-shm`）及独立使用的 `/backups` 卷，保留原环境变量、`ROLLBOOK_AUTH_PEPPER` 或其 `_FILE`、身份源环境变量或 `_FILE` 与认证配置加密密钥材料；在隔离环境验证恢复。完整 YAML 不能代替快照或恢复页面凭据。回退先停服，恢复升级前完整快照、原配置和密钥，再运行 `v0.12.0` 正式镜像；不能只换旧镜像读取升级后数据。丢失或更换 pepper 时，先在本机运行 `rollbook auth local-login enable` 保留恢复入口，再重新填写页面凭据。

## [v0.12.0](https://github.com/CosmoGao/rollbook/releases/tag/v0.12.0)

本版已正式发布：[v0.12.0 Release](https://github.com/CosmoGao/rollbook/releases/tag/v0.12.0)；tag 指向 `47f96d343eb91c06c6ceb23122de7a1bd058a3af`。本节依据 `v0.11.0..v0.12.0` 的 [#70](https://github.com/CosmoGao/rollbook/pull/70)、[#71](https://github.com/CosmoGao/rollbook/pull/71)、[#72](https://github.com/CosmoGao/rollbook/pull/72) 与正式 Release 归档。#70 仅补齐上一版文档和截图证据，#72 为本版发布文档，均不是新功能；#71 是本版用户可见修复。已核验正式镜像 `ghcr.io/cosmogao/rollbook:v0.12.0`（index `sha256:ccb1ead248f7d00d365473a581eab8eeb4d01a6407ba11c25dca041415077f49`）及 Release 的六平台 CLI、`install.sh` 和校验清单。正式发布不会自动升级现有实例。本段发版后文档补正属于后续 `main`，不在不可变的 `v0.12.0` tag 内。

### 修复

- 全站独立文本 `input`、`textarea`、`select` 的焦点反馈与自身 `border-box` 外框重合；搜索框和 HEADER 组合控件由外层绘制唯一外框。保留错误态、可辨识的键盘焦点和 forced-colors 支持；原生 select 的系统展开层不属于页面 CSS 修复范围。路由矩阵及脱敏浏览器截图见 [文本控件证据](docs/control-geometry.md)（[#71](https://github.com/CosmoGao/rollbook/pull/71)）。

### 升级注意事项

- 相对 `v0.11.0` 无新增数据库迁移、无密钥方案变更。升级前停服并保存完整 `/data` 文件级快照（含 SQLite 数据库、`-wal`、`-shm`）；独立使用 `/backups` 时也保存该卷。保留原环境变量、`ROLLBOOK_AUTH_PEPPER` 或其 `_FILE` 文件、身份源环境变量或 `_FILE` 文件及认证配置加密所需密钥材料，在隔离环境验证快照可恢复。完整 YAML 不能代替快照，也不能恢复页面身份源凭据。
- 回退先停服，恢复升级前完整快照、原配置与密钥，再运行 `v0.11.0` 正式镜像；不能只换旧镜像读取升级后的数据。页面密文只能用原 pepper 解密；若丢失或更换，须先在本机运行 `rollbook auth local-login enable` 保留本地恢复入口，再重新填写页面凭据。真实 LDAP、OIDC、飞书 IdP 尚未联调，没有新增全站 RBAC 或群组映射；现有登录策略保持不变。

## [v0.11.0](https://github.com/CosmoGao/rollbook/releases/tag/v0.11.0)

本版已正式发布：[v0.11.0 Release](https://github.com/CosmoGao/rollbook/releases/tag/v0.11.0)；tag 指向 `6600811f2468a31565ca6dc6886e8168ce163f11`。本节依据 `v0.10.0..v0.11.0` 的已合并 [#67](https://github.com/CosmoGao/rollbook/pull/67)、[#68](https://github.com/CosmoGao/rollbook/pull/68) 归档；[#66](https://github.com/CosmoGao/rollbook/pull/66) 仅补齐上一版 Release 链接。正式发布不等于现有实例已部署。

### 改进

- 系统认证设置改为本地登录、LDAP、OIDC、飞书、账号与绑定、认证审计六组导航，支持分组深链、独立保存反馈与未保存更改保护（[#68](https://github.com/CosmoGao/rollbook/pull/68)）。备份与个人设置仍在独立路径。

### 修复

- HEADER 的桌面 `form.top-search`、窄屏 `form.mobile-search` 和 `header.topbar label.theme-control` 由组合控件绘制单层边框与贴合边界的焦点反馈，内部输入、按钮和 select 不再绘制重复完整边框或偏移焦点环（[#67](https://github.com/CosmoGao/rollbook/pull/67)）。搜索快捷键、提交和主题持久化保持原有行为。

### 升级注意事项

- 相对 `v0.10.0` 无新增数据库迁移、无密钥方案变更。升级前仍须停服并保存完整 `/data` 文件级快照，包括 SQLite 数据库及 `-wal`、`-shm`；独立使用 `/backups` 时也保存该卷。保留原环境变量、`ROLLBOOK_AUTH_PEPPER` 或其 `_FILE` 文件、身份源环境变量或 `_FILE` 文件及认证配置加密所需密钥材料，并在隔离环境验证快照可恢复。完整 YAML 不能代替快照，也不含页面身份源凭据。
- 回退须先停服，恢复升级前完整快照、当时配置及密钥，再启动 `v0.10.0` 正式镜像；不能只换旧镜像读取升级后的数据。页面密文只能用原 pepper 解密；若丢失或更换，须先在本机运行 `rollbook auth local-login enable` 保留恢复通道，再重新填写页面凭据。真实 LDAP、OIDC、飞书 IdP 尚未联调；没有新增全站 RBAC 或群组映射。

## [v0.10.0](https://github.com/CosmoGao/rollbook/releases/tag/v0.10.0)

本节依据 `v0.9.0..v0.10.0` 的已合并 [#62](https://github.com/CosmoGao/rollbook/pull/62)、[#63](https://github.com/CosmoGao/rollbook/pull/63)、[#64](https://github.com/CosmoGao/rollbook/pull/64)、[#65](https://github.com/CosmoGao/rollbook/pull/65) 归档；#62 只补齐上一版 CHANGELOG 的 Release 链接，#65 为本版发布文档。

### 新增

- 设置目录新增认证设置页，可管理 LDAP、OIDC、飞书身份源、用户与显式绑定、本地登录，以及分页查看认证审计；页面凭据用现有 pepper 派生的密钥加密保存，不进入 YAML 备份。

### 修复

- 全局搜索框选中高亮与真实框线重合：深浅主题的悬停、聚焦状态沿同一条 `border-box` 框线变化，不再出现偏离的外圈（[#63](https://github.com/CosmoGao/rollbook/pull/63)）。

### 安全与兼容

- 环境变量按身份源整体优先于页面配置；外部登录流程在配置切换后安全失败。关闭本地登录与修改身份源时核对已启用源和近一小时真实试登录，变更撤销相关会话。
- 完整 YAML 导入清除目标库页面身份源配置，并拒绝本地入口已关闭的文件。升级和回退须保留升级前完整数据卷快照、当时的 pepper 和环境配置，不能只切换旧镜像。

### 升级注意事项

- 从 `v0.9.0` 升级会执行前向迁移 `017_auth_providers.sql`：新增加密凭据表 `auth_provider_settings` 和 `auth_pending.config_version`。停服务后保存完整 `/data` 文件级快照（含 SQLite 数据库、`-wal`、`-shm`），独立使用 `/backups` 时也保存该卷；同时保存原 `ROLLBOOK_AUTH_PEPPER` 或 `_FILE` 文件、身份源环境变量或 `_FILE` 文件及所需密钥材料。完整 YAML 不含页面凭据，不能代替快照。
- 回退须停服务，恢复升级前完整快照及当时配置和密钥，然后启动 `v0.9.0` 正式镜像；不能让旧镜像直接读取迁移后的数据库。原 pepper 丢失会使页面密文无法解密，须先用本机 `rollbook auth local-login enable` 保留本地入口，再重新填写页面凭据。真实 LDAP、OIDC、飞书 IdP 尚未联调；没有新增全站 RBAC 或群组映射。

## [v0.9.0](https://github.com/CosmoGao/rollbook/releases/tag/v0.9.0)

本节依据 `v0.8.0..main` 的已合并 [#55](https://github.com/CosmoGao/rollbook/pull/55)、[#56](https://github.com/CosmoGao/rollbook/pull/56)、[#57](https://github.com/CosmoGao/rollbook/pull/57)、[#58](https://github.com/CosmoGao/rollbook/pull/58)、[#59](https://github.com/CosmoGao/rollbook/pull/59)、[#60](https://github.com/CosmoGao/rollbook/pull/60) 归档。

### 新增

- 新增右上角头像账号入口和个人设置路由；按会话能力展示本地改密或上游密码指引，系统设置保留备份恢复。

- 可独立配置的 LDAP/LDAPS、通用 OIDC 与飞书 OAuth 登录入口；外部身份需本机显式绑定并列入允许名单。
- 会话身份类型与本地改密能力契约、本机本地入口关闭及恢复命令、认证操作审计。

### 修复

- 登录页与应用顶栏的主题控件改为显式选择浅色、深色或跟随系统；单次选择即时更新主题并保留原有浏览器持久化。
- 全局搜索输入框与外框不再重复描边，键盘焦点由组合控件统一显示；新增可键盘操作的明确清空按钮。
- 拓扑站点卡片的内容区在溢出时使用原生滚动，并优先接收卡片内滚轮；主内容滚动条贴近视口右缘，固定侧栏仍独立。

### 变更

- 资产维护的站点位置树改为左对齐的卡片导航；打开详情与展开下级位置使用独立按钮，保持原有导航和层级语义。
- 数据库支持无本地口令的外部账号与认证源会话；完整 YAML 备份包含绑定和入口设置，旧版备份可恢复。

### 升级注意事项

- 从 `v0.8.0` 升级前，保存 `.env`、认证 pepper 及所有已配置身份源密钥，并在停服务后保存完整 `/data` 数据卷快照；独立使用 `/backups` 时也保存该卷。SQLite 的数据库、WAL 与 SHM 文件须一起保存。完整 YAML 备份不能替代文件级快照。
- 本版的用户表允许无本地口令账号，并新增身份绑定、认证设置、审计及认证源会话结构；数据库迁移只保证向前执行。回退须停服务、恢复升级前的完整 `/data`（及独立 `/backups`）快照和原有配置与密钥，再启动 `v0.8.0` 正式镜像；不能仅换回旧镜像读取迁移后的数据库。
- 旧版完整 YAML 可恢复到新版库，缺省外部绑定为空且本地入口开放；新版完整 YAML 即使仍为 `schema_version: 1`，也不保证旧程序能够导入其中的身份绑定与设置。真实 LDAP、OIDC、飞书身份源联调尚未进行。

## [v0.8.0](https://github.com/CosmoGao/rollbook/releases/tag/v0.8.0)

### 变更

- 全站现有业务视图按固定版前端与 VI 规范 v1.0.0 统一语义颜色、控件轮廓、长内容排版与浅深主题；保留原有业务、API 与授权行为。
- 资产表单的 JSON 字段错误与输入关联并保留内容；待处理批量驳回弹层采用可管理焦点的 Dialog。
- 拓扑主机和子资产名称、侧栏账号操作使用规范字号；首次引导关闭后将键盘焦点还给当前导航入口。资产读取权限不足时显示明确提示。
- 拓扑筛选字段标签适配桌面与窄屏正文，窄屏菜单及抽屉关闭按钮扩大至 44px 触控目标；桌面资产表单输入恢复 36px 默认高度。

### 升级注意事项

- 本版相较 v0.7.0 未新增数据库迁移；升级前仍须保存 `.env`、认证密钥、完整 `/data` 快照，独立启用 `/backups` 时也须备份。回退先停服务、恢复升级前完整快照，再启动旧版；不要让旧版直接读取新库。
- 浏览器用例已对现有路由在五种 CSS 视口宽度、浅深两主题下生成同数据截图；真人读屏及系统软键盘专项尚未验证。

## [v0.7.0](https://github.com/CosmoGao/rollbook/releases/tag/v0.7.0)

### 新增

- 设备与主机新增十类：智能电视、游戏机、机顶盒与播放器、音箱与音频设备、摄像头与门铃、打印与扫描设备、智能家居中枢与网关、家电、传感器与控制器、其他设备；原有六类保留。无需为设备登记 OS、IP、管理 URL 或服务。
- 设备列表支持类别精确筛选，并与站点、搜索和树视图组合；列表与全局搜索可按名称、ID、别名、标签、型号和类别检索。

### 变更

- 网页统一使用「设备与主机」入口；设备详情可直接新增由该设备承载的服务。服务可由任意真实设备或计算实例承载，仍须指定承载对象；纯外部/SaaS 服务尚不能作为无宿主 service 登记。
- 分层拓扑默认覆盖实体设备和无部署父的独立实例，按需展开部署实例、服务与入口；无 IP、无依赖关系或遗留未归属站点的设备仍可定位。`physical/virtual/all` 参数保持兼容。
- SQLite 向前迁移扩充 `hosts.kind` 约束并重建搜索索引；旧记录不自动重分类，REST/MCP/CLI 的 host 契约、CSV 旧列名和 YAML `schema_version: 1` 保持不变（[#48](https://github.com/CosmoGao/rollbook/pull/48)）。

### 升级注意事项

- 升级前保存 `.env`、认证密钥以及完整 `/data` 卷快照；单独使用 `/backups` 时也须备份。回退须恢复升级前完整快照，不能让旧镜像直接打开迁移后的数据库。
- 含新增设备类别的备份不能直接导入 v0.6.0；不要在新旧版本间共用同一数据库。

## [v0.6.0](https://github.com/CosmoGao/rollbook/releases/tag/v0.6.0)

### 新增

- 总览任务入口、待核实范围及提议差异（[#45](https://github.com/CosmoGao/rollbook/pull/45)）。
- 分层拓扑画布的拓扑、访问链和影响面三视图及站点筛选；服务和主机详情展示递归影响面（[#46](https://github.com/CosmoGao/rollbook/pull/46)、[#43](https://github.com/CosmoGao/rollbook/pull/43)）。

### 变更

- 搜索关联上下文、站点主机树及资产录入流程得到完善（[#44](https://github.com/CosmoGao/rollbook/pull/44)）。

## [v0.5.1](https://github.com/CosmoGao/rollbook/releases/tag/v0.5.1)

### 变更

- 固定桌面侧栏和账号区，统一浅色/深色主题、页面标题、资产页签与表格、详情面包屑及待处理工具栏；手机端保留抽屉菜单。
- 待处理增加数量角标，批量驳回增加确认。

### 升级注意事项

- 本版仅修整前端，无新的数据库迁移；升级和回退步骤见 [Release](https://github.com/CosmoGao/rollbook/releases/tag/v0.5.1)。

## [v0.5.0](https://github.com/CosmoGao/rollbook/releases/tag/v0.5.0)

### 新增

- Agent 接入页整合 Key 管理、MCP 配置、Skill zip 和 CLI Release 入口；新增深浅色主题和手机布局。

### 变更

- 网页迁移至 React、TypeScript、Vite 与 shadcn/ui；主菜单改为总览、资产、拓扑、待处理、Agent 接入、设置六项，全局搜索移至顶栏。仍保持单一 Go 二进制、镜像及 Compose 部署方式。

### 升级注意事项

- 升级前备份 `.env`、认证密钥、完整 `/data` 卷及启用时的 `/backups` 卷；回退须恢复升级前数据库快照，详见 [Release](https://github.com/CosmoGao/rollbook/releases/tag/v0.5.0)。

## [v0.4.2](https://github.com/CosmoGao/rollbook/releases/tag/v0.4.2)

### 新增

- 登录前后展示统一构建版本（[#38](https://github.com/CosmoGao/rollbook/pull/38)）。

### 安全

- 统一新设、重设和修改密码的复杂度规则，更新引导提示；已有密码仍可登录（[#39](https://github.com/CosmoGao/rollbook/pull/39)）。

## [v0.4.1](https://github.com/CosmoGao/rollbook/releases/tag/v0.4.1)

### 新增

- 首次管理员环境变量引导和可选 Compose secret（[#36](https://github.com/CosmoGao/rollbook/pull/36)）。

### 安全与配置

- 通过 `ROLLBOOK_TRUSTED_PROXY_CIDRS` 仅信任指定 TCP 代理的 HTTPS 转发头，按请求决定 Cookie 安全标记；部署时须规范化转发头（[#37](https://github.com/CosmoGao/rollbook/pull/37)）。

## [v0.4.0](https://github.com/CosmoGao/rollbook/releases/tag/v0.4.0)

### 新增

- MCP resources 与 prompts；中英文界面与英文枚举别名兼容（[#28](https://github.com/CosmoGao/rollbook/pull/28)、[#29](https://github.com/CosmoGao/rollbook/pull/29)）。
- 服务/主机拓扑与访问链、主机树和网段 IP 占用（[#30](https://github.com/CosmoGao/rollbook/pull/30)、[#31](https://github.com/CosmoGao/rollbook/pull/31)）。
- 手机端查询与主屏入口、Key 元数据管理（[#34](https://github.com/CosmoGao/rollbook/pull/34)、[#35](https://github.com/CosmoGao/rollbook/pull/35)）。

### 变更

- 收敛搜索关联，完善排序和安全链接；统一详情、最近变更及共享上下文（[#32](https://github.com/CosmoGao/rollbook/pull/32)、[#33](https://github.com/CosmoGao/rollbook/pull/33)）。

## [v0.3.0](https://github.com/CosmoGao/rollbook/releases/tag/v0.3.0)

### 新增

- 首个公开 Release 包含清册模型、关系、搜索、账号和 Key 权限、REST/MCP/CLI、网页审批、上游版本检查及 YAML 备份恢复（[#4](https://github.com/CosmoGao/rollbook/pull/4)、[#5](https://github.com/CosmoGao/rollbook/pull/5)、[#6](https://github.com/CosmoGao/rollbook/pull/6)、[#7](https://github.com/CosmoGao/rollbook/pull/7)、[#8](https://github.com/CosmoGao/rollbook/pull/8)、[#9](https://github.com/CosmoGao/rollbook/pull/9)）。
- stdio MCP 桥、多平台 CLI 安装包、CSV 合并导入、备份轮转及恢复预览（[#24](https://github.com/CosmoGao/rollbook/pull/24)、[#26](https://github.com/CosmoGao/rollbook/pull/26)、[#23](https://github.com/CosmoGao/rollbook/pull/23)）。

### 安全与修复

- 登录、账号及 URL 加固；修复反代链、主机换址、HTTP 示例导入及本机重设密码后的 IP 锁定问题（[#20](https://github.com/CosmoGao/rollbook/pull/20)、[#12](https://github.com/CosmoGao/rollbook/pull/12)、[#18](https://github.com/CosmoGao/rollbook/pull/18)、[#19](https://github.com/CosmoGao/rollbook/pull/19)、[#27](https://github.com/CosmoGao/rollbook/pull/27)）。

### 配置注意事项

- 直接 HTTP 登录须显式开启 `ROLLBOOK_ALLOW_INSECURE_HTTP=true`（[#10](https://github.com/CosmoGao/rollbook/pull/10)）。
