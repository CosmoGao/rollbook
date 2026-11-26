# v0.13.0 Release 正文草稿（待发布）

本文件供正式发版时核对；目前没有 `v0.13.0` tag、Release URL、镜像或 digest，不得作为已发布产物引用。版本号依据 [发版检查](releasing.md) 和现有次版本归档方式选择。正式发布不会自动部署现有实例。相对 `v0.12.0`，[#73](https://github.com/CosmoGao/rollbook/pull/73) 仅修正上一版文档与链接；本版用户可见修复来自 [#74](https://github.com/CosmoGao/rollbook/pull/74)。

## 修复

- 飞书认证设置中，展示的 HTTPS 非 443 公开回调可保存。App ID 缺失、凭据动作非法或缺少替换凭据、回调地址不匹配时，返回中英文字段级 `422 validation`，分别定位 `client_id`、`secret_action`、`redirect_url`，不回显凭据。
- HTTP API 公开 origin 使用直连 Host/TLS，或仅采信可信直接代理的单值转发头；伪造、冲突、多值与逗号链拒绝。代理须覆盖客户端转发头，正确传送公开协议、主机和端口；详见[公开地址与校验](public-auth-origin.md)。未启用飞书时登录页不显示飞书入口。LDAP/OIDC 的 HTTPS、固定回调、显式绑定及安全失败约束不回退。

搜索快捷键、主题、认证 secret 只写不读、防锁死恢复入口和现有权限行为保持不变。真实 LDAP、OIDC、飞书 IdP 尚未联调；没有新增全站 RBAC 或群组映射。保持本地登录开关与生产身份源配置不变，不要因本次发版启用外部身份源。

## 升级与回退

复核 `v0.12.0..main`：`internal/db/migrations`、`internal/auth`、`internal/secrets`、`cmd` 无变化；相对 `v0.12.0` 无新增数据库迁移、无密钥方案变更。HTTP API 公开 origin 与设置校验确有变化。

升级前停服，保存完整 `/data` 文件级快照，包括 SQLite 数据库、`-wal`、`-shm`；独立使用 `/backups` 时一并保存该卷。保留原环境变量、`ROLLBOOK_AUTH_PEPPER` 或其 `_FILE` 文件、身份源环境变量或 `_FILE` 文件及认证配置加密所需密钥材料，并在隔离环境验证恢复。完整 YAML 不能代替文件级快照，也不能恢复页面身份源凭据。

待正式 tag、Release 与镜像产物核验完成后才能升级；不得使用 `main`、`latest` 或 `sha-*` 镜像冒充正式版。回退先停服，恢复升级前完整快照、原配置和密钥，再启动 `v0.12.0` 正式镜像；不能只换旧镜像读取升级后的数据。页面密文只能用原 pepper 解密；若 pepper 丢失或更换，先在本机运行 `rollbook auth local-login enable` 保留恢复入口，再重新填写页面凭据。操作示例见 [README 升级和回退](../README.md#升级和回退)。
