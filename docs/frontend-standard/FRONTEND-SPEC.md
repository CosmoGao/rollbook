# 前端与 VI 规范

版本：1.0 · 当前建议基线

日期：2026-11-23（Asia/Shanghai，UTC+8）

风格方向：低饱和石板蓝 · 文档语言：简体中文

## 1. 执行约定与决策来源

本文件可单独交给开发 AI 使用。完整包的 JSON/CSS 是同一规范的机器表达；只有本文件时，AI MUST 按下表建立语义变量并逐项验收，不得把缺少附件当成另造设计的理由。

- **MUST（必须）**：验收门槛，不满足不得宣称已完成。
- **SHOULD（应当）**：默认执行；偏离时记录具体原因、影响和批准依据。
- **MAY（可以）**：项目可选，不得破坏 MUST。
- 与既有项目高优先级指令冲突时，MUST 说明冲突并遵循其优先级；不得借本文件覆盖安全、业务或仓库规则。
- [沿用]：来自已选 B 石板蓝及风格页的颜色/密度/布局原则。
- [细化]：本版为可执行性新增的组件、响应式、VI 和工程规则，是建议规范，不是既有实现事实。
- [修订]：本版新增可访问性约束；静态 demo 不作为该项豁免依据。
- [可配置]：由各项目提供产品参数；未提供时保留当前产品名称与标识，缺失的标识使用纯文字，不生成正式 logo。

历史决策依据（非公开资料，不作为外部获取依赖）：
1. 历史风格选型采用 B 石板蓝，并确定中等密度、导航去重和固定应用壳原则。
2. 既有清册静态示例用于校验形态和页面模板；本规范将其提炼为跨项目规则，不绑定示例业务结构。
3. 2026-11-21 的内部设计系统调研提供语义令牌分层思路；本发布版未重新核验第三方包的最新 API 或许可证。

发布版本：`v1.0.0`；规范内容版本保持 `1.0`。仓库公开链接与固定版本入口见 README。以上是历史决策说明，不声称存在可公开访问的原始来源。

## 2. 视觉原则与 VI 总体结构

[沿用] 内容优先、冷静克制、中等信息密度。背景与表面使用蓝灰中性色；蓝色集中用于主操作、当前项和链接。MUST 以排版、留白和对齐建立层级，而不是大按钮、满屏彩色卡片或多层厚重阴影。

[细化] 跨项目共用的是语义色、字体体系、控件形态、间距、状态语言和布局骨架。产品名称、产品图形标、字标、专属图片和应用图标是独立插槽。MUST NOT 自动给项目改名或让不同产品共用一个虚构 logo。清册仅是列表/详情示例，不决定其他项目的栏目、字段和业务结构。

### 2.1 产品标识、字标与组合 [可配置]

- 产品配置 SHOULD 包含 `nameZh`、`nameEn`、`shortName`、`logoLight`、`logoDark`、`appIcon`、`favicon`、`logoClearSpaceRatio`、`logoMinSizePx`、主题覆盖及版本。名称与资源缺失使用 null，不猜测。
- 没有正式图形标时 MUST 只显示产品现有名称；不得把占位图标当作定稿。
- 字标默认使用本规范 sans，中文 16px/24px、600；英文与中文同排时 SHOULD 同基线，间隔 8px，英文 14px/22px、500。空间不足可仅保留主名称；不得强行塞两行挤压导航。
- 新标志的**建议约束**：以图形标边长 S 为基准，四周安全区至少 0.25S；独立图形标最小 20px，带字标横向组合整体高度至少 24px；侧栏建议图形标 32px。正式品牌文件给出更严格值时采用正式值。
- 浅底用深色/彩色批准版，深底用浅色/反白批准版；MUST 保持原始比例、字距和颜色，不描边伪造反白、不拉伸、不加阴影、不裁掉安全区。文字字标仍 MUST 通过相应文字对比度。
- 图形标使用已批准 SVG（含 viewBox，无脚本和外链），位图提供透明 PNG 1×/2×。有文字产品名在旁时图形为装饰并设空替代文本；独立链接标识 MUST 有产品名可访问名称。

### 2.2 应用图标与 favicon [细化/可配置]

品牌资产 SHOULD 交付 `logo-light.svg`、`logo-dark.svg`、`mark.svg`、`favicon.svg`、16/32px PNG favicon、180px Apple touch icon、192/512px manifest icon；需要 PWA 时提供独立 maskable 版本并在中心 80% 直径安全圆内保留必要图形。MUST 在 16/32px 真正缩小检查，图标中不放无法辨认的长产品名。资产清单和授权状态记录在项目品牌配置旁；未交付不阻塞通用组件落地，采用现有品牌资产或文字入口。

