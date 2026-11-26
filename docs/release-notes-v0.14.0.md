# v0.14.0 正式发布说明

本版已正式发布：[v0.14.0 Release](https://github.com/CosmoGao/rollbook/releases/tag/v0.14.0)。发布文档经独立验收并合并后，从最终 `main` 创建 tag；下文记录真实产物核验。发版本身不会更新现有实例；现网登录方式保持原样。本文发版后证据补正在后续 `main`，不属于不可变的 `v0.14.0` tag。

## 变更范围

本版按 `v0.13.0..v0.14.0` 的已合并 [#76](https://github.com/CosmoGao/rollbook/pull/76)、[#77](https://github.com/CosmoGao/rollbook/pull/77)、[#78](https://github.com/CosmoGao/rollbook/pull/78) 归档。#76 仅补齐上一正式版文档与 Release 链接，不是新功能。#77 增加个人设置邮箱与飞书绑定按钮、认证设置账号邮箱编辑，以及飞书邮箱唯一匹配的自动绑定。#78 稳定 HEADER 桌面及窄屏搜索、主题控件的强制颜色焦点验收；`web/header-controls.browser.test.cjs` 继续在 `emulateMedia({forcedColors:'active'})` 后断言外框 `outlineStyle` 为 `solid`、`outlineOffset` 为 `0px`，不使用其他控件截图替代。本次发版没有新增目标版本截图。

个人设置路径为 `/settings/personal`：本地登录用户可保存邮箱并点击「绑定飞书」，经现有飞书应用授权返回。管理员可在 `/settings/auth?section=accounts` 修改账号邮箱，飞书设置仍在 `/settings/auth?section=feishu`。邮箱可空；非空时去首尾空格、按 ASCII 大小写折叠并保持全局唯一。飞书登录优先使用已允许的稳定 `open_id` 绑定；若返回邮箱归属其他用户则拒绝冲突。无主体绑定时，仅在飞书 `data.email` 唯一匹配启用的现有账号时原子绑定并登录；缺失、不匹配或不唯一时不开户、不发 session。禁用绑定或停用用户不能绕过，不按姓名匹配，也不静默重绑已归属其他账号的主体。绑定与登录的 `state` 和 `purpose` 隔离，绑定回调保持原本地会话；公开回调地址须包含实际 HTTPS 端口，如 `:5443`。

邮箱匹配要求飞书应用启用并发布 `contact:user.email:readonly`，授权同时请求 `auth:user.id:read`。若尚无邮箱权限或上游未返回 `data.email`，按无唯一匹配处理；用户可先本地登录，再用个人设置中的绑定按钮。权限缺失不阻塞本次正式发版。真实飞书、LDAP、OIDC 上游联调仍需在受控环境验证；没有新增全站 RBAC、群组映射或自动开户。当前现网登录策略保持 `local=true`、`feishu=true`、`ldap=false`、`oidc=false`，本次发布文档不修改它们。

## 升级与回退

相对 `v0.13.0`，新增前向迁移 `internal/db/migrations/018_feishu_email.sql`：`users.email`，非空邮箱的 `lower(trim(email))` 唯一索引，以及 `auth_pending.purpose`、`auth_pending.user_id`、`auth_pending.session_hash`。认证后端增加邮箱匹配、绑定用途与会话校验；没有发现新的 pepper 或认证配置加密密钥方案。迁移只向前执行，不得让旧镜像直接读取升级后的数据库。

升级前停服，保存完整 `/data` 文件级快照，SQLite 数据库与 `-wal`、`-shm` 一同保存；另存独立使用的 `/backups` 卷。保留当时的环境变量与配置、`ROLLBOOK_AUTH_PEPPER` 或其 `_FILE` 文件、身份源环境变量或 `_FILE` 文件，以及认证配置加密所需密钥材料，并在隔离环境使用该快照验证恢复。完整 YAML 不等于文件级快照，也不含页面身份源凭据。具体 Compose 停服与复制示例见[升级和回退](../README.md#升级和回退)；仓库默认 `compose.yaml` 不自动转发身份源环境变量，须核对实际部署配置。

回退时先停服，恢复升级前完整 `/data` 与独立 `/backups` 快照、当时配置与密钥，然后启动上一正式镜像 `ghcr.io/cosmogao/rollbook:v0.13.0@sha256:9946132f927915687972e257b27bca8e74505d526427e0aa5cc3003f5be18e5c`。不能只更换旧镜像。原 pepper 丢失或更换时，页面密文不能解密；须先在本机运行 `rollbook auth local-login enable` 保留恢复入口，再重新填写页面凭据。实际升级与部署另行进行，不在本发版范围内执行。

## 正式产物核验

正式 [Release](https://github.com/CosmoGao/rollbook/releases/tag/v0.14.0)。annotated tag `v0.14.0` 解引用提交为 `29a4ddb62d018a08efa161cbad67bf22d0eeb753`，与推 tag 时最终 `main` 一致。tag Build [45804463968](https://github.com/CosmoGao/rollbook/actions/runs/45804463968) 的 `test`、`publish` 均成功。Release 非草稿、非预发布。

正式多架构镜像针：`ghcr.io/cosmogao/rollbook:v0.14.0@sha256:5b116dabebd401bcda38197db329680f7ca05fc2c277fd9f4ecb6998f39a01bc`。`linux/amd64` manifest：`sha256:8f5156b6469fa84aeca984f032638a82a632a46e2b1e7eeaa26d5e9f6d6c1b3d`；`linux/arm64` manifest：`sha256:f0906213cc0a41b9fae98d43ac34048a80d10ce59177c876a769b3bf852b95cc`。两个平台 image config 的 `org.opencontainers.image.version=v0.14.0`、`org.opencontainers.image.revision=29a4ddb62d018a08efa161cbad67bf22d0eeb753`，均已读取并核对。

下载 Release 所附六平台 CLI、`install.sh`、`SHA256SUMS` 后，执行 `sha256sum -c SHA256SUMS`：七项均 `OK`。`SHA256SUMS` 自身 SHA-256 为 `4136ff5114dce67a4830ff779a8f16bea63ec8f821d8219b13dcf9c215856602`。清单内容：

```text
c1320588889651785d755be94db326deee3182d8b0aa65f89d36badf3aca4898  rollbook_v0.14.0_darwin_amd64.tar.gz
15ebb50ec48b50b3a9a0e341a7306f598cd0d8539771d6b55d9333c6c09234e9  rollbook_v0.14.0_darwin_arm64.tar.gz
2492100981d4a050ef030b7665187a3658b5987d4bcbba68808a4cc4cacecc39  rollbook_v0.14.0_linux_amd64.tar.gz
3b998c54dcdcb074aa27be0328522fe3f71d13900c36b59b79325c3ddc8bfb5a  rollbook_v0.14.0_linux_arm64.tar.gz
c8745a44cc8477d16f7b33d3bf5778d2f1f6b5d9e9f77f44328e873cf398a5e8  rollbook_v0.14.0_windows_amd64.tar.gz
25083d18581c1f511398503a002635dc6f11dce01ccfe3b94e19dfebb8875f7d  rollbook_v0.14.0_windows_arm64.tar.gz
0aaba784085c18ccfb2a8a4b45b28a3b999bcf4aab9979d960381da177ef9515  install.sh
```

在 `linux/amd64` 解包并执行 `./rollbook version`，输出：`rollbook v0.14.0 (29a4ddb)`。这是发版产物核验，**不代表现网已部署**。

复核命令：`git rev-parse v0.14.0^{}`、`git ls-remote origin refs/heads/main refs/tags/v0.14.0 refs/tags/v0.14.0^{}`、`gh release view v0.14.0 --json url,isDraft,isPrerelease,assets`；对 GHCR tag 拉取 OCI index、双平台 manifest 与 image config，并按各对象原始字节复算 SHA-256，再读取 config Labels；`gh release download v0.14.0 --dir release-assets`、`cd release-assets && sha256sum -c SHA256SUMS`、`tar -xzf rollbook_v0.14.0_linux_amd64.tar.gz rollbook && ./rollbook version`。
