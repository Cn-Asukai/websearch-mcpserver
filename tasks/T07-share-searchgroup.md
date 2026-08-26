# T07 MCP 与 SearXNG 共用一个 SearchGroup

- Issue：[#2](https://github.com/daidaiJ/websearch-mcpserver/issues/2) P2-2
- 优先级：P2
- 状态：完成
- 依赖：无
- 方案：[plans/wave-3-p2.md](../plans/wave-3-p2.md)

## 要改什么

1. `mcp` 在 `applySearchEngine` 保存 `*search.SearchGroup`，导出 getter
2. `server.Run` 与 `server.Handler` 删除第二次 `search.NewFromConfig`，改为 `searxng.Init(mcpserver.GetSearchGroup())`
3. `searxng/handler.go`：`defaultInf == nil` → 503；`q` 为空 → 400
4. `Init` 仍忽略空 Primary 并打日志，由 handler 守卫兜底

## 验收

- 进程内只有一次 `NewFromConfig`（可用日志或测试 Hook 确认）
- 无引擎时打 `/searxng/search?q=test` 不 panic
- `go test ./server/... ./searxng/... ./mcp/...`