### 2.3 图标、插图、图片

- MUST 每个项目采用同一套线性图标；默认 24×24 网格、1.75px 描边、round 端点/连接；正文操作显示 16px，导航 20px，空态 24–48px。已有图标库为 2px 时可整套保持 2px，但不得混搭线宽风格。
- MUST 用 `currentColor`；必要图标对相邻背景至少 3:1。状态必须同时有文字/图形，不靠红绿区分。仅装饰图标 `aria-hidden=true`。
- 插图 SHOULD 用扁平、少量蓝灰面与单一线性轮廓，不做大面积渐变、玻璃拟态或高饱和 3D。空状态插图占内容高度 SHOULD 不超过 160px，不能挤走操作。
- 图片 SHOULD 保持真实内容、统一裁切比例、标明信息性替代文本；装饰图用空 alt。文字不要烧进图片；不得用库存图暗示真实客户/设备。资源必须记录来源及使用权。

### 2.4 品牌色、功能色与图表

- MUST 将品牌蓝与 success/warning/danger/info 独立映射，不用蓝色同时表示「已成功」。信息色可同属蓝系，但只在语义信息容器使用并配图标/标签。
- 图表分类按 `chart-1…6` 固定顺序；同实体在跨图和跨主题中保持序号。MUST 配直接标签、图例或纹理，折线另用点形/虚线区分；超过 6 类时分组或提供可切换表格。
- 图表 token 针对 bg/surface 的线条/必要图形对比度校验 ≥3；不保证任意两种颜色之间 ≥3。相邻实心色块 MUST 加表面色分隔或独立描边并人工复验；禁止只用色相作为类别唯一线索。
- 字体、状态色、信息架构、控件密度、focus 和 control-border 不是品牌随意覆盖项。主色/选中/链接可按产品主题提案覆盖，但 MUST 同时补齐浅深、hover/active/fg 及重新计算对比度；单个页面不得自行换色。

### 2.5 文字语气与控件气质

控件为 8px 圆角矩形，非默认胶囊；标签类徽标 6px，内容卡片 12px。按钮文案 SHOULD 用短动词宾语，例如「新建记录」「保存修改」「导出清单」。确认框 MUST 说清对象和后果；错误给原因和下一步，不仅显示「出错了」。语气简洁、明确，不夸张庆祝，不用晦涩代码代替用户可读说明。日期、数字、单位遵循项目地区设置；破折号表示无值且与零值区分。

## 3. 颜色令牌 [沿用 + 修订]

所有颜色 MUST 通过语义 token 引用；组件不得散落十六进制。浅深同名一一对应。CSS 颜色是**完整 sRGB HEX**（遮罩为 HEX8），用 `var(--color-text)`，不能写成 `hsl(var(--color-text))`。JSON 使用 `themes.light/dark`。

[修订] `border` 仍为装饰分割线；必要的输入框、次按钮、checkbox 等边界 MUST 使用 `control-border`。深色主按钮/危险按钮在暗表面边界不足时同样使用 `control-border` 外轮廓，不改变实心色。placeholder MUST 用不透明 text-muted；demo 的额外 opacity 不沿用。状态说明用 `*-text`，不要把深色实心按钮填色直接当成正文。

