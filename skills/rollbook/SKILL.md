---
name: rollbook
description: 查询和维护 Rollbook 清册中的站点、主机、IP、服务、入口、依赖与运维笔记。当用户问资产位置、服务依赖、版本、凭据引用或要求登记/修改资产时使用。
---

# Rollbook 清册

使用已配置的 Rollbook MCP 工具。清册是资产事实的记录，外部探测结果仍需核实。用户问“在哪台主机”“某 IP 属于谁”“服务入口/依赖是什么”时，先查清册；用户要求登记或纠正时，先搜索避免重复。

## 查询

1. `search` 搜名称、地址、域名或别名；根据结果 ID 调 `get_asset`。服务用 `get_service` 查看宿主、入口、依赖、版本；站点用 `get_site`；主机或网段用 `list_by_host`、`list_by_network`。
2. 结果为空就明确说明清册未记录；不要凭空补造资产。注明数据的来源、核实时间与不确定性。
3. `get_changes` 可查历史和待确认提议；`list_outdated` 的“待人工确认”不是已知升级版本。

## 写入与提议

1. 先 `search`，再 `get_asset` 读取当前 `rev` 和关系 ID。新建时按 `location → network/host → ip/service → endpoint/dependency` 的顺序建立引用对象。对象 ID 用 `type:唯一名称`，创建传 `object`、`base_rev: 0`、`reason`。提议级 key 调 `create` 会生成待确认提议，用户须在网页「待确认」批准后才生效。
2. 更新时提交 `id`、刚读取的 `rev`、只含要改字段的 `patch` 和 `reason`。提议级 key 用 `propose_change`；直接写 key 用 `update`。`record_note` 和 `mark_verified` 也需要目标最新 rev 与 reason。
3. `reason` 写可供审核的事实依据，例如“根据现场核对，主机已迁至机房 B”；不要只写“更新”或“修复”。提议描述要说明影响，等待网页用户决定。
4. 遇到 rev 冲突，重新 `get_asset`，比较他人的变更，再请用户决定或基于新 rev 重做；绝不自动覆盖。权限不足时停止写入，不尝试换工具绕过。

## Key 与秘密

- `read` 只读；`propose` 可提交待确认的新建和修改；`write` 可直接写入。审批仍由网页用户完成。
- 不将密码、私钥、token、连接字符串等秘密写进清册、笔记、reason 或日志。凭据放在用户的密码管理器或密钥库，只在 `credential` 对象中记录 `vault` 与 `item_ref` 等引用，不记录明文。示例：`{"type":"credential","id":"credential:demo","name":"数据库凭据引用","fields":{"target_id":"service:demo","vault":"家庭密码库","item_ref":"数据库/演示","purpose":"管理员访问"}}`。
- MCP 使用 `Authorization: Bearer <key>`。Key 只显示一次；应放在本机受保护的客户端配置或环境变量中，不提交到仓库。

## 英文枚举别名

REST 和 MCP 的创建、提议与修改入口接受英文别名；响应、审计和存储仍使用中文规范值。常用别名：`status` 的 `active`/`inactive`/`planned` 对应在用/停用/计划中；`host.fields.kind` 的 `physical`/`vm`/`virtual`/`container`/`network_device` 对应物理机/虚拟机/虚拟机/容器/网络设备；`location.fields.kind` 的 `site`/`room`/`rack`/`desk` 对应站点/房间/机柜/桌面；`endpoint.fields.kind` 的 `direct`/`reverse_proxy`/`cloudflare_tunnel`/`navigation_card` 对应内网直连/反向代理/Cloudflare Tunnel/导航页卡片。`confidence` 可用 `low`/`medium`/`high`；`assign` 可用 `static`/`dhcp_reservation`/`dynamic`。完整可用值以 MCP `tools/list` Schema 为准。原有中文值继续有效。

REST 可通过 `Accept-Language: en` 或 `X-Rollbook-Language: en` 请求英文错误提示；后者优先。错误 `code` 和 HTTP 状态不变。
