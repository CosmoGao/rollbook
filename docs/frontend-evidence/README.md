# 第一阶段真页面截图

同一组 mock API 数据、同一路由、同一视口分别截取浅色与深色。文件名 `v1-{login|overview}-{1440|390|320}-{light|dark}.png`，视口高度分别为 900 / 844 / 844 CSS px，缩放 100%，Chromium Playwright。`login` 为未授权登录页，`overview` 为登录态总览及应用壳。测试入口：`web/ui-refresh.browser.test.cjs` 的「固定版登录与应用壳」用例。截图中的 mock 版本号仅为测试数据。

截图证明视觉状态；尺寸、焦点、无溢出、固定顶栏和抽屉关闭由该用例断言。业务页及其他尺寸/缩放、读屏仍未验证。