| 颜色语义（CSS 前缀 `--color-`） | 浅色 | 深色 | 用途 |
|---|---|---|---|
| `bg` | `#F8FAFC` | `#0F172A` | 页面底色 |
| `surface` | `#FFFFFF` | `#1E293B` | 内容/弹层表面 |
| `border` | `#E2E8F0` | `#334155` | 装饰分割线，不用于必要控件轮廓 |
| `text` | `#0F172A` | `#F1F5F9` | 正文/必要图标 |
| `text-muted` | `#475569` | `#94A3B8` | 次要文字/占位文字，不透明 |
| `primary` | `#2563EB` | `#2563EB` | 主按钮实心 |
| `primary-fg` | `#FFFFFF` | `#FFFFFF` | 主按钮文字 |
| `selected-bg` | `#EFF6FF` | `#1E3A8A` | 当前项/选中背景 |
| `selected-fg` | `#1D4ED8` | `#BFDBFE` | 选中文字/图标 |
| `focus` | `#2563EB` | `#60A5FA` | 可见焦点环 |
| `link` | `#2563EB` | `#60A5FA` | 链接/文字按钮 |
| `link-hover` | `#1D4ED8` | `#93C5FD` | 见状态与组件规则 |
| `link-active` | `#1E40AF` | `#BFDBFE` | 见状态与组件规则 |
| `control-border` | `#64748B` | `#94A3B8` | 必要控件轮廓；实心按钮外轮廓 |
| `control-border-hover` | `#475569` | `#CBD5E1` | 见状态与组件规则 |
| `control-border-active` | `#334155` | `#F1F5F9` | 见状态与组件规则 |
| `surface-hover` | `#F1F5F9` | `#253247` | 次按钮/行/导航 hover |
| `surface-active` | `#EDF2F7` | `#29364B` | 次按钮/行/导航 active |
| `primary-hover` | `#1D4ED8` | `#1D4ED8` | 见状态与组件规则 |
| `primary-active` | `#1E40AF` | `#1E40AF` | 见状态与组件规则 |
| `disabled-bg` | `#F1F5F9` | `#253247` | 见状态与组件规则 |
| `disabled-fg` | `#64748B` | `#64748B` | 见状态与组件规则 |
| `disabled-border` | `#CBD5E1` | `#334155` | 见状态与组件规则 |
| `overlay` | `#0F172A66` | `#020617B3` | 弹层遮罩，不能承载正文 |
| `success` | `#157A3B` | `#3D9B63` | 见状态与组件规则 |
| `success-fg` | `#FFFFFF` | `#08150D` | 见状态与组件规则 |
| `success-subtle` | `#E8F6ED` | `#143021` | 见状态与组件规则 |
| `success-on-subtle` | `#14532D` | `#A7E3BD` | 见状态与组件规则 |
| `success-text` | `#157A3B` | `#A7E3BD` | 见状态与组件规则 |
| `warning` | `#9A4A00` | `#E0A14A` | 见状态与组件规则 |
| `warning-fg` | `#FFFFFF` | `#1A1206` | 见状态与组件规则 |
| `warning-subtle` | `#FFF6E8` | `#3A2A12` | 见状态与组件规则 |
| `warning-on-subtle` | `#7A3B00` | `#F3D19A` | 见状态与组件规则 |
| `warning-text` | `#9A4A00` | `#F3D19A` | 见状态与组件规则 |
| `danger` | `#C01515` | `#E06A6A` | 见状态与组件规则 |
| `danger-fg` | `#FFFFFF` | `#1A0B0B` | 见状态与组件规则 |
| `danger-subtle` | `#FDECEC` | `#3A1818` | 见状态与组件规则 |
| `danger-on-subtle` | `#8F1212` | `#F5B4B4` | 见状态与组件规则 |
| `danger-text` | `#C01515` | `#F5B4B4` | 见状态与组件规则 |
| `danger-hover` | `#A51212` | `#E98383` | 见状态与组件规则 |
| `danger-active` | `#8F1212` | `#F0A0A0` | 见状态与组件规则 |
| `info` | `#0369A1` | `#5BA3C9` | 见状态与组件规则 |
| `info-fg` | `#FFFFFF` | `#071218` | 见状态与组件规则 |
| `info-subtle` | `#E8F4FB` | `#163042` | 见状态与组件规则 |
| `info-on-subtle` | `#0C4A6E` | `#B9DDF0` | 见状态与组件规则 |
| `info-text` | `#0369A1` | `#B9DDF0` | 见状态与组件规则 |
| `chart-1` | `#2563EB` | `#60A5FA` | 见状态与组件规则 |
| `chart-2` | `#0F766E` | `#2DD4BF` | 见状态与组件规则 |
| `chart-3` | `#7E22CE` | `#C084FC` | 见状态与组件规则 |
| `chart-4` | `#9A4A00` | `#E0A14A` | 见状态与组件规则 |
| `chart-5` | `#BE185D` | `#F472B6` | 见状态与组件规则 |
| `chart-6` | `#475569` | `#CBD5E1` | 见状态与组件规则 |


### 3.1 合法组合与交互状态

