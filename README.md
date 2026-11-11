# Rollbook 清册

Rollbook 是面向家庭与小企业的自建服务和资产清册。请只在可信内网运行。

## 本地运行

需要 Go 1.24 或更新版本。

```sh
go test ./...
export ROLLBOOK_AUTH_PEPPER='<在部署环境中设置至少 32 字节的随机值>'
go run ./cmd/rollbook bootstrap-user owner
go run ./cmd/rollbook serve
```

默认监听 `127.0.0.1:8080`，数据库写入 `./data/rollbook.db`。`bootstrap-user` 只允许创建首个用户，密码从终端读取。访问 `/` 可打开清册网页，`/healthz` 返回 `ok`。网页登录的会话 Cookie 带 `Secure`，实际访问需要 HTTPS 反向代理。环境变量 `ROLLBOOK_LISTEN_ADDR` 和 `ROLLBOOK_DB_PATH` 可修改监听地址与数据库路径。首次启动自动迁移；再次启动不会重复执行已应用的迁移。数据库使用 WAL、外键约束和 FTS5。

REST `/api/` 与 Streamable HTTP `/mcp` 使用 `Authorization: Bearer <key>`；网页使用用户会话。启动服务须设置至少 32 字节的 `ROLLBOOK_AUTH_PEPPER`，该值应保存在部署环境中，不要提交到仓库。网页可搜索、浏览站点/主机/网段、查看服务和两层拓扑、审核提议，并管理 key。key 生成后只显示一次。MCP 支持 `initialize`、`tools/list`、`tools/call` 的 JSON 响应模式，工具调用与 REST 共用应用服务和权限检查。`check_versions` 当前返回 `not_configured`；上游检查任务将在版本检查阶段接入。

CLI 通过 `ROLLBOOK_URL`（默认 `http://127.0.0.1:8080`）和 `ROLLBOOK_KEY` 连接同一 REST API：

```sh
rollbook search Overleaf
rollbook get service:overleaf
rollbook update service:overleaf 1 '{"name":"Overleaf 2"}' '修正名称'
```

## 容器运行

```sh
docker build -t rollbook:local .
docker run --rm -p 127.0.0.1:8080:8080 -v rollbook-data:/data rollbook:local
```

容器内使用 `/data/rollbook.db`，监听 8080。示例只把端口发布到宿主机回环地址。实际部署应由内网反向代理接入，并持久化 `/data`。

GitHub Actions 在 PR 上运行测试、vet 和镜像构建；推送 main 后使用仓库自带的 `GITHUB_TOKEN` 发布 `ghcr.io/<owner>/<repo>:latest`。工作流不部署服务。
