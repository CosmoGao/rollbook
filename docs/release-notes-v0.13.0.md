# v0.13.0 发布记录

本版已正式发布：[v0.13.0 Release](https://github.com/CosmoGao/rollbook/releases/tag/v0.13.0)，非草稿、非预发布。tag 解引用提交为 `8b478b4c8ab4199baf68ffc33411e3cdac23c92f`。相对 `v0.12.0`，[#73](https://github.com/CosmoGao/rollbook/pull/73) 仅修正上一版文档与链接；本版用户可见修复来自 [#74](https://github.com/CosmoGao/rollbook/pull/74)，发布文档来自 [#75](https://github.com/CosmoGao/rollbook/pull/75)。正式发布不会自动部署现有实例。本文件的发版后事实补正只进入后续 `main`，不在不可变的 `v0.13.0` tag 内。

## 已核验产物

- 正式镜像：`ghcr.io/cosmogao/rollbook:v0.13.0`；多架构 index `sha256:9946132f927915687972e257b27bca8e74505d526427e0aa5cc3003f5be18e5c`；`linux/amd64` manifest `sha256:1df4218e00df2c765df698e3f0f67f676e8e883eab297cfc88fcddf9931e289f`；`linux/arm64` manifest `sha256:4cc2939cae95d088450b9396f902e2e1635b6d74eb37f7f3d9f381a6d5592b75`。两架构 image config 的 OCI `version=v0.13.0`、`revision=8b478b4c8ab4199baf68ffc33411e3cdac23c92f`。
- [Release 附件](https://github.com/CosmoGao/rollbook/releases/tag/v0.13.0) 包含 Linux、macOS、Windows 各 amd64/arm64 的六份 CLI 压缩包、`install.sh` 和 `SHA256SUMS`；下载后 `sha256sum -c SHA256SUMS` 七项均通过。校验清单自身 SHA256 为 `a2480d9b13289529db37c48b401510cf5fb24952fcda9d281ae45dfec4f6ba70`；Linux amd64 的 `rollbook version` 输出 `rollbook v0.13.0 (8b478b4)`。

## 修复

- 飞书认证设置中，展示的 HTTPS 非 443 公开回调可保存。App ID 缺失、凭据动作非法或缺少替换凭据、回调地址不匹配时，返回中英文字段级 `422 validation`，分别定位 `client_id`、`secret_action`、`redirect_url`，不回显凭据。
- HTTP API 公开 origin 使用直连 Host/TLS，或仅采信可信直接代理的单值转发头；伪造、冲突、多值与逗号链拒绝。代理须覆盖客户端转发头，正确传送公开协议、主机和端口；详见[公开地址与校验](public-auth-origin.md)。未启用飞书时登录页不显示飞书入口。LDAP/OIDC 的 HTTPS、固定回调、显式绑定及安全失败约束不回退。

搜索快捷键、主题、认证 secret 只写不读、防锁死恢复入口和现有权限行为保持不变。真实 LDAP、OIDC、飞书 IdP 尚未联调；没有新增全站 RBAC 或群组映射。保持本地登录开关与生产身份源配置不变，不要因本次发版启用外部身份源。

## 升级与回退

复核 `v0.12.0..v0.13.0`：`internal/db/migrations`、`internal/auth`、`internal/secrets`、`cmd` 无变化；相对 `v0.12.0` 无新增数据库迁移、无密钥方案变更。HTTP API 公开 origin 与设置校验确有变化。

升级前停服，保存完整 `/data` 文件级快照，包括 SQLite 数据库、`-wal`、`-shm`；独立使用 `/backups` 时一并保存该卷。保留原环境变量、`ROLLBOOK_AUTH_PEPPER` 或其 `_FILE` 文件、身份源环境变量或 `_FILE` 文件及认证配置加密所需密钥材料，并在隔离环境验证恢复。完整 YAML 不能代替文件级快照，也不能恢复页面身份源凭据。

正式 tag、Release 与镜像产物已核验；升级请固定 `v0.13.0`，不得使用 `main`、`latest` 或 `sha-*` 镜像冒充正式版。回退先停服，恢复升级前完整快照、原配置和密钥，再启动 `v0.12.0` 正式镜像；不能只换旧镜像读取升级后的数据。页面密文只能用原 pepper 解密；若 pepper 丢失或更换，先在本机运行 `rollbook auth local-login enable` 保留恢复入口，再重新填写页面凭据。操作示例见 [README 升级和回退](../README.md#升级和回退)。
