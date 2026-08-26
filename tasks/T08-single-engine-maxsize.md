# T08 单引擎模式不再默默截成 4 条

- Issue：[#2](https://github.com/daidaiJ/websearch-mcpserver/issues/2) P2-3
- 优先级：P2
- 状态：完成
- 依赖：无
- 方案：[plans/wave-3-p2.md](../plans/wave-3-p2.md)

## 背景（按代码，不是按 issue 字面）

`postSearchFilter` 在引擎名不在 `smartsearch.engines` 时用 `defaultEngineMaxSize=4`。`apipool` 的 `Name()` 是 `"apipool"`，example 里没有该项，因此 `factory.go` 里 `SetMaxSize(SmartSearch.MaxSize)` 会被 tool 层再截成 4。`baidu_ai` 同理。tavily/exa/baidu_api 在 yaml 写了 engines 时正常。

## 要改什么

`mcp/tool.go` `postSearchFilter`：per-engine `MaxSize<=0` 时 **不要** 回落到 4，只应用全局 `MaxSize`。核对 `hybrid.go` 缺省 4 的语义，两边对齐或在注释写清「仅 hybrid 缺省 / 仅显式配置」。

补 `mcp/tool_test.go`：空 engines + 10 条 + 全局 max 10 → 10 条；engineName=`apipool` 同理。

## 验收

- `go test ./mcp/... ./pkg/search/...`
- mode=apipool 且 `smartsearch.max_size: 10` 时结果可超过 4 条（单测即可）
