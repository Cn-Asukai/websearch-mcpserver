# Wave 3 — P2：正确性、可观测、性能、文档

> 对应任务：T06–T13。建议拆 2～3 个 PR：Key/编排、proxy、文档测试。

---

## P2-1 KeyPool.MarkLastInvalid 并发标错 key

**现状：** `lastKey` 是池级 `atomic.Value`（`keypool.go:22`）。A `Next()`→key1，B `Next()`→key2，A 失败 `MarkLastInvalid()` 会冷却 key2。调用点 `apipool.go:100`。`MarkInvalid(key)` 已存在。

**方案：** `callProviderWithRetry` 在 `callSingle` 前 `key := p.pool.Next()` 不够——当前引擎内部自己 `keys.Next()`。正确修法：

1. `callSingle` 失败时从引擎取出「本次使用的 key」，或
2. 更干净：搜索适配器 `SearchRaw` 失败返回包装错误（含 key），apipool `errors.As` 后 `MarkInvalid(key)`。
3. 若不想改错误类型：让 `KeyPool.Next()` 返回 key，调用方持有局部变量，失败时 `MarkInvalid(used)`。这意味着 baidu/tavily/exa 要把「本次 Next 的 key」暴露给 apipool。

推荐最小改动：

```go
// 适配器 SearchRaw 内：
key := t.keys.Next()
res, err := client.DefaultClient.R().SetHeader(..., key)...
if err != nil || res.StatusCode() != 200 {
    return nil, &KeyError{Key: key, Err: err}
}
```

apipool：

```go
var ke *KeyError
if errors.As(err, &ke) {
    p.pool.MarkInvalid(ke.Key)
} else if p.pool != nil {
    p.pool.MarkLastInvalid() // 免费引擎无 pool
}
```

`MarkLastInvalid` 可留作单线程兼容，文档标明非并发安全；新代码禁止再用。补 `keypool` / `apipool` 并发单测：两个 goroutine Next+失败，断言只冷却自己的 key。

---

## P2-2 双 SearchGroup：限流翻倍、Key 状态分裂

**现状：** `server/server.go:131-141`（`Handler()` `:214-225` 同样）：`Init(WithSearchEngine)` 已 `NewFromConfig` 一次，接着再 `search.NewFromConfig` 给 searxng。error 被 `_` 丢掉；`searxng.Init` 在 `group.Primary == nil` 时只打日志，`handlerSearch:23` 会对 nil 接口 panic。

**方案：**

- `mcp` 包保存 `*search.SearchGroup`（或至少 `Primary`），导出 `GetSearchGroup()` / `GetPrimary()`。
- `server.Run` / `Handler`：`searxng.Init(mcpserver.GetSearchGroup())`，删掉第二次 `NewFromConfig`。
- `searxng.handlerSearch`：`defaultInf == nil` 或 query 空 → `503` / `400`，禁止 panic。
- `Init` 传入 nil group 时保持现状日志，handler 守卫兜底。

---

## P2-3 单引擎结果被默认截到 4 条

**现状微调（issue 略绝对化）：**

`postSearchFilter`（`mcp/tool.go:437-478`）对 hybrid 以外都跑。`engineMax` 来自 `smartSearchConf.Engines[engineName]`；缺省 `MaxSize==0` 则用 `defaultEngineMaxSize=4`（`inf.go:14`）。

| 模式 | `Name()` | 有无 engines 配置 | 实际 |
|------|----------|-------------------|------|
| tavily | `tavily_api` | 有则用配置 | 与 example 对齐 |
| exa | `exa` | 同上 | 同上 |
| baidu | `baidu_api` | 同上 | 同上 |
| baidu AI | `baidu_ai` | example **没有** 这一项 | 落到 4 |
| apipool | `apipool` | example **没有** | 落到 4，**覆盖** `factory.go:205-207` 的 `SetMaxSize(SmartSearch.MaxSize)` |

所以「apipool.maxSize 永不生效」成立；「所有单引擎都被截 4」只在 yaml 没写 `smartsearch.engines` 时成立。

**方案：**

```go
engineMax := ec.MaxSize
if engineMax <= 0 {
    // 无 per-engine 配置：不要默默截 4，交给全局 maxSize / 引擎自己的截断
    engineMax = 0
}
```

全局 `smartSearchConf.MaxSize > 0` 时仍截断。apipool 内部 `SetMaxSize` 继续生效。删掉 tool 层对「未知引擎名」的魔法 4；`defaultEngineMaxSize` 仅留给 hybrid 里真正缺省的 per-engine 过滤（核对 `hybrid.go` 是否也用了 4，保持两边语义一致）。

