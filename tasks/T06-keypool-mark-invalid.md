# T06 Key 失效按实际使用的 key 标记

- Issue：[#2](https://github.com/daidaiJ/websearch-mcpserver/issues/2) P2-1
- 优先级：P2
- 状态：完成
- 依赖：[T02](T02-upstream-timeout.md)（无超时的 SK 重试会放大挂起）
- 方案：[plans/wave-3-p2.md](../plans/wave-3-p2.md)

## 要改什么

1. 给搜索 API 错误加可取出的 `Key`（例如 `pkg/search` 内 `KeyError`）
2. `baidu` / `baidu_ai` / `tavily` / `exa` 在 `keys.Next()` 后把该 key 写入失败错误
3. `apipool.callProviderWithRetry`：`MarkInvalid(ke.Key)`，停止依赖 `MarkLastInvalid`
4. `MarkLastInvalid` 保留但注释「非并发安全，新代码勿用」
5. 并发单测：两 goroutine 交错 Next + 失败，断言冷却的是自己的 key

## 验收

- `go test -race ./pkg/search/...` 新并发测试通过
- 单 key 路径行为不变
