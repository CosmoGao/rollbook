# v0.14.0 发布说明草稿（待发布）

本文件随独立发布文档 PR 提交。当前尚无 `v0.14.0` 正式 tag、Release、镜像 digest、CLI 哈希或目标版本截图，以下内容不得视为正式产物证据。发布文档 PR 完成 CI、独立验收并 squash 合并后，才核对最终 `main`、推 tag、核验 Release 和产物。发布本身不会更新现有实例；现网登录方式保持原样。

## 变更范围

本版按 `v0.13.0..main` 的已合并 [#76](https://github.com/CosmoGao/rollbook/pull/76)、[#77](https://github.com/CosmoGao/rollbook/pull/77)、[#78](https://github.com/CosmoGao/rollbook/pull/78) 归档。#76 仅补齐上一正式版文档与 Release 链接，不是新功能。#77 增加个人设置邮箱与飞书绑定按钮、认证设置账号邮箱编辑，以及飞书邮箱唯一匹配的自动绑定。#78 稳定 HEADER 桌面及窄屏搜索、主题控件的强制颜色焦点验收；`web/header-controls.browser.test.cjs` 继续在 `emulateMedia({forcedColors:'active'})` 后断言外框 `outlineStyle` 为 `solid`、`outlineOffset` 为 `0px`，不使用其他控件截图替代。此 PR 不新增目标版本截图。

个人设置路径为 `/settings/personal`：本地登录用户可保存邮箱并点击「绑定飞书」，经现有飞书应用授权返回。管理员可在 `/settings/auth?section=accounts` 修改账号邮箱，飞书设置仍在 `/settings/auth?section=feishu`。邮箱可空；非空时去首尾空格、按 ASCII 大小写折叠并保持全局唯一。飞书登录优先使用已允许的稳定 `open_id` 绑定；若返回邮箱归属其他用户则拒绝冲突。无主体绑定时，仅在飞书 `data.email` 唯一匹配启用的现有账号时原子绑定并登录；缺失、不匹配或不唯一时不开户、不发 session。禁用绑定或停用用户不能绕过，不按姓名匹配，也不静默重绑已归属其他账号的主体。绑定与登录的 `state` 和 `purpose` 隔离，绑定回调保持原本地会话；公开回调地址须包含实际 HTTPS 端口，如 `:5443`。

邮箱匹配要求飞书应用启用并发布 `contact:user.email:readonly`，授权同时请求 `auth:user.id:read`。若尚无邮箱权限或上游未返回 `data.email`，按无唯一匹配处理；用户可先本地登录，再用个人设置中的绑定按钮。权限缺失不阻塞本次正式发版。真实飞书、LDAP、OIDC 上游联调仍需在受控环境验证；没有新增全站 RBAC、群组映射或自动开户。当前现网登录策略保持 `local=true`、`feishu=true`、`ldap=false`、`oidc=false`，本次发布文档不修改它们。

## 升级与回退

相对 `v0.13.0`，新增前向迁移 `internal/db/migrations/018_feishu_email.sql`：`users.email`，非空邮箱的 `lower(trim(email))` 唯一索引，以及 `auth_pending.purpose`、`auth_pending.user_id`、`auth_pending.session_hash`。认证后端增加邮箱匹配、绑定用途与会话校验；没有发现新的 pepper 或认证配置加密密钥方案。迁移只向前执行，不得让旧镜像直接读取升级后的数据库。

升级前停服，保存完整 `/data` 文件级快照，SQLite 数据库与 `-wal`、`-shm` 一同保存；另存独立使用的 `/backups` 卷。保留当时的环境变量与配置、`ROLLBOOK_AUTH_PEPPER` 或其 `_FILE` 文件、身份源环境变量或 `_FILE` 文件，以及认证配置加密所需密钥材料，并在隔离环境使用该快照验证恢复。完整 YAML 不等于文件级快照，也不含页面身份源凭据。具体 Compose 停服与复制示例见[升级和回退](../README.md#升级和回退)；仓库默认 `compose.yaml` 不自动转发身份源环境变量，须核对实际部署配置。

回退时先停服，恢复升级前完整 `/data` 与独立 `/backups` 快照、当时配置与密钥，然后启动上一正式镜像 `ghcr.io/cosmogao/rollbook:v0.13.0@sha256:9946132f927915687972e257b27bca8e74505d526427e0aa5cc3003f5be18e5c`。不能只更换旧镜像。原 pepper 丢失或更换时，页面密文不能解密；须先在本机运行 `rollbook auth local-login enable` 保留恢复入口，再重新填写页面凭据。实际升级与部署另行进行，不在本发布文档 PR 中执行。

## 正式发布后待核验

在正式 tag 流水线完成后，核实 Release 非草稿且非预发布、tag 解引用提交与发布时最终 `main` 一致；检查 GHCR 多架构 index、`linux/amd64` 与 `linux/arm64` manifest digest，并读取两个平台 image config 的 OCI `version` 与 `revision` 标签。下载六平台 CLI、`install.sh` 和 `SHA256SUMS`，执行 `sha256sum -c SHA256SUMS`；在适用的 `linux/amd64` 环境执行 `rollbook version`，核对 tag 与短提交号。只在取得真实证据后回写 Release URL、digest、哈希及命令输出。