- 正文、次要文字和必要图标：`text`/`text-muted` 对 `bg`/`surface`/`surface-hover`/`surface-active`；选中区域内统一 `selected-fg` 对 `selected-bg`。
- 实心主按钮：`primary-fg` 对 `primary` / `primary-hover` / `primary-active`；实心危险按钮：`danger-fg` 对 `danger` / `danger-hover` / `danger-active`。
- 状态徽标：`X-fg` 对 `X`，或 `X-on-subtle` 对 `X-subtle`。独立状态说明与必要状态图标：`X-text` 对中性表面。
- 文字链接：`link` / `link-hover` / `link-active` 对中性表面，正文内默认有下划线，不能只凭颜色区分。
- 次按钮/ghost：text 对 surface → surface-hover → surface-active。ghost 默认可透明，但仅放在已校验的中性表面；选中区域不要叠此背景，使用 selected-fg 保持语义。
- 必要控件轮廓：control-border → control-border-hover → control-border-active；错误使用 danger-text；焦点使用 focus，与控件留 focus-offset 的背景色间隙。焦点环放在 bg/surface 邻接区域；放到彩色区域必须增加中性隔离并重新测量。
- disabled-bg/fg/border 仅用于真正不可操作项，禁用控件是对比度要求的例外；MUST 不用整组件 opacity，也不能借禁用样式表示普通只读信息。只读信息沿用正常文字对比度。
- 不在表内的新组合 MUST 实算后写入项目扩展 token；不能因为单个颜色「通过」就推定任何背景都通过。

## 4. 排版、间距、尺寸与层级

[沿用] 桌面正文 14/22px；窄屏正文/输入 16/24px；表格不要为增加行数缩正文。12/18px 只用于补充注释、非关键时间戳，不得用于核心值、按钮、字段标签。用户缩放时不得裁字。

标题：区块 16/24、分区 20/28、页标题 24/32，600 字重。正文 400，字段标签/按钮 500。等宽字体用于 IP、编号、代码；长编号允许复制，tabular-nums 用于需要纵向比较的数字。MUST 一个页面有一个 h1，标题级别不跳跃。

共享值如下，CSS 尺寸是基于默认根字号的 px 参考，应用 SHOULD 换成等值 rem 并保留浏览器缩放；MUST 不锁用户字号：

| 共享 token（CSS 前缀 `--`） | 值 |
|---|---|
| `font-sans` | `system-ui, "PingFang SC", "Noto Sans SC", "Microsoft YaHei", sans-serif` |
| `font-mono` | `ui-monospace, "IBM Plex Mono", "Noto Sans Mono CJK SC", "SFMono-Regular", Menlo, Consolas, monospace` |
| `font-size-body` | `14px` |
| `line-height-body` | `22px` |
| `font-size-mobile` | `16px` |
| `line-height-mobile` | `24px` |
| `font-size-small` | `12px` |
| `line-height-small` | `18px` |
| `font-size-title-sm` | `16px` |
| `line-height-title-sm` | `24px` |
| `font-size-title-md` | `20px` |
| `line-height-title-md` | `28px` |
| `font-size-title-lg` | `24px` |
| `line-height-title-lg` | `32px` |
| `font-weight-body` | `400` |
| `font-weight-label` | `500` |
| `font-weight-heading` | `600` |
| `space-1` | `4px` |
| `space-2` | `8px` |
| `space-3` | `12px` |
| `space-4` | `16px` |
| `space-5` | `24px` |
| `space-6` | `32px` |
| `space-7` | `48px` |
| `radius-control` | `8px` |
| `radius-card` | `12px` |
| `radius-badge` | `6px` |
| `border-width` | `1px` |
| `control-h` | `36px` |
| `control-h-compact` | `32px` |
| `control-h-touch` | `44px` |
| `table-row-h` | `40px` |
| `sidebar-w` | `232px` |
| `header-h` | `56px` |
| `content-max` | `1440px` |
| `form-max` | `800px` |
| `dialog-max` | `560px` |
| `drawer-w` | `480px` |
| `focus-width` | `2px` |
| `focus-offset` | `2px` |
| `icon-sm` | `16px` |
| `icon-md` | `20px` |
| `icon-lg` | `24px` |
| `icon-stroke` | `1.75` |
| `duration-fast` | `120ms` |
| `duration-normal` | `180ms` |
| `shadow-card` | `none` |
| `shadow-overlay` | `0 8px 24px #02061726` |
| `z-header` | `20` |
| `z-popover` | `40` |
| `z-overlay` | `60` |
| `z-toast` | `80` |


默认间距取 4/8 序列：4、8、12、16、24、32、48px。表单 label 到输入 4px，字段间 16px，分组间 24px，页面区块间 24px；桌面内容左右 32px，上 24px；窄屏四周 16px。图标与文案 8px，按钮横向 padding 12px（触控 16px）。内容宽度允许到 1440px，超过居中；阅读和表单区建议 800px 内，数据表可占满。

