# 外部身份登录（未发布）

本功能默认只有本地账号入口。LDAP、OIDC、飞书登录分别由服务端配置启用；不得把本文件的示例值当成真实密钥。每个网页用户仍拥有清册全部管理能力。本版没有 RBAC、自动开户、邮件/同名自动绑定，也没有群组映射。外部身份只有在本机命令预置清册用户、明确绑定稳定主体且标记允许后才能登录。推荐在受控环境先做试登录。

## 服务端配置

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

配置值只在服务端读取。`GET /api/config` 只返回各入口是否可显示与 HTTP 风险，不返回上游 URL、bind DN 或 secret。OIDC 采用发现文档、授权码、state、nonce、PKCE S256 和 ID Token 签名、issuer、audience、时效校验；飞书独立使用授权码换用户 token，再读取 `open_id`；LDAP 仅经验证证书的 LDAPS 或 StartTLS 搜索和用户 bind。所有失败均拒绝建会话。外部回调临时 cookie 为短时 HttpOnly、SameSite=Lax；正式会话仍为原有 HttpOnly、SameSite=Strict 和 CSRF 双提交。
飞书接口以官方[授权码](https://open.feishu.cn/document/authentication-management/access-token/obtain-oauth-code)、[用户 token](https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/authentication-management/access-token/get-user-access-token-v3)与[用户信息](https://open.feishu.cn/document/server-docs/authentication-management/login-state-management/get)文档为准；部署前需在受控环境核对应用权限和回调配置。

## 绑定、试登录与关闭本地入口

在服务所在本机、以可读取数据库的受控系统账号运行命令；`ROLLBOOK_DB_PATH` 指向运行中的数据库。不要把秘密放到命令行。先备份完整数据库与配置：

```sh
rollbook auth user-create <清册登录名>
rollbook auth bind ldap <目录实例名> <稳定主体值> <清册登录名>
rollbook auth bind oidc <完整issuer> <sub> <清册登录名>
rollbook auth bind feishu <App ID> <open_id> <清册登录名>
```

可绑定已有本地用户，形成混合账号；纯外部用户的 `password_hash` 为 `NULL`。`bind` 默认允许；调整允许名单使用 `rollbook auth allow <类型> <实例> <主体> <清册登录名> true|false`。禁用或重新启用清册用户使用 `rollbook auth user-enable <清册登录名> true|false`。这些变更在同一数据库事务中撤销受影响用户的全部清册浏览器会话，并记入 `auth_audit`。OIDC 的实例必须是完整 issuer；飞书实例必须是 App ID。邮箱、显示名、目录登录名都不是主体键。
若本地入口已关闭，管理命令会拒绝移除最后一个已试登录的外部入口；需要先执行本机恢复命令。禁用最后一个清册用户也会被拒绝。

使用独立浏览器窗口试登录目标外部账号，核对显示的清册登录名及实际权限，确认 `identity_bindings.verified_at` 已写入。只有在至少一个当前已启用且在过去一小时内完成试登录的外部绑定存在时，本机命令才允许关闭本地入口：

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

回滚到旧程序须先停服务，恢复升级前的**完整数据库快照**与当时配置，再启动旧程序。旧程序不能直接打开含新迁移的数据库。旧版完整 YAML 备份可恢复到新版库，缺省外部绑定为空且本地入口开放；恢复不保留浏览器会话。不要把新库备份直接恢复到旧版。
