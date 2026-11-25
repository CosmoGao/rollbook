# v0.12.0 Release 说明

正式正文见 [v0.12.0 Release](https://github.com/CosmoGao/rollbook/releases/tag/v0.12.0)。tag 指向 `47f96d343eb91c06c6ceb23122de7a1bd058a3af`；正式镜像 `ghcr.io/cosmogao/rollbook:v0.12.0` 的 index 为 `sha256:ccb1ead248f7d00d365473a581eab8eeb4d01a6407ba11c25dca041415077f49`，Release 附有六平台 CLI、`install.sh` 与 `SHA256SUMS`。此发版后文档补正只进入后续 `main`，不在不可变的正式 tag 内。

## 修复

- 全站独立文本 `input`、`textarea`、`select` 的焦点色与可见强度沿自身 `border-box` 绘制；搜索框与 HEADER 组合控件继续由外层绘制唯一外框。错误态和 forced-colors 焦点反馈保留。原生 select 的系统展开层不属于页面 CSS 修复范围（[#71](https://github.com/CosmoGao/rollbook/pull/71)；[路由矩阵与脱敏截图](control-geometry.md)）。

`v0.11.0..v0.12.0` 另有 [#70](https://github.com/CosmoGao/rollbook/pull/70) 和 [#72](https://github.com/CosmoGao/rollbook/pull/72)，分别修正上一版文档与归档本次发布文档，均不是本版新功能。快捷键、主题、认证凭据只写不读及现有登录策略保持不变。真实 LDAP、OIDC、飞书 IdP 尚未联调；没有新增全站 RBAC 或群组映射。

## 升级与回退

相对 `v0.11.0` 无新增数据库迁移、无密钥方案变更。升级前停服并保存完整 `/data` 文件级快照，包含 SQLite 数据库及 `-wal`、`-shm`；独立使用 `/backups` 时也保存该卷。保留原 `.env`、`ROLLBOOK_AUTH_PEPPER` 或其 `_FILE` 文件、身份源环境变量或 `_FILE` 文件及认证配置加密所需密钥材料，在隔离环境验证快照可恢复。完整 YAML 不能代替快照，也不能恢复页面身份源凭据。

升级前核对正式 tag、Release 与镜像，并完成快照恢复验证。回退先停服，恢复升级前完整快照、原环境和密钥，再运行 `v0.11.0` 正式镜像；不能仅更换镜像读取升级后的数据。原 pepper 丢失或更换时，须先在本机运行 `rollbook auth local-login enable` 保留本地恢复入口，再重新填写页面凭据。详见 [README 升级和回退](../README.md#升级和回退)。发布不会自动部署现有实例。