边框默认 1px；卡片不加重阴影，弹层可用 shadow-overlay。不得通过不断嵌套卡片制造层级。徽标高度 22px，只是信息显示，不充当小触控按钮。动画默认 120/180ms，仅做状态过渡；MUST 尊重 prefers-reduced-motion，关闭不必要移动/骨架闪烁，保留非动画进度说明。

## 5. 应用壳、导航与滚动

- MUST 固定应用壳在视口中（建议 100dvh，保留 100vh 回退），侧栏 232px、顶栏存在时 56px，主列 min-width:0、min-height:0。
- MUST 只由主内容拥有页面纵向滚动；侧栏不跟着主内容上下移动。导航项过多时允许侧栏**自身导航区**独立滚动，品牌和底部操作不动。
- MUST 只保留一个当前位置表达信源。侧栏当前项配页标题可以；同层级不要再加一行相同面包屑。只有真正多层路径且有返回价值时才使用面包屑，并移除重复位置标签。
- 当前导航 MUST 有 aria-current、背景与字重变化，不能只变色；导航项桌面 36px，触控 44px。折叠侧栏必须保留可访问名称。
- 页面只放一个主要操作组；组内最多一个实心主按钮。表单的主要提交操作在底栏，页头不重复一个同样强的提交按钮。表格批量操作为局部上下文组，与页级主要操作分层，避免同时抢主色。
- SHOULD 采用 grid/flex 固定轨道，而不是 body 和 main 同时滚动。表格横向滚动只在表格容器；冻结表头只相对该容器，不盖页头。

## 6. 按钮与交互目标

| 类型 | 默认 / hover / active | 边界与用法 |
|---|---|---|
| 主按钮 | primary / primary-hover / primary-active，primary-fg | 页级唯一主要组内的首要提交/创建；control-border 外轮廓 |
| 次按钮 | surface / surface-hover / surface-active，text | 取消、导出等次操作，control-border 各状态 |
| ghost | 透明 / surface-hover / surface-active，text | 行操作、工具栏；不承担唯一且难找的主操作 |
| 文字按钮/链接 | link / link-hover / link-active | 低强调操作；链接导航用 a，动作使用 button |
| 危险按钮 | danger / danger-hover / danger-active，danger-fg | 破坏性确认；正文页优先次级危险文字，确认弹层才突出实心危险 |

所有类型 MUST 有 keyboard focus-visible（2px focus，offset 2px），disabled、loading 状态。loading 保持原宽度，保留动作文字如「保存中…」，aria-busy=true，防重复提交；不把每次异步都改成空白 spinner。aria-disabled 的自定义元素 MUST 同时阻止鼠标和键盘激活，原生 button 优先用 disabled。disabled 原因应可在附近文字获得，不只在不可聚焦 tooltip 中。

尺寸：默认 36px、紧凑 32px、触控 44px；按钮字号均至少 14px，窄屏随正文到 16px。图标按钮可见正方形 36/32px，触控点击区域 MUST ≥44×44px；图标尺寸不是点击区域尺寸。一般桌面目标至少 24×24px，行内紧凑动作间距至少 8px；触屏及 coarse pointer 的所有常用交互区域 SHOULD 44×44px，主要操作 MUST ≥44px。不得把桌面全部按钮拉到 48/56px，也不得用 32px 覆盖触屏场景。

仅图标按钮 MUST 有 aria-label，tooltip 是补充不是名称。按钮不得嵌套链接，整行可点击时不得吞掉内部链接/复选框的键盘操作。按下态通过颜色及必要时边界变化表达，不做跳动缩放。

## 7. 表单、校验与固定底栏

