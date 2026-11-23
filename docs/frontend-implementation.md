# 前端规范 v1.0.0 接入与阶段边界

## 固定来源与优先级

项目规则优先；视觉、组件和验收以 `frontend-standard/FRONTEND-SPEC.md` 为解释来源，以同目录 `tokens.json` / `tokens.css` 为机器值。固定 tag `v1.0.0`，commit `d20328e1f79c3aa3243fc48f25a581136d86719d`。`AGENTS.md` 只引用这一份规范。旧 `base.css` 绿色和 `style.css` 蓝/锌双主题已映射到语义变量；`--background`、`--foreground`、`--card`、`--input` 等旧别名只作兼容，项目不再维护第二套颜色值。详情见 `frontend-standard/SOURCE.md`。

## 实际入口与页面模板

| 直达 URL | 实际组件 | 模板 | 阶段 |
|---|---|---|---|
| `/` | `Overview.tsx` | 总览 | 壳已接入；内容第二阶段 |
| `/assets`，`/assets/services`，`/assets/sites`，`/assets/hosts`，`/assets/networks` | `Assets.tsx`、`HostTree.tsx` | 列表/树 | 第二阶段，未验证 |
| `/topology` | `Topology.tsx`、`Flow.tsx`、`topology-model.ts` | 拓扑 | 第二阶段，未验证 |
| `/pending`，`?tab=unverified|outdated` | `Pending.tsx` | 列表 | 第二阶段，未验证 |
| `/agent` | `Keys.tsx`、`AgentGuide.tsx` | 列表/表单 | 第二阶段，未验证 |
| `/settings`，`/settings/backup` | `Settings.tsx`、`Backup.tsx` | 设置/表单 | 第二阶段，未验证 |
| `/changes`，`/contexts` | `Audit.tsx` | 列表 | 第二阶段，未验证 |
| `/search` | `Search.tsx` | 列表 | 第二阶段，未验证 |

对象详情 `AssetDetail.tsx`、新建/编辑 `AssetForm.tsx` 与首次引导 `FirstRun.tsx` 是内部视图或弹层，不是独立 URL。`Browse.tsx` 尚有组件，但 `controller.ts` 当前 switch 未挂载，不新增 URL。

## 组件复用与差异

| 现有入口 | 本次映射 | 待第二阶段核对 |
|---|---|---|
| `web/src/components/ui/button.tsx` | 四变体、36/32/44 尺寸、禁用/加载状态与语义颜色 | 各业务按钮是否复用 |
| `web/src/components/ui/catalog.tsx` | Tabs 选中语义、Table 表头/行密度、骨架加载语义 | 排序、分页、表格内容状态 |
| 原生 `input/select/textarea`、Radix Dialog | `control-border`、焦点/错误轮廓；登录/改密码保留原行为，抽屉提供关闭 | 业务字段错误、弹层草稿、批量操作 |
| `main.tsx` 与 `controller.ts` | 固定侧栏/顶栏、主内容滚动、当前导航与跳转链接、系统主题 | 各业务页长内容与表单底栏 |

旧实现的 body 与 `.main` 均可能滚动、系统暗色依赖单独覆盖、桌面与窄屏有不同侧栏逻辑、顶部搜索与侧栏颜色残留旧值。本阶段改为 `html[data-theme=light|dark]` 解析后的单一主题状态；`rollbook.theme=system` 仍保留偏好并响应系统变化。旧 `.crumb` 同级位置标签隐藏，详情页有价值的返回路径保留。既有 React/TypeScript/shadcn/ui/Tailwind、业务 API 与权限模型不变。

## 品牌参数与验收界限

产品名称沿用 Rollbook / 清册，侧栏 `R` 和 `web/public/icon.svg` 保留。正式浅/深 logo、清晰区比例、最小尺寸、完整 favicon/PWA 尺寸及品牌覆盖批准记录尚未提供，记为可配置缺口，不伪造资产。本阶段仅验收真实登录页和总览/壳；表中业务内容、筛选/创建/编辑/拓扑交互、200%/400% 缩放和读屏专项仍标为**未验证**，由第二阶段逐项补齐，不宣称全站通过。

## 第一阶段验证记录

- `python3 docs/frontend-standard/validate.py`：166/166 个对比组合、272/272 个结构检查通过。
- `npm run typecheck --prefix web`、`npm run build --prefix web`：通过；仓库没有独立 lint 脚本。
- Chromium Playwright：31 个现有/新增浏览器用例通过。其中 30 个在 `mcr.microsoft.com/playwright:v1.65.0-noble` 执行，依赖 Go 的 HTTP/TLS 用例在仓库已有 `rollbook-playwright-go:latest` 环境单独执行通过。
- `docs/frontend-evidence/` 有真实登录页与总览/壳的 1440、390、320 CSS px 同路由同数据浅深配对截图。键盘焦点、滚动和抽屉关闭有浏览器断言。业务视图、200%/400% 缩放、读屏等为**未验证**，第二阶段覆盖。
