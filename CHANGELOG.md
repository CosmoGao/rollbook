# 更新记录

按 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/) 分类，依据各 Release、tag 区间和已合并 PR 编写。公开 Release 从 v0.3.0 开始；另有 `v0.0.1` tag，但无对应 Release。未发现 v0.1.0、v0.2.0 的发布证据，不为其编造条目。

## 未发布

### 变更

- 全站现有业务视图按固定版前端与 VI 规范 v1.0.0 统一语义颜色、控件轮廓、长内容排版与浅深主题；保留原有业务、API 与授权行为。
- 资产表单的 JSON 字段错误与输入关联并保留内容；待处理批量驳回弹层采用可管理焦点的 Dialog。
- 拓扑主机和子资产名称、侧栏账号操作使用规范字号；首次引导关闭后将键盘焦点还给当前导航入口。资产读取权限不足时显示明确提示。
- 拓扑筛选字段标签适配桌面与窄屏正文，窄屏菜单及抽屉关闭按钮扩大至 44px 触控目标；桌面资产表单输入恢复 36px 默认高度。

### 验证

- Playwright 对现有路由在五种 CSS 视口宽度、浅深两主题下生成同数据截图；发布前仍须完成独立验收和 Release 核对。

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