补单测：`postSearchFilter` + 空 engines map + 10 条结果 + `MaxSize=10` → 保留 10；`Name=apipool` 同理。

---

## P2-4 多引擎失败静默

**现状：** `hybrid.go:129-134` `continue` 吞掉 `r.err`，无日志。`pkg/bing`、`pkg/google`、`pkg/ddg` 实现内 0 处 `log` 调用。全挂时只返回「所有搜索引擎均失败」。

**方案：** 只在编排层打日志，不要给每个引擎塞 log（避免噪音 + 破坏 antirobot 层边界）：

```go
if r.err != nil {
    log.Warnf("hybrid: engine %s failed: %v", h.engineNames[r.index], r.err)
    continue
}
```

全失败时 error 可附带各引擎错误摘要（注意不要把 API Key 打进日志）。

---

## P2-5 每请求 Clone transport + 注册表放大

**现状：** `proxy/proxy.go:22-41` 每次 `RoundTrip` `base.Clone()`，连接池失效。自动代理模式 `ProxyResolver()`（`config.go:270-271`）每次调用 `DetectSystemProxy()` → Windows 开注册表（`sysproxy_windows.go:37-60`）。

**方案：**

1. `dynamicProxyTransport` 按「当前 endpoint 字符串」缓存 `*http.Transport`（`sync.Map` 或 mutex+map）。endpoint 变了才新建。无代理也缓存一条 `Proxy: nil` 的 transport，不要每请求 Clone。
2. `DetectSystemProxy` 加 5s～30s TTL 内存缓存（`atomic` + `time.Time`）。环境变量变化不要求即时，可接受 TTL 延迟。

与 T11 同一 PR。

---

## P2-6 proxyFromWinHTTP 定长强转越界

**现状：** `sysproxy_windows.go:200`：`(*[512]uint16)(unsafe.Pointer(info.Proxy))[:256:256]`，而 buf 按 `bufLen` 分配。`INTERNET_PROXY_INFO.Proxy` 是指向 buf 内 UTF-16 的指针，定长 512 可能读出分配区。

**方案：** 在 `buf` 切片范围内从 `info.Proxy` 扫到 NUL：

```go
func utf16PtrToString(p *uint16, buf []byte) string {
    if p == nil {
        return ""
    }
    end := unsafe.Pointer(unsafe.SliceData(buf)[len(buf):]) // 或用 uintptr 算剩余 uint16 个数
    // 逐个 uint16 直到 0 或越出 buf
}
```

实现时用 `unsafe.Pointer` 算术 + 边界比较，禁止魔法 512。无 WinHTTP 单元测试环境就加纯函数测试：给定一段 UTF-16 bytes 转 string。

---

## P2-7 viper env 对 YAML 缺失 key 静默失效

**现状纠正：** `applyKnownEnv`（`config.go:632-651`）已经写好，但 **只在 `Default()` 调用**。`Load()` Unmarshal 之后没有回填。因此：

- 零配置 `LoadOrDefault` → Default → env 生效。
- 精简 `config.yaml`（没有 `tavily.api_key` 字段）→ `TAVILY_SK` 仍可能被忽略。

**方案：** `Load()` 在 Unmarshal 和默认值处理之后 `applyKnownEnv(&conf)`。加测试：临时 yaml 只有 `port: 8338`，设 `TAVILY_SK`，断言 `conf.Tavily.APIKey`。

与 T04 同一 PR 最省事。

---

## P2-8 国内网络可用性未标注；部分集成测试仍直连

**现状纠正：** 下列已有 `testing.Short()`：`google_test.go`、`baidu_test.go`、`tavily_test.go`、`exa_test.go`、`baidu_api_test.go`。下列 **没有**：`pkg/bing/bing_test.go` 的 `TestBingSearch`、`pkg/academic/engines_test.go` 的 `TestCrossrefSearch` / `TestArxivSearch`、`google_scholar_test.go`。

**方案：**

1. 所有打真实外网的 `Test*Search` 统一：

```go
if testing.Short() {
    t.Skip("skipping network integration test")
}
```

本地快速回归用 `go test -short ./...`；全量仍可 `go test ./...`。不为此加 GitHub Actions。

2. README 模式表 / `docs/search.md` 加一行网络说明：Google / DDG / Crossref / Google Scholar 在 `network: china` 且无代理时不稳定；Bing 网页抓取亦可能超时。不要承诺「国内开箱全绿」。

`config.go:124` 已有「被反爬拦截暂不可用」类注释的，同步到用户文档即可。
