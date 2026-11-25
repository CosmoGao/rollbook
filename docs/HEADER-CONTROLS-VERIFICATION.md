# HEADER 控件边框修复验证

本次仅改 `web/src/style.css` 中的 HEADER 桌面搜索、窄屏搜索和主题控件。沿用现有 `control-border`、`control-border-hover`、`focus`、`radius-control` token；搜索和主题的 DOM、快捷键、提交与持久化逻辑未改。通用输入与全站焦点规则保持原样。

本记录与截图来自已合并的 HEADER 修复 PR #67，并作为计划中 `v0.11.0` 的控件证据。后续 PR #68 增加认证设置导航及其样式，未修改这些 HEADER 控件的选择器与样式规则；正式 tag 尚待发布流程生成。截图使用模拟空库与演示会话，不包含真实账号或密钥。

## 浏览器证据

使用 Playwright 1.65 的真实 Chromium；模拟已登录会话，语言为中文，初始路由 `/`，搜索提交后路由 `/search`。截图在 `docs/header-controls-screenshots/`；文件名包含主题、视口 CSS px、准确选择器和状态：

| 选择器 | 路由 | 视口与主题 | 截图状态 |
|---|---|---|---|
| `form.top-search` | `/` | 1280×800；浅、深 | `idle`、`hover`、`focus`、`input` |
| `form.mobile-search` | `/` | 390×800、320×800；浅、深 | `idle`、`hover`、`focus`、`input`、`submit-focus` |
| `header.topbar label.theme-control`（内含 `select` 与图标） | `/search` | 1280×800、390×800、320×800；浅、深 | `idle`、`hover`、`focus`、`open` |

命名示例：`light-1280-formtop-search-idle.png`、`dark-320-headertopbarlabeltheme-control-focus.png`。修复前对照为 `before-light-1280-formtop-search-idle.png` 和 `before-light-1280-headertopbarlabeltheme-control-focus.png`。`open` 状态由键盘 Space 打开原生 select；Chromium 无头截图只记录页面外观，操作系统绘制的原生选项弹窗未进入截图。

浏览器断言逐状态检查外层 1px 边框、8px 圆角、内部 0px 边框与 0px 独立焦点轮廓、焦点色、视口内几何和无页面水平溢出。还检查 Ctrl/⌘+K 聚焦搜索页输入、HEADER 搜索提交、主题切换和刷新后的持久化；强制色模式下两个外框都有贴合边界的系统焦点轮廓。`320px` 视口检验了缩放后可能出现的有效窄宽度；未在浏览器 UI 中独立执行 200%/400% 缩放，也未截到系统原生 select 弹窗。

## 本地验证

```sh
npm run typecheck --prefix web
npm run build --prefix web
docker run --rm -u 1000:1000 -v "$PWD":/work -w /work/web mcr.microsoft.com/playwright:v1.65.0-noble node --test header-controls.browser.test.cjs search-control.browser.test.cjs
docker run --rm -u 1000:1000 -v "$PWD":/work -w /work rollbook-playwright-go:latest sh -lc 'go test ./...'
docker run --rm -u 1000:1000 -v "$PWD":/work -w /work rollbook-playwright-go:latest sh -lc 'go vet ./...'
```

结果：类型检查、构建、两项浏览器测试、Go 测试和 `go vet` 通过。既有搜索页 `.search-box` 测试单独通过，未作为 HEADER 控件的验收依据。规范全站清单中的其他页面、读屏与完整缩放组合未验证；本次变更没有扩大到这些页面。
