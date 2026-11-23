# 颜色与文件一致性校验结果

版本1.0 · 2026-11-23（Asia/Shanghai，UTC+8）

复现：`python3 validate.py`（Python 3 标准库，无第三方依赖）。

- 颜色组合：166/166 通过。
- 结构与一致性断言：272/272 通过。
- 最小文字组合：4.5889:1；最小非文本组合：4.2251:1。
- 方法：WCAG 2.x sRGB 相对亮度，线性化阈值0.04045；(L亮+0.05)/(L暗+0.05)；不透明组合；未四舍五入值用于判定。
- 范围：计划中合法 token 组合，不检查 DOM、真实背景叠加、焦点几何、屏幕阅读器或整站合规。不宣称整站无障碍认证。

## 必要轮廓修订与主要组合

| 主题 | 组合 | 计算值 | 门槛 | 结果 |
|---|---|---|---|---|
| light | border / surface | 1.2328:1 | 3:1 | 装饰线，不充当必要轮廓 |
| light | control-border / surface | 4.7588:1 | 3:1 | 通过 |
| light | control-border / bg | 4.5484:1 | 3:1 | 通过 |
| light | text / bg | 17.0629:1 | 4.5:1 | 通过 |
| light | text-muted / surface-active | 6.7278:1 | 4.5:1 | 通过 |
| light | primary-fg / primary | 5.1686:1 | 4.5:1 | 通过 |
| light | selected-fg / selected-bg | 6.1580:1 | 4.5:1 | 通过 |
| light | focus / surface | 5.1686:1 | 3:1 | 通过 |
| dark | border / surface | 1.4128:1 | 3:1 | 装饰线，不充当必要轮廓 |
| dark | control-border / surface | 5.7053:1 | 3:1 | 通过 |
| dark | control-border / bg | 6.9627:1 | 3:1 | 通过 |
| dark | text / bg | 16.2960:1 | 4.5:1 | 通过 |
| dark | text-muted / surface-active | 4.7519:1 | 4.5:1 | 通过 |
| dark | primary-fg / primary | 5.1686:1 | 4.5:1 | 通过 |
| dark | selected-fg / selected-bg | 7.2896:1 | 4.5:1 | 通过 |
| dark | focus / surface | 5.7538:1 | 3:1 | 通过 |

## 修订说明

装饰 border 沿用色值，在白色/暗表面上不足3:1；保留其分割用途，新增 control-border 用于必要控件边界。深色实心按钮采用该轮廓与周围中性背景区分；焦点环以2px背景间隙隔离。完整状态值已显式定义，placeholder不叠加透明度。状态正文用独立 *-text，避免把暗色填色当文字。浅色active表面由初次候选#E2E8F0修正为#EDF2F7：默认链接由4.1926:1提高到4.5889:1，成功说明由4.3917:1提高到4.8068:1。

## 校验边界

禁用项为规范例外，不作为通过组合；装饰分割线、遮罩叠加、品牌覆盖和图表色两两对比不被泛化认证。实际组件仍需检查可见轮廓、字号、层叠、键盘和读屏。全部合法组合实数和每项文件断言见 validation-results.json。