- MUST 每个输入有真实关联的 label（for/id 或嵌套），placeholder 不代替标签；必填用可读「必填」说明和程序化 required，不能仅红星。
- 输入默认高 36px、compact 32px；触屏常用输入 ≥44px、字号 16px；textarea 最小 96px，允许纵向扩展。checkbox/radio 可见图形建议 16–20px，但标签连同图形构成完整点击区。
- 默认 surface/text/control-border；hover、active 边界同令牌；focus 可见。只读保留值与 label 对比度；disabled 不提交的语义必须符合实际业务。
- 一行一字段为默认；桌面确有关联的短字段可两列，列间 16px。标签在上方，帮助说明在下方。长中文不固定高度截断，数值带单位说明。
- 校验 SHOULD 在失焦或提交后触发，输入中纠正已有错误；不要用户刚打第一个字就报错。错误边界 danger-text + 错误图标 + 具体说明，设置 aria-invalid、aria-describedby；帮助和错误都有独立 id。
- 提交失败 MUST 保留所有输入，聚焦错误摘要或首个无效字段；服务端错误映射到字段，不能只 toast。提交成功在实际完成后反馈，不伪造成功。
- 表单结构 SHOULD 为「固定页头 / 可滚动字段区 / 非滚动底部操作条」三行 grid；底栏位于主内容列内，不跨过侧栏。固定高度不得遮住最后一个字段、错误或焦点；字段滚动区保留 24px 底部间距，触屏加 env(safe-area-inset-bottom)。
- 若采用 sticky 底栏，MUST 预留实际占用空间并验证键盘弹出、200% 缩放和窄屏换行。离开未保存表单应提示且可取消，不丢草稿。

## 8. 表格、筛选与分页

- 列表顺序 MUST 是页头 → 筛选/搜索 → 结果与批量动作 → 表格/窄屏卡片 → 分页。页头不放没有需求的统计卡。
- 表格基础行高 40px 是单行**最小高度**，多行内容自然增高；表头 40px。单元格左右 12px，文字 14/22px，垂直居中；数字右对齐、文本左对齐、状态用一致位置。
- MUST 使用语义 table/th/td、caption 或关联标题，排序表头是可聚焦按钮并有 aria-sort；checkbox 有「选择某条记录」名称。不要随意加 role=grid 承担未实现的复杂键盘协议。
- 重要名称/标识不应无提示省略；长文本 MAY 截断，但可聚焦展开或详情可读。行 hover 使用 surface-hover，选中用 selected-bg/fg，勾选框和计数同步；状态徽标保留自己的状态颜色。
- 筛选 SHOULD 有明确「应用/重置」或文档化的即时更新策略；搜索带防抖时说明加载，Enter 可提交。筛选改变回到第 1 页，保留筛选输入；清空只清筛选，不删数据。
- 分页 MUST 有结果总数（确实已知时）、当前页、上一页/下一页，末页禁用正确。页容量默认 20，可选 20/50/100；未知总数显示「已加载 N 条」而不编造总数。
- 刷新/换页保留容器高度与筛选；加载不把整个应用清空。批量操作先明确选中范围，跨页选择必须说明数量。
- 窄屏优先保留名称、状态、关键值和操作，次字段转详情；若业务需要横向对照，可局部横滚并给提示。禁止整个应用横滚。

## 9. 弹窗、抽屉与反馈

- Dialog 默认 max-width 560px、距视口至少 16px；抽屉默认 480px、窄屏最大 100%；内容过高时仅弹层内容滚动，标题/操作仍可到达。弹层圆角 12px，shadow-overlay。
- MUST 有可访问标题、必要描述、关闭按钮；模态开启后焦点移入、Tab 不逃出、背景 inert、关闭后回到触发点。Escape 默认可关闭，未保存内容先确认；破坏操作初始聚焦取消或说明，不直接聚焦确认删除。
- 确认文案说清对象/数量/可恢复性，按钮用「删除记录」而非仅「确定」。危险行为不依靠红色作唯一提示。
- Popover/menu 的触发器状态、方向键/Enter/Escape 语义按所用可访问原语实现；MUST 不自造不完整焦点管理。
- Toast 放应用内容边缘，不遮关键操作；成功/普通消息 SHOULD 约 5 秒且可关闭，持久错误必须在内容区保留。一般反馈 aria-live=polite，紧急且必要错误才用 assertive，不对整个表格逐格播报。
- Tooltip 鼠标和键盘都可触发，不能承载唯一操作、字段错误或关键业务说明。

## 10. 四类页面模板

### 10.1 列表
页标题 24px 与短描述、右侧主要操作；下方筛选工具栏、表格、分页。数据行只保留与决策有关的字段与低强调行操作。清册示例可为「主机」列表：名称、IP、状态、位置、更新时间；这是示例，各项目按实际数据建模。

### 10.2 详情
页头为对象名、状态和一个主要动作组；内容按基本信息、关联数据、操作记录等真实领域分组。字段 label 使用 text-muted，值使用 text，空值显示「—」并可区分未配置。桌面两列字段，窄屏一列。返回入口在确有上层上下文时提供，不重复一整套导航。

