# 外部身份登录

## v0.13.0 已发布修复

飞书设置页 `/settings/auth?section=feishu` 显示的带端口 HTTPS 回调现在可保存。App ID、凭据动作及回调校验失败分别返回中英文字段级 `422 validation`（`client_id`、`secret_action`、`redirect_url`）；密文解密失败仍为 `409 credential_unavailable`。公开 origin 使用直连 Host/TLS 或可信直接代理的单值转发头；伪造、冲突、多值与逗号链拒绝。未启用时登录页没有飞书入口。LDAP/OIDC 的 HTTPS、固定回调、显式绑定和安全失败约束保持不变。代理配置见[公开地址与校验](public-auth-origin.md)；详见[本版发布记录](release-notes-v0.13.0.md)与[正式 Release](https://github.com/CosmoGao/rollbook/releases/tag/v0.13.0)。

相对 `v0.12.0` 无新增数据库迁移或密钥方案变更，但 HTTP API 公开 origin 与设置校验已修复。真实 LDAP、OIDC、飞书 IdP 尚未联调；无新增全站 RBAC 或群组映射。升级时保持本地登录开关与生产身份源配置不变；先停服并保存含 SQLite `-wal`、`-shm` 的完整 `/data` 快照、独立 `/backups` 卷和原环境、pepper 或 `_FILE`、身份源及加密密钥材料，在隔离环境验证恢复。完整 YAML 不能代替快照或恢复页面凭据。回退先停服，恢复升级前完整快照、原配置及密钥，再运行 `v0.12.0` 正式镜像，不能让旧镜像直接读取升级后的数据。pepper 丢失或更换时先在本机运行 `rollbook auth local-login enable`，再重新填写页面凭据。

以下既有操作说明对应已正式发布的 [v0.11.0 Release](https://github.com/CosmoGao/rollbook/releases/tag/v0.11.0)。发布不会自动升级现有实例。

已发布的 [v0.12.0 Release](https://github.com/CosmoGao/rollbook/releases/tag/v0.12.0) 中，全站文本控件焦点修复仅改变界面描边；正式产物已核验，发布不等于生产部署，当前登录策略和身份源配置保持不变。相对 `v0.11.0` 无新增数据库迁移、无密钥方案变更。升级前停服并保存完整 `/data` 文件级快照（含 SQLite `-wal`、`-shm`），独立使用 `/backups` 时也保存该卷；保留原环境、pepper 或其 `_FILE`、身份源配置与密钥，并在隔离环境验证恢复。回退先停服、恢复完整快照与原配置，再运行 `v0.11.0` 正式镜像；不能只切换镜像。完整 YAML 不能代替快照或恢复页面身份源凭据。详见 [升级和回退](../README.md#升级和回退)。

真实 LDAP、OIDC、飞书 IdP 联调尚未进行；现有验收使用假上游、协议测试与浏览器测试。默认只有本地账号入口。每个网页用户仍拥有清册全部管理能力。本版没有全站 RBAC、自动开户、邮件/同名自动绑定或群组映射。外部身份须明确绑定稳定主体到已有或预置的清册用户并允许登录。发布、部署和实际身份源试登录应分别核验。

## 网页认证设置与边界

系统「设置 → 认证设置」（`/settings/auth`，默认「本地登录」；`?section=ldap|oidc|feishu|accounts|audit` 可直达其他分组）通过六组导航管理身份源、用户与绑定、允许名单、本地登录开关及只读的分页认证审计。右上角头像的「个人设置」（`/settings/personal`）管理当前用户信息、可用时的本地改密和清册会话；系统「备份与恢复」（`/settings/backup`）管理 YAML，不负责保存页面身份源凭据。所有已登录网页用户仍有完整管理权；Bearer API Key 无法调用认证设置管理接口。

切换分组或离开页面时，未保存草稿会触发确认；刷新或关闭浏览器使用浏览器原生确认。保存某一身份源只刷新该源，其他未保存草稿继续保留。

在认证设置页分别配置并启停 LDAP/LDAPS、通用 OIDC、飞书。LDAP 表单包含服务器 URL、实例名、Base DN、Bind DN、含 `{login}` 的搜索筛选器和稳定主体属性；Bind Password 在敏感凭据栏填写。OIDC 表单包含 Issuer、Client ID、固定回调 URL 与 Client Secret；飞书表单包含 App ID、固定回调 URL 与 App Secret。页面展示回调 URL，须在 IdP 后台登记完全相同的公开 HTTPS 地址；若服务在反向代理后，先确认浏览器访问的主机名正确。LDAP 使用受验证证书的 LDAPS 或 StartTLS，不提供跳过证书验证选项。

公开 HTTPS 使用非 443 端口时，回调地址必须包含该端口。TLS 反代若将内层 `Host` 改为无端口的主机名，需将代理的直接地址设为可信代理，并覆盖客户端转发头，传入 `X-Forwarded-Proto: https` 与带公开端口的 `X-Forwarded-Host`，或另传 `X-Forwarded-Port`；否则设置页会显示浏览器与服务端公开地址差异，保存时拒绝。`Forwarded` 的单值 `host`/`proto` 也可用；冲突、多值或伪造的转发头不被接受。操作示例、错误码和兼容行为见[外部认证公开地址与校验](public-auth-origin.md)。

页面仅显示敏感凭据「已配置／未配置」，支持明确选择保持、覆盖或清除；不会读回明文。服务端以现有 `ROLLBOOK_AUTH_PEPPER` 派生密钥加密后保存于 `auth_provider_settings`，凭据不以明文落库，也不进入完整 YAML 导出、定时 YAML 备份、前端 API 响应、日志或错误回显。应保护 pepper 与数据卷；更换 pepper 后原密文无法解密。

身份源按源整体判定来源：任一对应环境变量或 `_FILE` 存在时，该源由启动环境接管，页面显示来源为「启动环境」且只读，之前保存的页面配置被遮蔽。移除环境变量后须重启服务才能切换来源；未被环境接管的页面配置保存及启停热生效，无需重启容器。页面「测试连通」只校验网络与参数，不进行用户登录，不写 `identity_bindings.verified_at`；必须用真实外部身份试登录并核对清册账号与权限，才能满足关闭本地入口的条件。认证审计当前只显示时间、操作、目标，不记录操作者或 IP。

## 启动环境配置（可选）

| 类型 | 变量 | 用途 |
| --- | --- | --- |
| LDAP | `ROLLBOOK_LDAP_URL` | `ldaps://host:636` 或 `ldap://host:389`；后者必须成功 StartTLS，证书走系统信任链，不允许跳过验证。留空关闭。 |
| LDAP | `ROLLBOOK_LDAP_INSTANCE`, `ROLLBOOK_LDAP_BASE_DN`, `ROLLBOOK_LDAP_BIND_DN` | 稳定目录实例名、最小权限只读搜索范围和 bind 账号 DN。 |
| LDAP | `ROLLBOOK_LDAP_BIND_PASSWORD` 或 `ROLLBOOK_LDAP_BIND_PASSWORD_FILE` | 搜索账号口令，只可设置其一。 |
| LDAP | `ROLLBOOK_LDAP_FILTER`, `ROLLBOOK_LDAP_SUBJECT_ATTRIBUTE` | 查询过滤器含 `{login}` 占位符，例如 `(uid={login})`；输入由服务端按 LDAP 规则转义。主体属性必须稳定且唯一，例如 `entryUUID`。 |
| OIDC | `ROLLBOOK_OIDC_ISSUER`, `ROLLBOOK_OIDC_CLIENT_ID` | HTTPS issuer 发现地址与 client ID；issuer 留空关闭。绑定键为 issuer + `sub`。 |
| OIDC | `ROLLBOOK_OIDC_CLIENT_SECRET` 或 `ROLLBOOK_OIDC_CLIENT_SECRET_FILE` | 机密客户端 secret，只可设置其一。 |
| OIDC | `ROLLBOOK_OIDC_REDIRECT_URL` | 固定公开 HTTPS 地址，路径必须为 `/api/auth/oidc/callback`；在 IdP 登记完全相同地址。 |
| 飞书 | `ROLLBOOK_FEISHU_CLIENT_ID` | 飞书 App ID，留空关闭；绑定实例为 App ID。 |
| 飞书 | `ROLLBOOK_FEISHU_CLIENT_SECRET` 或 `ROLLBOOK_FEISHU_CLIENT_SECRET_FILE` | 飞书 App Secret，只可设置其一。 |
| 飞书 | `ROLLBOOK_FEISHU_REDIRECT_URL` | 固定公开 HTTPS 地址，路径必须为 `/api/auth/feishu/callback`；在飞书后台登记完全相同地址。 |

表内环境变量须在服务进程启动环境中提供；仓库默认 `compose.yaml` 并未转发这些身份源变量，不要仅在 `.env` 中增加变量就认为容器已接收。变更启动环境或 `_FILE` 须重启。`GET /api/config` 只返回各入口是否可显示与 HTTP 风险，不返回上游 URL、bind DN 或 secret。OIDC 采用发现文档、授权码、state、nonce、PKCE S256 和 ID Token 签名、issuer、audience、时效校验；飞书独立使用授权码换用户 token，再读取 `open_id`；LDAP 仅经验证证书的 LDAPS 或 StartTLS 搜索和用户 bind。所有失败均拒绝建会话。外部回调临时 cookie 为短时 HttpOnly、SameSite=Lax；正式会话仍为原有 HttpOnly、SameSite=Strict 和 CSRF 双提交。
飞书接口以官方[授权码](https://open.feishu.cn/document/authentication-management/access-token/obtain-oauth-code)、[用户 token](https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/authentication-management/access-token/get-user-access-token-v3)与[用户信息](https://open.feishu.cn/document/server-docs/authentication-management/login-state-management/get)文档为准；部署前需在受控环境核对应用权限和回调配置。

## 绑定、试登录与关闭本地入口

在服务所在本机、以可读取数据库的受控系统账号运行命令；`ROLLBOOK_DB_PATH` 指向运行中的数据库。不要把秘密放到命令行。先备份完整数据库与配置：

```sh
rollbook auth user-create <清册登录名>
rollbook auth bind ldap <目录实例名> <稳定主体值> <清册登录名>
rollbook auth bind oidc <完整issuer> <sub> <清册登录名>
rollbook auth bind feishu <App ID> <open_id> <清册登录名>
```

可在认证设置页新增外部账号、启停用户、保存显式绑定及允许标记，也可使用上述本机命令。可绑定已有本地用户，形成混合账号；纯外部用户的 `password_hash` 为 `NULL`。`bind` 默认允许；本机调整允许名单使用 `rollbook auth allow <类型> <实例> <主体> <清册登录名> true|false`。禁用或重新启用清册用户使用 `rollbook auth user-enable <清册登录名> true|false`。这些变更在同一数据库事务中撤销受影响用户的全部清册浏览器会话，并记入 `auth_audit`。OIDC 的实例必须是完整 issuer；飞书实例必须是 App ID。邮箱、显示名、目录登录名都不是主体键。
若本地入口已关闭，管理命令会拒绝移除最后一个已试登录的外部入口；需要先执行本机恢复命令。禁用最后一个清册用户也会被拒绝。

使用独立浏览器窗口真实试登录目标外部账号，核对显示的清册登录名及实际权限，确认 `identity_bindings.verified_at` 已写入。连通测试不写这个时间，也不能作为关闭本地入口的依据。只有至少一个当前已启用身份源下、绑定允许、用户启用且在过去一小时内完成真实试登录的外部绑定存在时，页面开关或本机命令才允许关闭本地入口：

```sh
rollbook auth local-login disable
```

关闭时撤销全部本地浏览器会话。**恢复通道**是同一台服务器上的公开命令，不存在网页后门：

```sh
rollbook auth local-login enable
rollbook reset-password <已有清册登录名>
```

第二条命令在交互终端读新口令，适用于需要重置或给纯外部账号补建本地凭据；会撤销该用户所有浏览器会话。恢复命令及绑定/名单/账号变更写入 `auth_audit`，管理员应通过主机访问控制和日志限制谁能操作数据库。`ROLLBOOK_AUTH_PEPPER` 与所有 IdP secret 不得进入仓库、备份说明或前端。

## 会话、失效与回滚

`GET /api/auth/session` 保留 `login`，增加 `identity_type`（`local`、`mixed`、`external`）、`auth_provider` 和 `can_change_password`。只有本地口令存在且本地入口开放时才能在清册改密；纯外部账号应到上游平台管理密码。清册“退出登录”只撤销当前浏览器会话，“退出所有设备”撤销该用户全部清册浏览器会话；两者均不退出上游 SSO，也不撤销 API Key。Key 的 `read/propose/write` 权限及禁用用户对 Key 的影响保持不变。

清册能在**下一次登录**发现上游拒绝登录、LDAP bind 失败或上游 token/用户接口失败；已签发的清册会话不会实时查询上游。没有上游事件或轮询源时，离职、冻结等状态**无法即时感知**，既有会话最迟在 12 小时有效期到达时失效。管理员可立即用 `auth allow ... false` 或 `auth user-enable ... false` 撤销对应清册会话。停用 provider 后，相关会话在下一次清册请求时被拒绝并删除。

从 `v0.9.0` 升级到 `v0.10.0` 时会执行前向迁移 `017_auth_providers.sql`，新增页面身份源配置表（含密文）及 `auth_pending.config_version`。升级前停服务，保存完整 `/data` 数据卷文件级快照（SQLite 数据库与 `-wal`、`-shm` 一起保存），独立使用 `/backups` 时也保存该卷；保留原 `ROLLBOOK_AUTH_PEPPER` 或其 `_FILE` 文件、身份源环境变量或 `_FILE` 文件及必要密钥材料。完整 YAML 不能代替快照，也不能恢复页面凭据；全量 YAML 导入会清除目标库页面身份源配置，并拒绝本地入口已关闭的 YAML。回退须停服务，恢复升级前完整快照、配置和密钥，再启动 `v0.9.0` 正式镜像，不能让旧镜像直接读取迁移后的数据库。若原 pepper 遗失，页面密文不可解，须先用本机 `rollbook auth local-login enable` 保留恢复路径，再重新填写页面凭据。旧版完整 YAML 可导入新版库，缺省外部绑定为空且本地入口开放；不要把新库备份直接恢复到旧版。

## 从 v0.10.0 升级到 v0.11.0

相对 `v0.10.0` 无新增数据库迁移、无密钥方案变更。升级前停服务，保存完整 `/data` 文件级快照（SQLite 数据库、`-wal`、`-shm`），独立使用 `/backups` 时也保存该卷；保留原环境变量、`ROLLBOOK_AUTH_PEPPER` 或其 `_FILE` 文件、身份源环境变量或 `_FILE` 文件及认证配置加密所需密钥材料，并在隔离环境验证恢复。完整 YAML 不能代替快照，也不含页面身份源凭据。确认正式 tag、Release 与镜像后方可升级。回退时停服务，恢复完整升级前快照、配置与密钥，再启动 `v0.10.0` 正式镜像；不能只切换旧镜像读取升级后的数据。原 pepper 丢失或更换时，页面密文无法解密，须先在本机执行 `rollbook auth local-login enable` 保留本地恢复通道，再重新填写页面凭据。真实 LDAP、OIDC、飞书 IdP 联调尚未进行；没有新增全站 RBAC 或群组映射。
