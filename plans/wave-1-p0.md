# Wave 1 — P0：默认本机监听 + 上游超时

> 对应任务：[T01](../tasks/T01-bind-and-auth.md) [T02](../tasks/T02-upstream-timeout.md)
> 目标：局域网不能白嫖付费配额；上游黑洞不能把 MCP 工具永久挂起。

---

## P0-1 HTTP 监听所有网卡且业务端点无鉴权

### 现状（已核实）

- `server/server.go:154-155`：`Addr: fmt.Sprintf(":%d", conf.Port)` → 绑 `0.0.0.0` / `::`。
- 仅 `__admin/refcount|status|shutdown` 走 `localOnlyMiddleware`（`:50-62`）。
- `__admin/health` 故意裸奔（daemon 探活需要），保持即可。
- `/mcp`（`mcp/server.go:102`）和 `GET /searxng/search`（`searxng/handler.go:42`）无鉴权。
- `config.Config` 无 `host` / `listen` / token 字段。

影响属实：服务若跑在局域网可达机器上，任意主机可打 MCP / SearXNG / cleanfetch，烧掉 Tavily/Exa/千帆配额。

### 方案（两道闸，默认关外网）

**闸 1：默认只绑回环。**

```go
// config
Host string `mapstructure:"host"` // 默认 "127.0.0.1"；"0.0.0.0" 才对所有网卡开放
```

`server.Run`：

```go
host := conf.Host
if host == "" {
    host = "127.0.0.1"
}
Addr: net.JoinHostPort(host, strconv.Itoa(conf.Port))
```

`0.0.0.0` / `::` / 空以外的值按用户配置。日志打实际 `addr`，不要再写 `:%d`。

**闸 2：可选 Bearer token，覆盖业务端点。**

```go
AuthToken string `mapstructure:"auth_token"` // 空 = 不鉴权；环境变量 WEBSEARCH_TOKEN
```

中间件挂在 `/mcp` 和 `/searxng/search`：

- `Authorization: Bearer <token>` 或 `X-API-Key: <token>` 任一通过。
- `__admin/*` 继续用 `localOnlyMiddleware`，不加 token（本机 daemon 协议保持简单）。
- `host` 为 `0.0.0.0`/`::` 且 `auth_token` 为空时：启动打 **Error 级警告**（不直接拒绝启动，避免已有局域网部署突然起不来）。文档写清「开放网卡时强烈建议配 token」。

MCP 客户端配置要在 `docs/installation.md` 补 headers 示例。

### 不做

- 不给 health 加鉴权（`start`/`stop`/`status` 会坏）。
- 不上 mTLS / OAuth。
- 不改 stdio MCP 传输（无 HTTP 暴露面）。

---

## P0-2 API 上游无超时

### 现状（已核实）

- `pkg/client/client.go:8-10`：`resty.New()`，未 `SetTimeout`。
- 无超时调用：`pkg/llm/llm.go:70`（非流式 Chat）、`pkg/search/baidu.go:110`、`baidu_ai.go:128`、`tavily.go:103`、`exa.go:113`。
- 对照：`ChatStream` 已 `SetContext(ctx)`（`llm.go:111`）；MinerU 客户端已 60s timeout；Bing 引擎 15s。说明是 API 路径遗漏。
- `SearchInf.SearchRaw(query string)` **没有 ctx**，MCP 取消传不进 API 调用。apipool 失败会循环 `MarkLastInvalid` + 下一个 SK（`apipool.go:95-106`），每个 SK 都可能无限挂起。

### 方案（先止血，后可选改造）

**本 Wave 必做：**

```go
// pkg/client/client.go
func init() {
    DefaultClient = resty.New().SetTimeout(30 * time.Second)
}
```

30s 覆盖单次 API RTT；apipool 多 SK 最坏约 `N * 30s`，可接受。可用 `WEBSEARCH_UPSTREAM_TIMEOUT` 或 `config` 项覆盖，没有强烈需求就先写死 30s + 注释。

**本 Wave 建议顺手：**

- `llm.Chat` 增加 `ctx` 参数并 `SetContext`（调用方 MCP 已有 ctx）。若改签名影响面小，一并做。
- 搜索 API 适配器暂不改接口。

**明确不做（避免 P0 膨胀）：**

- 不把 `SearchInf` 改成 `SearchRaw(ctx, query)`。那是独立重构，涉及 hybrid/apipool/全部适配器。

### 测试

- `pkg/client`：用 `httptest` 不响应的 handler，断言 30s 内返回 timeout（测试里把 timeout 调到 200ms 或给 client 提供可注入 timeout）。
- 现有 baidu/tavily/exa 单测保持 `-short` 可 skip。
