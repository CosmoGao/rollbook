# 文本控件外框与焦点几何

本页记录 2026-11-25 在 Chromium 中访问实际路由所得的控件矩阵。浏览器使用本地构建和脱敏 API 模拟数据；没有连接正式实例，也没有读取私人截图。完整的每控件 selector、type、border-box、computed style、浅/深色及 1280/390px 测量见 `control-geometry-evidence/verified.json`。复现前的「筛选记录」测量见 `baseline-pending.json`。

此修复由 [PR #71](https://github.com/CosmoGao/rollbook/pull/71) 合入，拟归档在 `v0.12.0`；截至本文更新，正式 tag、Release、镜像和 CLI 尚未生成，现有实例不会自动更新。截图是 #71 对应源码的本地浏览器证据，不是正式镜像或生产环境截图。目标版本的 Release 与产物须发布后再按实际结果核对。

## 外框策略

- 独立的 `input`（排除 checkbox/radio）、`select`、`textarea` 自己拥有 `border-box` 外框；键盘焦点改该边框颜色，并在框内用 inset 加粗，总计 2px 可见焦点强度。`aria-invalid=true` 保持错误色边框，蓝色内线仍标出焦点。强制颜色模式用向内的 2px `Highlight` outline。
- `form.search-box`、`form.top-search`、`form.mobile-search`、`label.theme-control` 拥有组合控件的外框；其子控件不再绘制自己的框线和焦点阴影。焦点时外框变色并向内加粗。普通按钮、链接、表格行等继续使用原有全局 `:focus-visible`。
- 使用仓库内固定版 `docs/frontend-standard/tokens.css` v1.0.0 的控制框、圆角和焦点 token。`--focus-offset` 仍适用于其他普通焦点环。

## 浏览器路由矩阵

以下 selector 均来自实际 DOM。`verified.json` 记录每条路由在浅/深色 × 桌面/窄屏下的具体控件、类型与尺寸；下表列出内容区域控件，所有登录后页面还包含 HEADER 的 `form.top-search input`（桌面）或 `form.mobile-search input[type=search]`（窄屏）、`#language` 与 `header.topbar label.theme-control select`。

| 路由或入口 | 实际控件和 type | 外框拥有者 |
| --- | --- | --- |
| 未登录 `/` | `#login-form input[name=login]` text、`input[name=password]` password、`#login-language` select、`.login-theme select`；本地/LDAP 切换为 button | 独立控件；登录主题由 `.login-theme` |
| `/` 总览 | `form.overview-search input` search | input |
| `/assets`、`/assets/services`、`/assets/sites`、`/assets/networks` | `.asset-toolbar .filter-fields input` search、`.asset-type-select` select | 各控件自身 |
| `/assets/hosts` | 上述控件，另有 `.asset-toolbar .filter-fields select` 主机类别和站点 | 各控件自身 |
| 资产新建弹窗 | `.asset-form input` text/number、`.asset-form select`（含条件多选）、`.asset-form textarea`；pinned checkbox 不按文本框处理 | 各控件自身；字段依所选资产类型显示 |
| `/search` | `form.search-box input[type=search]`，清空和提交为 button | `form.search-box` |
| `/pending`、`?tab=unverified` | `.pending-toolbar .filter-fields input[type=search]`「筛选记录」 | input |
| `/pending?tab=outdated` | 上述搜索框，另有 Webhook `section.section label input[type=url]`、格式 `select` | 各控件自身；启用勾选为 checkbox |
| `/changes` | `form.filter-bar input` text/date、`form.filter-bar select` | 各控件自身 |
| `/contexts` | 内容区只有归档 checkbox；无文本框 | 不适用 |
| `/topology` 与 `?site=...` | `.topology-toolbar label select` 三个：站点、聚焦对象、展开层级 | 各 select 自身 |
| `/agent` | `.controls select.field` 权限级别；创建 Key 弹窗 `.modal input` text/datetime-local、`select`、`textarea` | 各控件自身；只展示一次的 Key 没有进入截图 |
| `/settings` | 设置入口无内容区文本控件 | 不适用 |
| `/settings/personal` | 改密码弹窗 `.modal input` password | 各 input 自身 |
| `/settings/auth` | LDAP/OIDC/飞书页签中的 `.auth-panel .auth-fields input` text/password、`select`；只读环境值与勾选按实际状态检查 | 各控件自身；不同身份源条件显示 |
| `/settings/backup` | `section.section form.controls select.field`、`input[type=file]`，分别属于 YAML 与 CSV 导入 | 各控件自身 |
| 移动菜单抽屉、首次使用弹窗 | 无可见文本控件；检查键盘焦点仍保留在按钮上 | 不适用 |

资产详情的编辑、删除、笔记、停用和恢复弹窗依对象类型及数据而显示；浏览器测试覆盖了资产新建弹窗。原生 `prompt` 与 select 的操作系统展开层不属于页面 CSS 控制范围。剪贴板回退时临时创建的隐藏 textarea 不属于可见页面控件。

本轮仅修复可见控件的焦点与外框几何；快捷键、主题选择、认证凭据只写不读、登录策略和权限行为不变。真实 LDAP、OIDC、飞书 IdP 尚未联调，也没有新增全站 RBAC 或群组映射。升级和回退按 [README](../README.md#升级和回退) 的文件级快照步骤办理。

## 复现与证据

修复前 `baseline-pending.json` 在 `/pending` 量得「筛选记录」输入框为 310×36px、灰色 1px 边，键盘焦点额外绘制 2px 蓝 outline，`outline-offset: 2px`。修复后同一控件尺寸不变，outline 为 0，边框与 1px inset 同为蓝色。对应的 [修复前截图](control-geometry-evidence/before-pending-light-1280.png) 和 [修复后截图](control-geometry-evidence/after-pending-light-1280.png) 均截取该控件；浅/深色 × 1280/390px 的四组截图使用相同命名规则。

其他代表截图均为焦点状态的实际控件裁切，文件名见 `control-geometry-evidence/`：`after-search-light-1280.png` 是 `form.search-box`；`after-header-top-search-*`、`after-header-mobile-search-*`、`after-header-theme-control-*` 分别对应 HEADER 桌面搜索、窄屏搜索和主题控件。`after-assets-*`、`after-changes-*`、`after-topology-*`、`after-settings-backup-*` 以及 `after-*-dialog.png` 分别对应表中的各组控件。截图只包含脱敏控件，没有口令或 Key 值。

运行：在 `web` 执行 `npm ci && npm run typecheck && npm run build`，再在仓库根目录执行 `npm ci && node --test web/control-geometry.browser.test.cjs`。测试对真实 DOM 逐路由读取 computed style、焦点轮廓和尺寸；另覆盖登录、资产、Key、个人密码与认证页签弹窗，以及「筛选记录」的 hover、输入、错误和强制颜色模式。
