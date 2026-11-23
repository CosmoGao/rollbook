# 前端规范 v1.0.0 接入与阶段边界

## 固定来源与优先级

项目规则优先；视觉、组件和验收以 `frontend-standard/FRONTEND-SPEC.md` 为解释来源，以同目录 `tokens.json` / `tokens.css` 为机器值。固定 tag `v1.0.0`，commit `d20328e1f79c3aa3243fc48f25a581136d86719d`。`AGENTS.md` 只引用这一份规范。旧 `base.css` 绿色和 `style.css` 蓝/锌双主题已映射到语义变量；`--background`、`--foreground`、`--card`、`--input` 等旧别名只作兼容，项目不再维护第二套颜色值。详情见 `frontend-standard/SOURCE.md`。

## 实际入口与页面模板

| 直达 URL | 实际组件 | 模板 | 阶段 |
|---|---|---|---|
| `/` | `Overview.tsx` | 总览 | 已接入；业务截图 |
| `/assets`，`/assets/services`，`/assets/sites`，`/assets/hosts`，`/assets/networks` | `Assets.tsx`、`HostTree.tsx` | 列表/树 | 已接入；业务截图 |
| `/topology` | `Topology.tsx`、`Flow.tsx`、`topology-model.ts` | 拓扑 | 已接入；业务截图 |
| `/pending`，`?tab=unverified|outdated` | `Pending.tsx` | 列表 | 已接入；业务截图 |
| `/agent` | `Keys.tsx`、`AgentGuide.tsx` | 列表/表单 | 已接入；业务截图 |
| `/settings`，`/settings/backup` | `Settings.tsx`、`Backup.tsx` | 设置/表单 | 已接入；业务截图 |
| `/changes`，`/contexts` | `Audit.tsx` | 列表 | 已接入；业务截图 |
| `/search` | `Search.tsx` | 列表 | 已接入；业务截图 |

对象详情 `AssetDetail.tsx`、新建/编辑 `AssetForm.tsx` 与首次引导 `FirstRun.tsx` 是内部视图或弹层，不是独立 URL。`Browse.tsx` 尚有组件，但 `controller.ts` 当前 switch 未挂载，不新增 URL。

## 组件复用与差异

| 现有入口 | 本次映射 | 业务视图核对 |
|---|---|---|
| `web/src/components/ui/button.tsx` | 四变体、36/32/44 尺寸、禁用/加载状态与语义颜色 | 业务页主次操作按同一语义按钮与 CSS 入口呈现 |
| `web/src/components/ui/catalog.tsx` | Tabs 选中语义、Table 表头/行密度、骨架加载语义 | 现有页签、列表、加载与空态；未实现的排序和分页不虚构 |
| 原生 `input/select/textarea`、Radix Dialog | `control-border`、焦点/错误轮廓；登录/改密码保留原行为，抽屉提供关闭 | 资产 JSON 字段错误关联并保留输入；批量驳回改用 Radix 焦点管理 |
| `main.tsx` 与 `controller.ts` | 固定侧栏/顶栏、主内容滚动、当前导航与跳转链接、系统主题 | 业务长内容换行与主内容范围内的表单底栏 |

旧实现的 body 与 `.main` 均可能滚动、系统暗色依赖单独覆盖、桌面与窄屏有不同侧栏逻辑、顶部搜索与侧栏颜色残留旧值。本阶段改为 `html[data-theme=light|dark]` 解析后的单一主题状态；`rollbook.theme=system` 仍保留偏好并响应系统变化。旧 `.crumb` 同级位置标签隐藏，详情页有价值的返回路径保留。既有 React/TypeScript/shadcn/ui/Tailwind、业务 API 与权限模型不变。

## 品牌参数与验收界限

产品名称沿用 Rollbook / 清册，侧栏 `R` 和 `web/public/icon.svg` 保留。正式浅/深 logo、清晰区比例、最小尺寸、完整 favicon/PWA 尺寸及品牌覆盖批准记录尚未提供，记为可配置缺口，不伪造资产。业务视图的页面布局、创建/编辑/筛选/详情、待处理、拓扑、Key 与备份路径由现有和新增浏览器用例回归。实际浏览器 200%/400% 缩放、读屏专项、软键盘、离线、权限不足等状态仍为**未验证**；这些项目须由测试审查补验，不以 320 CSS px 截图等同于 400% 缩放。

## 第一阶段验证记录

