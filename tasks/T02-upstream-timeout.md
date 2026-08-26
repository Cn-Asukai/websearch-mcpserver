# T02 API 上游默认 30s 超时

- Issue：[#2](https://github.com/daidaiJ/websearch-mcpserver/issues/2) P0-2
- 优先级：P0
- 状态：完成
- 依赖：无（T06 应在本任务之后）
- 方案：[plans/wave-1-p0.md](../plans/wave-1-p0.md)

## 要改什么

1. `pkg/client/client.go`：`resty.New().SetTimeout(30 * time.Second)`
   - 超时时间做成包级变量或 `New(timeout)`，方便测试注入，避免单测真等 30s
2. 可选小改：`pkg/llm/llm.go` 的 `Chat` 增加 `ctx` 并 `SetContext`（`ChatStream` 已有）
3. **不要** 本轮改造 `SearchInf` 加 context

涉及调用方（只需间接受益，通常不用改）：`baidu.go` / `baidu_ai.go` / `tavily.go` / `exa.go` / `llm.go` 非流式。

## 验收

- 用 `httptest` 阻塞不响应的 server，注入 200ms timeout，断言错误为 timeout 且耗时 < 1s
- `go test ./pkg/client/... ./pkg/llm/...`
- 现有搜索单测不被拖死（集成测试本身应 `-short` skip）
