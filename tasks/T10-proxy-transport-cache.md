# T10 代理 transport 按端点缓存，系统代理检测加 TTL

- Issue：[#2](https://github.com/daidaiJ/websearch-mcpserver/issues/2) P2-5
- 优先级：P2
- 状态：完成
- 依赖：与 [T11](T11-winhttp-utf16-bounds.md) 同 PR
- 方案：[plans/wave-3-p2.md](../plans/wave-3-p2.md)

## 要改什么

1. `pkg/proxy/proxy.go` `dynamicProxyTransport`：按 endpoint 缓存 `*http.Transport`，禁止每请求 `Clone`
2. `DetectSystemProxy`（`sysproxy.go`）：进程内 TTL 缓存（建议 10s）
3. 单测：同一 endpoint 连续 RoundTrip 复用同一 transport 指针（可小范围导出或测试 hook）

## 验收

- `go test ./pkg/proxy/...`
- 自动代理模式下连续请求不再每发一次就读注册表（可用 TTL 测试：两次调用间隔 < TTL 只探测一次）
