# T11 WinHTTP 代理字符串按缓冲区边界读取

- Issue：[#2](https://github.com/daidaiJ/websearch-mcpserver/issues/2) P2-6
- 优先级：P2
- 状态：完成
- 依赖：与 [T10](T10-proxy-transport-cache.md) 同 PR
- 方案：[plans/wave-3-p2.md](../plans/wave-3-p2.md)

## 要改什么

`pkg/proxy/sysproxy_windows.go` `proxyFromWinHTTP`：删除 `(*[512]uint16)(...)[:256:256]`。从 `info.Proxy` 起在 `buf` 范围内读 UTF-16 至 NUL。

抽出纯函数（输入 `[]byte` + 指针偏移或 `[]uint16`）方便测试，避免在 CI（linux）上跑 WinHTTP。

## 验收

- 纯函数测试：短字符串、刚好填满、无 NUL 时不越界 panic
- `go test ./pkg/proxy/...`
