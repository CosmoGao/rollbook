# 外部认证公开地址与校验

本文记录已正式发布的 [v0.13.0 Release](https://github.com/CosmoGao/rollbook/releases/tag/v0.13.0) 中 HTTP API 公开 origin 和飞书设置校验修复；产物和升级说明见[本版发布记录](release-notes-v0.13.0.md)。本轮相对 `v0.12.0` 无新增数据库迁移或密钥方案变更；正式发布不会自动部署，生产本地登录和身份源配置保持不变。本段发版后补正只进入后续 `main`，不在不可变的 `v0.13.0` tag 内。

`v0.14.0` 已正式发布，见[本版说明](release-notes-v0.14.0.md)与 [Release](https://github.com/CosmoGao/rollbook/releases/tag/v0.14.0)。飞书登录与个人授权绑定共用回调，非 443 公开端口（如 `:5443`）仍须完整保留；两流程用 `state` 与 `purpose` 隔离。本发版流程未部署；下文 `v0.13.0` 的已发布结论只适用于上一版。

认证设置页面的只读回调 URL 以浏览器 `location.host` 组成 HTTPS 地址，路径固定为 `/api/auth/{oidc|feishu}/callback`。服务端解析请求的公开 origin 后，用它检查设置保存的 `Origin`、只读回调 URL，以及 OAuth start/callback 的入口地址。直接连接以 `Host` 和 TLS 为准；只有 TCP 直接对端落入既有 `TrustedProxyCIDRs` 时，才接受单值的 `X-Forwarded-Proto`、`X-Forwarded-Host`、`X-Forwarded-Port` 或 `Forwarded` 的 `proto`、`host`。未提供的部分沿用直接连接信息。两套头冲突、多值、逗号链、无效主机或端口均拒绝。普通客户端的转发头不能覆盖直接连接信息。

代理必须**覆盖**客户端送来的转发头，并传送完整公开入口。例如外部入口为 `https://example.com:5443`，内层 HTTP 的 `Host` 为 `example.com` 时，代理须发送 `X-Forwarded-Proto: https`、`X-Forwarded-Host: example.com:5443`；也可用 `X-Forwarded-Host: example.com` 加 `X-Forwarded-Port: 5443`。`Forwarded: host=example.com:5443;proto=https` 也可用。若代理保留带端口的 Host，只需正确传 proto。必须将代理的直接地址加入 `TrustedProxyCIDRs`，只列受控代理地址，不列任意客户端网段。

缺少公开端口时，服务端不会从 `Origin`、`Referer` 或回调 URL 猜测端口。管理页面会显示浏览器地址与服务端 `public_origin` 的差异；带 Origin 的保存请求返回 `403 public_origin_mismatch` 和修复提示。若 Origin 恰好与错误的服务端地址相符，但只读回调仍不同，返回 `422 validation`，`fields` 中定位 `redirect_url`。这保留了已有直连部署行为；旧代理若未正确转发公开地址，需调整代理头才能保存。没有新增环境变量或数据库迁移。

飞书配置的 `422 validation` 使用固定原因的 `fields`：`client_id`（缺 App ID）、`secret_action`（非法动作、覆盖时缺新凭据或启用时无凭据）、`redirect_url`（回调与公开入口不符）。中英文由服务端本地化，错误不含提交的凭据值。已保存密文无法解密时仍返回 `409 credential_unavailable`，要求恢复原 pepper 或覆盖凭据。OAuth 回调失败继续跳转到 `/?auth_error=external_login_failed`。HTTPS、固定路径、state/CSRF、显式主体绑定及凭据只写不回读保持原约束。
