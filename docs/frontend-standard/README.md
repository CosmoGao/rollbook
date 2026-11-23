# 前端与 VI 规范

发布版本：**v1.0.0** · 规范内容版本：**1.0**

2026-11-23（Asia/Shanghai，UTC+8）。这是当前建议基线：低饱和石板蓝、完整浅深语义令牌、组件与页面规则、VI 与验收要求。它可供不同开发 AI 读取，不绑定特定框架。

## 固定版本入口

- 仓库：https://github.com/CosmoGao/frontend-standard
- 发布与整包下载：https://github.com/CosmoGao/frontend-standard/releases/tag/v1.0.0
- 主规范原始 Markdown：https://raw.githubusercontent.com/CosmoGao/frontend-standard/v1.0.0/FRONTEND-SPEC.md
- 主规范 GitHub 文件页：https://github.com/CosmoGao/frontend-standard/blob/v1.0.0/FRONTEND-SPEC.md
- 直接下载规范 ZIP：https://github.com/CosmoGao/frontend-standard/releases/download/v1.0.0/frontend-standard-v1.zip

外部 agent 引用 SHOULD 固定 `v1.0.0`，不要无意跟随 `main` 的后续变化。公开网页不保证每个 AI 工具能自动读取；raw 不可访问时尝试同版本 GitHub 文件页，或由有网络能力的环境下载到本地，再读取完整文件。

## 最快使用

**复制给其他 agent：**

> 请先完整读取 https://raw.githubusercontent.com/CosmoGao/frontend-standard/v1.0.0/FRONTEND-SPEC.md ，再按其中的前端与 VI 规范实施本次授权任务；复用现有组件与语义令牌，保留产品名称/logo，遵循更高优先级项目指令，完成后逐项验收。若无法读取该链接，请明确说明并改读同版本 GitHub 文件页或请求提供本地文件，不得声称已读取。

**只给 AI 一个文件：**下载或上传 `FRONTEND-SPEC.md`。该文件自包含全部关键颜色、尺寸与规则。

**本地 vendor：**下载 Release 的 ZIP，解压后将其中 `frontend-standard-v1/` 的内容放入业务仓库 `docs/frontend-standard/`。把 `AI-INSTRUCTIONS.md` 的引用片段合并追加进已有 `AGENTS.md` / `CLAUDE.md` / 开发工具规则文件，**不能覆盖原文件**，不能凌驾既有高优先级指令。仅加入文档不自动修改业务页面，实施范围以当次任务授权为准。

**下载示例：**在自己的项目目录执行以下命令；不会自动改项目规则或覆盖现有文件。

```bash
curl -fL https://github.com/CosmoGao/frontend-standard/releases/download/v1.0.0/frontend-standard-v1.zip -o frontend-standard-v1.zip
unzip -t frontend-standard-v1.zip
# 确认目标目录后再解压，将规范目录内容放到 docs/frontend-standard/。
```

安装/绑定变量示例（路径按业务项目实际目录调整）：

```css
/* 相对路径以项目目录为准；无需安装额外运行库。 */
@import "./docs/frontend-standard/tokens.css";
body {
  background:var(--color-bg); color:var(--color-text);
  font-family:var(--font-sans);
  font-size:var(--font-size-body); line-height:var(--line-height-body);
}
/* 组件也必须绑定尺寸、状态和边界；此处只是最小接入。 */
```

```js
// 应用唯一主题状态驱动 html；用户选择可持久化，系统仅作初始回退。
document.documentElement.dataset.theme = 'dark'; // 'light' | 'dark'
```

变量文件不施加reset、不实现组件；主题必须设在`html`，不依赖隐式`.dark`。颜色是完整HEX，直接`var()`，不要再套`hsl()`。窄屏字号/44px目标需应用按规范绑定；不能只导入变量就宣称完成。

## 目录