### 10.3 新建/编辑表单
标题与必要说明；内容按任务分组，宽度建议 ≤800px；滚动字段区；底部主内容边界内操作条：取消 + 保存。表单错误留在对应字段，成功后按项目流程导航或显示完成反馈。

### 10.4 总览
最多建议 2–4 个真正可行动的中性指标块，配待办/最近变化/异常列表；指标要有统计口径和时间范围。状态色用于异常含义，不把每个 KPI 做不同颜色。不为填满屏幕虚构图表或指标。

## 11. 空、加载、错误、权限与长内容

| 状态 | MUST 呈现 | 操作 |
|---|---|---|
| 首次无数据 | 对象名与简短解释 | 有权限时提供首次创建 |
| 筛选无结果 | 保留筛选条件，说明没有匹配项 | 清空/调整筛选 |
| 初次加载 | 对应内容区骨架/进度，aria-busy | 不清空导航、不整页闪白 |
| 后台刷新 | 保留旧数据并标记刷新中 | 可取消或重试，避免重复点击 |
| 读取失败 | 原因/可理解消息、数据是否可能过期 | 重试；保留筛选与已填信息 |
| 无权限 | 说明需要的权限，不暴露受限数据 | 返回或申请权限（真实存在时） |
| 离线 | 显示离线及缓存时间（确有缓存时） | 恢复后重试；不伪装已保存 |
| 局部失败 | 只替换失败区块 | 单区重试，其他区域可继续 |

MUST 测试长产品名、长中文标题、无空格编号、0/1/大量行、缺失值。布局使用 min-width:0 和可控换行；不得靠 overflow:hidden 隐藏关键字段或错误。

## 12. 响应式与输入方式

[细化] 以下为布局参考断点，按内容溢出提前调整；不绑定特定 CSS 框架。

| 可用宽度 | 导航/布局 | 排版与控件 |
|---|---|---|
| ≥1200px | 232px 侧栏，主内容最多1440px；详情可双列 | 正文14/22，常规36px |
| 768–1199px | 空间不足折叠侧栏为可关闭抽屉；工具栏换行 | 不缩字硬塞；按输入设备调整目标 |
| <768px | 固定顶栏+抽屉导航，正文单列，页面padding16px | 正文/输入16/24，主要操作≥44px |

MUST 在 1440、1024、768、390px 和 320 CSS px 宽测试；200% 缩放不重叠、键盘可达；400%/320px 的重排测试允许需要二维对照的数据表局部横滚，其他内容不丢失。粗指针即使在宽屏也按触控目标处理。菜单不能只靠 hover；触屏点开后可关闭。软键盘出现后保存按钮和当前字段必须可到达；底栏换行时不截文案。

## 13. 键盘、焦点与无障碍门槛

- MUST 先用原生语义元素；表单标签真实关联，button 的 type 明确。提供跳到主内容链接，main/nav/header 等地标清楚。
- Tab 顺序与视觉顺序一致，不用正 tabindex；链接/按钮有可读名称，悬停操作也可经聚焦或常驻菜单找到。
- focus-visible 不能全局取消；轮廓不被 overflow 或 sticky 底栏遮挡。任何被脚本移动的焦点说明原因，关闭弹层恢复焦点。
- 文字与按钮标签按 WCAG 2.x 相对亮度公式 **≥4.5:1**；大字（常规≥24px，粗体≥18.67px）**≥3:1**。本版仍以所有计划文字组合4.5验收。
- 必要图标、辨认控件所需边界、焦点指示对相邻背景 **≥3:1**；纯装饰分割线和真正禁用控件例外。不得把必要控件轮廓当装饰绕过要求。
- 颜色不单独传意，状态有文字，错误有说明，选中有程序化状态与形状/字重/勾选。动画尊重 reduced-motion；高对比 forced-colors 下保留系统轮廓，不能只靠 box-shadow。
- tokens 的计算不是整站无障碍认证。MUST 补做实际渲染、字体、缩放、焦点、读屏和业务流程检查。

## 14. 工程接入、主题与扩展

