# T01 默认绑定 127.0.0.1，业务端点可选 token

- Issue：[#2](https://github.com/daidaiJ/websearch-mcpserver/issues/2) P0-1
- 优先级：P0
- 状态：完成
- 依赖：无
- 方案：[plans/wave-1-p0.md](../plans/wave-1-p0.md)

## 要改什么

1. `pkg/config/config.go` 增加：
   - `Host string`（yaml `host`，默认 `127.0.0.1`）
   - `AuthToken string`（yaml `auth_token`，env `WEBSEARCH_TOKEN`）
   - `Load`/`Default`/`applyKnownEnv` 接上默认值和 env
2. `server/server.go`：`net.JoinHostPort(host, port)`；日志打完整 addr
3. `/mcp` 与 `/searxng/search` 加 Bearer / `X-API-Key` 中间件；token 为空则跳过
4. `__admin/health` 保持无鉴权；其余 admin 继续 `localOnlyMiddleware`
5. `host` 为 `0.0.0.0`/`::` 且无 token 时启动打 Error 警告
6. 同步 `config.example.yaml`、`pkg/config/config.example.yaml`、`docs/configuration.md`、`docs/configuration.en.md`、`docs/installation.md`（MCP headers 示例）

## 不要改

- stdio MCP
- health 探活协议

## 验收

- 默认监听 `127.0.0.1:8338`，局域网其它机器 SYN 不到
- `auth_token` 有值时无 header 访问 `/mcp`、`/searxng/search` → 401
- 本机 `start`/`stop`/`status` 仍走通
- `go test ./pkg/config/... ./server/... ./mcp/... ./searxng/...`（若无现成 HTTP 测试，补中间件单测即可）