| 文件 | 用途 |
|---|---|
| FRONTEND-SPEC.md | 自包含主规范，15章，含VI、全部token表和可检验规则 |
| AI-INSTRUCTIONS.md | 可复制提示、跨AI工具接入片段与交付格式 |
| tokens.json | 自定义机器结构的共享值和浅深色 |
| tokens.css | 可直接import的CSS变量与data-theme覆盖 |
| project-brand.json | 保留产品独立标识的空配置示例，不是定稿品牌 |
| CHECKLIST.md | 实施前后/宽窄屏/深色/键盘/截图验收 |
| validate.py | Python 3标准库校验，不启动浏览器 |
| VALIDATION.md | 颜色实算摘要与范围说明 |
| validation-results.json | 每个合法颜色组合实数与文件一致性断言 |
| MANIFEST.sha256 | 打包时的文件SHA-256清单（不含自身） |
| AGENTS.md | 本规范仓库自身的agent维护要求 |
| CHANGELOG.md | 公开发布版本记录 |

## JSON 结构与映射

`tokens.json` 是本包 **project-semantic-tokens-v1 自定义结构**，不是 DTCG 文件，也不是 shadcn 配置，可写适配器消费：

```text
meta: {name, version, date, timezone, format, description}
shared: { tokenName: CSS字符串 }
themes:
  light: { semanticColorName: "#RRGGBB" 或遮罩 "#RRGGBBAA" }
  dark:  { 同一组semanticColorName: 对应HEX }
```

- `shared.control-h = "36px"` → CSS `--control-h:36px`；数字型字重/层级也是字符串，以CSS可直接消费为准。
- `themes.light["text-muted"]` → `--color-text-muted`，所有颜色键均加`color-`前缀。
- 深浅键必须完全相同。shared不分主题；尺寸中px可由项目适配为等值rem，保留浏览器缩放。
- 状态`X`为实心填色，`X-fg`为其上文字，`X-subtle`/`X-on-subtle`为浅底容器组合；`X-text`为中性背景上的说明文字。
- `border`只用于装饰线，`control-border`及hover/active用于必要控件轮廓。禁用值只给真正禁用控件。
- shadow-overlay带透明度；overlay为HEX8，未泛化计算任意底图叠加。图表颜色不保证彼此之间的对比度。
- 主规范第14章给出shadcn语义别名映射；已有HSL通道式模板必须同时调整消费方式，不能把完整HEX误当通道。

## 校验与证据

```bash
python3 validate.py  # 在本规范仓库根目录运行；vendor时先进入docs/frontend-standard
```

校验重新生成`VALIDATION.md`和`validation-results.json`，检查166个不透明合法颜色组合、272个文件/基线断言。精确结果见报告；不检测DOM、真实渲染或整个应用的可访问性。复制到业务项目后仍要逐项执行`CHECKLIST.md`；改token或品牌覆盖后重新校验，新增组合也要加入测试。

[修订] 装饰border在表面上不足非文本3:1，已新增浅色`#64748B`、深色`#94A3B8`的control-border，保留原装饰线；hover/active/disabled显式定义，placeholder不降透明度。浅色active表面使用`#EDF2F7`，避免链接/状态说明在更深背景上不足4.5:1。

## 品牌可配置项

本包没有已确认的新品牌名/标志，不输出伪造正式logo；`project-brand.json`中的null由各项目现有产品信息补齐。字标、横向组合、安全区、最小尺寸、深浅底、图标/插图/图片、应用图标、favicon要求见主规范第2章。安全区0.25S等是新建议值，可用正式品牌指南覆盖，但必须满足显示与无障碍门槛。产品名、logo资产路径、项目字段/栏目为可配置项，不阻塞采用本规范。主题覆盖要批准、补齐全部关联状态并复验。

## 来源与决策状态

- [沿用] 作者历史风格选型中的 B 石板蓝与密度/布局原则，提炼为跨项目基线。
- [参考] 既有清册静态示例仅用于形态与模板讨论，不绑定其他项目的业务字段。
- [历史调研] 2026-11-21（中国标准时间，UTC+8）的内部设计系统调研提供语义分层思路；本发布版未重新核验第三方包最新 API 或许可证。
- [细化/修订] 本规范新增 VI 建议、完整组件状态、响应式、工程接入与 control-border 约束，不冒充已有项目实测结果，也不宣称某开源库的官方现成主题。内部历史材料不是本仓库运行或读取依赖。

## 使用与许可状态

本仓库供作者项目及其授权 agent 引用；**公开可见不等于授予额外开源许可，许可证待确定**。本次发布没有选择 MIT、CC 或其他许可证。使用第三方库、图标、字体或图片时仍需分别遵守其适用条款。

复制规范不会自动授权发送、部署、改库或更换品牌；开发仍按当次项目任务与权限执行。