- `python3 docs/frontend-standard/validate.py`：166/166 个对比组合、272/272 个结构检查通过。
- `npm run typecheck --prefix web`、`npm run build --prefix web`：通过；仓库没有独立 lint 脚本。
- Chromium Playwright：31 个现有/新增浏览器用例通过。其中 30 个在 `mcr.microsoft.com/playwright:v1.65.0-noble` 执行，依赖 Go 的 HTTP/TLS 用例在仓库已有 `rollbook-playwright-go:latest` 环境单独执行通过。
- `docs/frontend-evidence/` 有真实登录页与总览/壳的 1440、390、320 CSS px 同路由同数据浅深配对截图。键盘焦点、滚动和抽屉关闭有浏览器断言。业务视图、200%/400% 缩放、读屏等为**未验证**，第二阶段覆盖。

## 第二阶段页面与截图证据

使用 `web/ui-refresh.browser.test.cjs` 的固定示例资产集；所有截图均由 Chromium 真页面生成，数据、路由与视口在浅深两张图中一致，无真实账号或 Key。文件名模式：`web/screenshots/v2ri-71/v1-business-{宽度}-{light|dark}-{场景}.png`。宽度为 1440、1024、768、390、320 CSS px，屏高为桌面 900、窄屏 844。测试逐页断言单个 h1、主题和无页面级横向溢出。共 182 张截图，按同路由、同数据、同视口成对保存。

| 真实路由/内部视图 | 场景名 | 视口/主题 | 业务验证 |
|---|---|---|---|
| `/` | `overview` | 五宽度、浅深 | 总览任务与站点入口由 `overview.browser.test.cjs` 回归 |
| `/assets`、`/assets/services`、`/assets/sites`、`/assets/hosts`、`/assets/networks` | `assets`、`assets-services`、`assets-sites`、`assets-hosts`、`assets-networks` | 五宽度、浅深 | 创建、编辑、筛选、树、详情由 `maintenance`、`devices`、`asset-context`、`ui-refresh` 用例回归 |
| `/topology` | `topology` | 五宽度、浅深 | 展开、选中、访问与影响视图由 `layered-topology`、`topology`、`impact` 用例回归 |
| `/pending` 及 `?tab=unverified|outdated` | `pending`、`pending-unverified`、`pending-outdated` | 五宽度、浅深 | 批量选择/驳回与确认焦点由 `ui-refresh` 用例回归 |
| `/agent` | `agent` | 五宽度、浅深 | Key 创建、编辑、吊销及一次性显示由 `agent`、`keys` 用例回归 |
| `/settings`、`/settings/backup` | `settings`、`backup` | 五宽度、浅深 | 账号和备份行为由 `password`、`react`、`maintenance` 用例回归 |
| `/changes`、`/contexts` | `changes`、`contexts` | 五宽度、浅深 | 审计筛选和 Markdown 安全由 `details` 用例回归 |
| `/search` | `search` | 五宽度、浅深 | 搜索上下文、详情入口由 `search` 用例回归 |
| 资产详情内部视图 | `detail` | 五宽度、浅深 | 编辑与返回路径由 `maintenance`、`details` 用例回归 |
| 资产表单内部弹层 | `form-error` | 1440、390，浅深 | 新增 JSON 错误与输入保留断言由 `maintenance`、`ui-refresh` 覆盖 |
| 资产加载/错误状态 | `assets-loading`、`assets-error` | 1440、390，浅深 | 浏览器模拟延迟/500，分别断言进度与重试反馈 |
| 首次引导内部弹层 | — | 未截图 | 空库进入流程由 `maintenance` 用例覆盖；浅深专项未验证 |

`Browse.tsx` 在 `controller.ts` 中只有卸载调用，没有挂载分支，因此不增加虚构路由。截图只存储虚构示例数据；更多真实内容状态如权限不足、离线、软键盘和读屏仍需独立验收。

## 第二阶段验证记录

- `python3 docs/frontend-standard/validate.py`、`npm run typecheck --prefix web`、`npm run build --prefix web`、`go test ./...`：通过。
- Chromium Playwright：`node --test --test-concurrency=1` 执行除 HTTP/TLS 外 31 个浏览器用例，31/31 通过；HTTP/TLS 用例在 `rollbook-playwright-go:latest` 配合 `GOFLAGS=-buildvcs=false` 单独执行通过。截图矩阵用例使用 `ROLLBOOK_SCREENSHOT_DIR` 指向 `web/screenshots/v2ri-71`。
- 未发布新 tag；本变更为待合并 PR 的前端改造，不涉及数据库迁移、API 和部署。发布前按 `docs/releasing.md` 核对合并提交、Release 正文及备份/回退说明。