1. AI MUST 先读当前项目规则、此文件、现有组件和页面路由；输出拟采用的组件/页面映射、差异和品牌参数，再实施授权范围内的变更。
2. 优先复用已有设计组件；语义组件封装 Button/Input/Table/Dialog 等状态。推荐 React + TypeScript + shadcn/ui + Tailwind 作为新项目候选，不锁框架或包版本；Vue 等按同一语义实现，不为换肤迁移框架。
3. 完整包可 import `tokens.css`，由 `html[data-theme="light"|"dark"]` 切换。首次主题采用用户保存选择，否则参考系统；渲染前尽早设置主题以避免闪烁，持久化失败不得破坏页面。
4. CSS 只定义变量，不修改 reset。项目 MUST 自行给 body/component 绑定 font、颜色和尺寸，不能以为 import 后组件已实现。窄屏/粗指针尺寸由组件样式消费 control-h-touch，不能只改颜色。
5. 已有 `.dark` 体系可以把它与 data-theme 统一到一个主题状态，不维护两个独立真源。shadcn 的 variable alias 使用完整颜色值，例如 `--background:var(--color-bg)`、`--foreground:var(--color-text)`、`--input:var(--color-control-border)`、`--ring:var(--color-focus)`；不要再包 hsl()。旧模板若用 `hsl(var(--background))`，MUST 同时修改消费表达式，先检查安装版本，不混用 HEX/OKLCH/HSL 通道。
6. `--border` 映射装饰 border；真正按钮/输入需明确 control-border。`--destructive` 对 danger、foreground 对 danger-fg；报错说明独立用 danger-text。不要依赖库自动派生颜色后不验收。
7. 产品差异写入项目 `brand-overrides.css` 和品牌参数文件，记录批准依据、版本及对比度；组件内部不直接覆盖全局设计语言。新增 token 带语义、浅深、用途、合法组合和测试。
8. MUST 完成清单后报告改动文件、受影响路由、使用的 token、测试结果、截图及未完成项。文档更新与实现偏差必须明确，不宣称尚未验证的「全站统一」。

### 14.1 无框架绑定的壳布局参考

```css
/* 类名仅示意；真实项目沿用自身命名。 */
.app-shell { height:100vh; height:100dvh; display:grid;
  grid-template-columns:var(--sidebar-w) minmax(0,1fr); overflow:hidden; }
.main-column { min-height:0; min-width:0; display:grid; grid-template-rows:auto minmax(0,1fr); }
.main-scroll { min-height:0; min-width:0; overflow:auto; }
.form-page { min-height:0; display:grid; grid-template-rows:auto minmax(0,1fr) auto; }
.form-fields { min-height:0; overflow:auto; padding-bottom:var(--space-5); }
.form-actions { background:var(--color-surface); border-top:1px solid var(--color-border);
  padding:12px 16px calc(12px + env(safe-area-inset-bottom)); }
:focus-visible { outline:var(--focus-width) solid var(--color-focus); outline-offset:var(--focus-offset); }
@media(max-width:767px) { .app-shell { grid-template-columns:minmax(0,1fr); } }
```

此示意仅展示轨道关系；项目 MUST 实现窄屏抽屉、地标、控件绑定及本文完整规则，不将示意当成完整组件。

## 15. 验收与版本维护

- MUST 浅深主题都覆盖列表/详情/表单/总览，至少补齐 empty/loading/error/permission；检查默认、hover、active、focus、disabled、loading、selected/invalid。
- MUST 验证滚动时侧栏/顶栏位置不变、底栏不遮字段、当前位置不重复、桌面按钮36/32且触控主要操作≥44、正文未被压到12px。
- MUST 提供相同数据、相同路由、相同视口的浅深截图作对照；截图记录视口/缩放/主题/状态，不仅给截图说「已完成」。
- 机器校验使用包内 `validate.py`，校验 JSON/CSS/文档表格一致性、预定义颜色组合及基础尺寸；它不执行 DOM/浏览器测试，不代替人工验收。
- 1.0 为当前建议基线。产品名/正式 logo、品牌资产路径、项目专属文案与字段属于可配置项，不阻塞采用。新增品牌覆盖、组件变体或密度变化 SHOULD 升版本并保留差异记录。

### 本版相对静态 demo 的明确修订

1. 分离装饰 border 和必要 control-border；浅色 #64748B、深色 #94A3B8，包含 hover/active。
2. 添加固定 hover/active/disabled/focus 语义，不再由 AI 随意 color-mix 派生。
3. placeholder 使用完整 text-muted 不降透明度；状态正文与实心填色分离。
4. 深色实心按钮加可辨轮廓；焦点环通过中性间隙与实心隔离。
5. 浅色 active 表面采用 #EDF2F7，保证链接与成功说明在按下背景上仍达到正文4.5:1。
6. 补全响应式、44px触控、表单底栏占位、键盘及 VI 参数边界。
