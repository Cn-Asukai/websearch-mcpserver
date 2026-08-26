# T13 外网集成测试可跳过，文档标明网络限制

- Issue：[#2](https://github.com/daidaiJ/websearch-mcpserver/issues/2) P2-8
- 优先级：P2
- 状态：完成
- 依赖：无（不做远程 CI 门禁；`-short` 仍方便本地快速跑单测）
- 方案：[plans/wave-3-p2.md](../plans/wave-3-p2.md)

## 背景（issue 部分过时）

已有 `testing.Short()`：`pkg/google/google_test.go`、`pkg/baidu/baidu_test.go`、`pkg/search/tavily_test.go`、`exa_test.go`、`baidu_api_test.go`。

仍直连、无 skip：

- `pkg/bing/bing_test.go` → `TestBingSearch`
- `pkg/academic/engines_test.go` → `TestCrossrefSearch`、`TestArxivSearch`（及文件内其它真实 HTTP 测试）
- `pkg/academic/google_scholar_test.go`

不要再写「google_test.go 无跳过」——已过时。

## 要改什么

1. 上述测试函数开头加 `if testing.Short() { t.Skip(...) }`
2. README / `docs/search.md`（及 `.en.md`）注明：Google、DDG、Crossref、Google Scholar 在国内直连不稳定，需代理或 `network` 配置；Bing 抓取也可能超时

## 验收

- `go test -short ./...` 不发起这些外网请求
- 文档有对应句子
