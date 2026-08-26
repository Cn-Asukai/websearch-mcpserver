# T03 MinerU 精准 API 仅用于 PDF URL

- Issue：[#2](https://github.com/daidaiJ/websearch-mcpserver/issues/2) P1-1
- 优先级：P1
- 状态：完成
- 依赖：无
- 方案：[plans/wave-2-p1.md](../plans/wave-2-p1.md)

## 要改什么

`pkg/webfetch/webfetch.go` `Fetch` 远程分支：

- 现有：`HasToken()` → 任意 URL `ParseURL`
- 改为：仅当 URL path（去 query/fragment）以 `.pdf` 结尾（大小写不敏感）才 `ParseURL`
- `file://` 本地 PDF 逻辑不动
- 不新增配置项，除非后续需要关闭远程 MinerU（默认保持 PDF 增强）

## 验收

- 单测：有 Token + HTML URL → mock/spy 显示 `ParseURL` 0 次
- 单测：有 Token + `.PDF?x=1` → `ParseURL` 1 次
- `go test ./pkg/webfetch/...`
