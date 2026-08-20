# Changelog

[English](CHANGELOG.en.md) | [中文](CHANGELOG.md)

## v3.0 — 2026-08-20

### 新增
- **stdio 纯 CLI**：独立入口 `cmd/cli`，默认在 stdin/stdout 上运行 MCP；无配置文件时使用 `mode: engine` 内存默认值
- Release 额外发布 `websearch-mcp-cli-{linux,windows,darwin}-{amd64,arm64}` 及对应 `.sha256`
- CLI 子命令：`init` 写出示例配置，`version` 显示版本；日志写 stderr，避免污染 JSON-RPC
- 文档补充 HTTP / stdio 配置差异说明（`docs/config.md`）

## v2.15.0 — 2026-08-14

### 变更
- **pdf_parser 本地优先解析**：本地 PDF 先用 PDF 库（ledongthuc/pdf via go-webfetch）提取文本，成功则直接返回；仅在无文本层（扫描件/图片型）时才考虑 MinerU
- **MinerU OCR 按需回退**：需显式开启 `pdf_parser.mineru_ocr`；未开启时错误提示可配置 OCR；开启后走 MinerU Agent/精准 API，失败保留本地解析错误信息
- **MinerU 客户端初始化条件**：由「`enabled` 或有 Token」改为「有 Token 或开启 `mineru_ocr`」，避免仅启用 pdf_parser 就对每个本地文件优先上传 MinerU
- `PDFParserConfig` 新增 `MinerUOCREnabled()`；工具描述与配置示例同步说明新策略

### 测试
- 新增 `needsOCRFallback`、MinerU OCR 初始化条件、无 OCR 时错误提示等单元测试

## v2.14.0 — 2026-08-02

### 新增
- **智能相关性评分管线（Wigolo）**：多引擎结果采用 RRF（Reciprocal Rank Fusion, K=60）融合排名，叠加词汇对齐、稀有词/短语连续匹配、域名品质惩罚、多引擎共识 / 权威站 / 时效性加分，纯启发式无需 AI 模型
  - 新增 `enhance.go`（RRF / ConsensusBoost / ApplyScoreFloor）、`enhance_domain.go`（DomainQuality / AuthorityBoost）、`enhance_text.go`（LexicalAlignment / RareTermsFactor）、`enhance_intent.go`（意图分类）
- **MMR 多样性重排**：评分流水线阀值过滤之后、maxSize 截断之前执行贪心 MMR 重排（Token Jaccard 相似度），打散同一话题的高相似结果（转载站/镜像站/同源博客），Top-1 保护
  - 新增 `mmr.go`（ApplyMMR）、`enhance_text.go` 导出 `JaccardSimilarity`
  - 配置项：`smartsearch.mmr.enabled`（默认 true）、`lambda`（默认 0.7）、`target_count`（默认 0）
- **学术搜索评分增强**：六大学术引擎结果采用 RRF 融合排名，叠加学术特有信号——引用数（对数压缩，clamp [1.0, 1.7]）、高影响力期刊/会议加分、PDF 全文可用性、新鲜度因子（时间敏感查询时近 1 年 ×1.15）；低分论文自动过滤（Top-1 + 每引擎保底）
  - 新增 `academic_enhance.go`（EnhanceAcademicResults / CiteFactor / JournalBoost / AcademicRecencyFactor）；OpenAlex / Semantic Scholar 回传的 relevance score 参与引擎内排名
  - 配置项：`academic.enhance`（默认 true）、`academic.threshold`（默认 0.02），独立于 smartsearch（四个工具配置解耦）
- **LLM 摘要流式推送**：`smartsearch` 摘要阶段通过 MCP `notifications/progress` 逐 token 实时推送生成过程（搜索完成 → 阶段通知 → token 流），客户端断开自动取消，失败自动回退非流式摘要
  - `pkg/llm` 新增 `ChatStream`（OpenAI 兼容 SSE 流式接口）、`pkg/summarizer` 新增 `SummarizeStream`

### 变更
- **低质量上下文真正被裁剪**：修正 `ApplyScoreFloor`，移除“每引擎保底保留”逻辑，score 低于全局阈值（默认 0.05）的结果被丢弃，仅保留 Top-1（并保证最少 2 条）

### 修复
- **admin 端点不可达**：`config.Load` 未配置 `port` 时默认为 8338，避免绑定到随机端口 `:0` 导致 daemon/CLI 与 agent 无法通过 `http://127.0.0.1:{port}/__admin` 访问端点
- **百度网页搜索返回空结果**：`web_search` 请求补充千帆必填字段 `search_source: "baidu_search_v2"`；`baidu.go` 新增 HTTP 状态码检查，非 200 时显式暴露鉴权/配额/参数错误，避免误报为“内容为空”

### 测试
- 新增 `TestEnhanceFiltersLowQuality` 观测性测试，实测低质量结果被裁剪（5→2）
- 新增 MMR 测试（Top-1 保护 / 多样性打散 / λ=1 退化 / targetN 截断）与学术增强测试（引用数 / 期刊 / 新鲜度信号 / 每引擎保底）
- 新增 `ChatStream` 单元测试（httptest mock SSE 服务：正常流 / HTTP 错误 / ctx 取消 / 畸形行跳过）
- 百度实时 API 集成测试改为环境不可用（网络/鉴权/配额/空结果）时 skip，避免 `go test ./...` 因外部依赖不可用而失败

## v2.13.0 — 2026-07-19

### 变更
- **百度智能搜索 model 可选**：不配置 `baidu.model` 时走免费百度搜索（不产生 LLM 费用），配置模型名才启用 LLM 智能搜索生成；`enable_ai_search` 保持默认 `true`
- **升级 go-webfetch v0.2.0**：cleanfetch 引擎继承 TLS 指纹伪装（tls-client Chrome 131）+ 重试退避 + 系统代理支持，增强反爬能力

### 新增
- cleanfetch 新增 `use_system_proxy`（默认 false）和 `max_retries`（默认 3）配置项

## v2.12.0 — 2026-07-15

### 新增
- **`apipool` 搜索模式**：新增 API Key 池轮转模式，百度智能搜索 + Tavily + Exa 并发去重
  - 负载策略：先选供应商（round-robin），再轮转该供应商的 SK（KeyPool）
  - 百度网页搜索作为无 Key 兜底引擎自动参与
  - 配置项：`mode: apipool`
- **百度智能搜索端点**：新增 `chat/completions` 智能搜索 API（`baidu_ai.go`），返回 LLM 生成的回答 + 参考来源
  - `baidu.enable_ai_search` 控制端点选择（默认 `true` = 智能搜索，`false` = 旧版网页搜索）
  - 支持 `model`（默认 `ernie-4.5-turbo-32k`）、`search_source`（默认 `baidu_search_v2`）、`enable_reasoning`、`enable_deep_search`、`search_mode` 等配置
- **API Key 多 Key 轮询（KeyPool）**：`baidu`/`tavily`/`exa` 三个供应商均支持 `sk_list` 多 Key 轮询
  - `sk_list` 非空时优先使用，为空时自动用 `api_key` 构造单元素列表
  - KeyPool 线程安全，round-robin 轮转
  - 支持 Key 失效标记（`MarkInvalid`），失效 30 分钟后自动恢复
  - 所有 Key 均失效时自动降级为最早恢复的 Key

### 变更
- `baidu` 模式默认使用智能搜索端点（`enable_ai_search: true`），可通过配置切回旧版 `web_search`
- `hybrid` 模式百度引擎也使用 `enable_ai_search` 配置控制端点选择
- `NewBaiduSeach`、`NewTavilySearch`、`NewExaSearch`/`NewExaSearchWithResults` 构造函数改为接收 `*KeyPool` 参数
- `lookbackDaysToRecency(0)` 修复为返回 `semiyear`（默认值）而非 `day`

## v2.11.0 — 2026-07-13

### 新增
- **Exa Web Search API 引擎**：新增 Exa 通用搜索，支持 `type: auto` 自动选择搜索类型、`highlights` 高亮摘要、`excludeDomains` 排除域名
  - `mode=exa` 独立使用 Exa；`mode=hybrid` 时自动参与混合搜索
  - 配置项：`exa.api_key`（环境变量 `EXA_API_KEY`）、`exa.num_results`（默认 5）、`exa.lookback_days`（默认 90）
- **API 引擎时间范围配置**：所有 API 引擎（Tavily、百度千帆、Exa）统一支持 `SearchTimeRanger` 接口，可按时间范围过滤搜索结果
  - Tavily：映射为 `time_range` 参数（day/week/month/year）
  - 百度千帆：映射为 `search_recency_filter` 参数（day/week/month/semiyear/year）
  - Exa：映射为 `startPublishedDate` / `endPublishedDate` 日期范围
- **smartsearch 工具 time_range 参数**：新增 `time_range` 参数（月为单位），支持动态控制搜索时间范围，默认 3 个月
  - 示例：`time_range=1` 搜索近 1 个月，`time_range=6` 搜索近半年，`time_range=12` 搜索近一年
- 新增 `SearchTimeRanger` 可选接口，支持时间范围的引擎自动实现
- 新增 `HybridSearchImpl.SearchRawWithTimeRange`，将时间范围透传给支持的子引擎
- 新增 Exa、Tavily、百度千帆集成测试（API Key 存储在 `config.test.yaml`，已 gitignore）

### 变更
- `config.example.yaml` 搜索模式注释新增 `exa` 模式说明
- 百度千帆 `search_recency_filter` 从硬编码 `semiyear` 改为可配置，默认值不变

## v2.10.0 — 2026-07-12

### 新增
- **DuckDuckGo 搜索引擎**：新增 DuckDuckGo 通用搜索（需代理），`html.duckduckgo.com/html/` POST + goquery 解析，自动参与 engine/hybrid 模式
- **HEAD 预检防大文件**：cleanfetch 抓取前先发 HEAD 请求检查 Content-Length，超过阈值（默认 10MB，`max_fetch_size_mb` 可配）直接拒绝
- **DNS rebinding 防护**：cleanfetch 抓取前 DNS 解析目标域名，检查所有 IP 是否为内网/私有地址，与 go-webfetch 的 BlockPrivateIP 形成双重防护
- **Jina Reader DNS 防护增强**：`isPrivateHost()` 增加 DNS 解析检查，修复纯字符串匹配的绕过风险

### 变更
- **Google 引擎默认禁用**：Google 搜索被反爬机制拦截（TLS 指纹+JS Challenge），默认 `google.enabled: false`，需显式启用
- `CleanFetchConfig` 新增 `MaxFetchSizeMB` 字段（默认 10）
- `Config` 新增 `DuckDuckGoConfig` 和 `GoogleConfig` 结构体

### 修复
- **Google 反爬检测增强**：`detectSorry()` 新增 JS Challenge 页面识别（`/httpservice/retry/enablejs`、`SG_SS`）
- **Google 解析防御**：`parseResults()` 增加 `div#rso`/`div#search` 容器预检，非搜索结果页面直接返回空
- **Google 解析策略增强**：新增 SearXNG 风格的 `a[data-ved]:not([class])` 选择器作为主解析路径，`div.g` 作为回退

## v2.9.0 — 2026-07-12

### 新增
- **MinerU AI 增强 PDF 解析**：`pdf_parser` 工具集成 [MinerU](https://mineru.net) 文档解析平台，支持表格/公式/多栏/图片智能识别
  - **双 API 模式自动切换**：
    - 有 Token → 精准解析 API（`/api/v4`），支持远程 URL 输入，≤200MB/200页，ZIP 输出含 Markdown + JSON
    - 无 Token → Agent 轻量 API（`/api/v1/agent`），支持本地文件签名上传，≤10MB/20页，Markdown 输出
  - **本地文件优先 MinerU**：Agent API 自动签名上传本地 PDF，失败时静默回退到本地解析（go-webfetch）
  - **远程 URL 优先 MinerU**：精准 API 解析远程 URL，失败时回退到本地 webfetch
  - **友好错误处理**：API 错误码统一翻译为中文提示，原始错误仅记录日志，不暴露给客户端
  - 配置项：`mineru_token`（环境变量 `MINERU_TOKEN`）、`mineru_model`（pipeline/vlm）、`mineru_ocr`、`mineru_formula`、`mineru_table`、`mineru_lang`
- 新增 `pkg/mineru/` 包：MinerU API 客户端实现 + 单元测试 + 集成测试

### 变更
- `PDFParserConfig` 扩展 MinerU 相关字段，新增 `MinerUEnabled()`、`GetMinerUModel()` 等辅助方法
- `webfetch.Fetcher` 初始化时自动检测 MinerU 配置并创建客户端
- `pdf_parser` MCP 工具描述动态显示 MinerU 增强状态

## v2.8.0 — 2026-07-11

### 修复
- **cleanfetch 系统代理自动检测**：修复 cleanfetch 无法访问境外站点的问题，支持自动检测系统代理配置

## v2.7.0 — 2026-06-25

### 新增
- **缓存显式开关**：`CacheConfig` 新增 `cache.enabled` 字段（`*bool`），支持显式启用/禁用缓存
  - 不设置时按 `storage_path` 是否非空判断（向后兼容旧行为）
  - 显式 `false` 强制禁用缓存（忽略 `storage_path`）
  - 显式 `true` 强制启用缓存
- **英文版文档**：新增 `README.EN.md`、`docs/config.en.md`、`docs/api.en.md`、`CHANGELOG.en.md`，中英文文档顶部互加语言切换链接
- 新增 `CacheEnabled()` 单元测试（6 个用例覆盖所有分支）

## v2.6.0 — 2026-06-24

### 新增
- **SmartSearch Score 过滤**：per-engine `min_score` / `max_size`，全局 `max_size`，`show_meta` 控制来源和分数展示
  - 引擎回传 score 时按 `min_score` 过滤，保留 `max_size` 条
  - 引擎不回传 score 时忽略 `min_score`，取 `min(max_size, ⌈global_max_size / 引擎数⌉)` 截断
  - 全局 `max_size`：有 score 时按 score 排序截断，无 score 时按引擎轮询均匀分配
  - `show_meta` 控制输出中是否显示引擎来源和相关性分数（默认 true）
- **API 引擎命名区分**：Tavily → `tavily_api`、百度千帆 → `baidu_api`；百度网页搜索保持 `baidu`
- 引擎 `Name()` 方法统一纳入 `SearchInf` 接口

## v2.5.0 — 2026-06-03

### 新增
- **系统代理自动检测**：默认读取 Windows 注册表（`ProxyEnable` / `ProxyServer`）和环境变量（`HTTP_PROXY` / `HTTPS_PROXY`），Clash、V2RayN 等代理软件开启系统代理后自动生效，无需手动配置 `proxy.enabled` 和重启服务
  - `pkg/proxy/sysproxy.go` — 跨平台系统代理检测（Windows 注册表 + WinHTTP + 环境变量）
  - `pkg/proxy/detector.go` — 后台轮询检测器，30s 周期检测代理变更并通知回调
  - `pkg/proxy/proxy.go` — `DynamicProxyTransport` 请求级动态代理解析，每次请求实时获取代理端点
- **Jina Reader 不再依赖 proxy.enabled**：配置 `jina.api_key` 即可启用，代理由系统自动检测
- **全局限流重试**：所有 HTTP 客户端自动处理 429 限流，读取 `Retry-After` 头等待后重试一次（最大等待 5s，超限直接返回 429）
- **arXiv 引擎限流**：内置 1 req/s 限流器（arXiv 官方建议），超限时等待而非触发 429

### 变更
- **引擎初始化逻辑重构**：Google、Semantic Scholar、Google Scholar 始终初始化（除非各自 `disable_*` 显式禁用），代理在请求级别动态解析，开关代理无需重启
- **Jina Reader 去除 proxy.enabled 门控**：`NewFromConfig` 仅检查 `jina.api_key`，代理由 `ProxyResolver` 动态解析
- **Google 引擎 HTTP 客户端**：从静态 `http.ProxyURL` 改为 `DynamicProxyTransport`，请求时实时解析代理
- **学术引擎 HTTP 客户端**：从 `proxy.NewHTTPClient(endpoint)` 改为 `proxy.NewDynamicHTTPClient(resolver)`，支持运行时代理切换；国内引擎也使用带 retry 的客户端
- **测试 URL 更新**：`TestFetchWebPage` / `TestCleanFetch_WebFetchSuccess` 测试 URL 从 `arthurchiao.art`（已不可达）更换为 `wmyskxz.cn`

### 向后兼容
- `proxy.enabled: true` + `proxy.endpoint` 仍正常工作（显式代理模式）
- `proxy.enabled: false` 仍正常工作（显式禁用代理）
- 未设置 `proxy.enabled` 时行为变更：从"不使用代理"变为"自动检测系统代理"

### 新增文件
- `pkg/proxy/sysproxy.go` — 系统代理检测核心逻辑
- `pkg/proxy/sysproxy_windows.go` — Windows 注册表 + WinHTTP 检测实现
- `pkg/proxy/sysproxy_other.go` — 非 Windows 平台占位（仅环境变量）
- `pkg/proxy/detector.go` — 后台代理变更检测器

## v2.4.0 — 2026-06-02

### 新增
- **百度网页搜索引擎**：参考 SearXNG baidu.py 实现，使用 `tn=json` JSON API 直接抓取百度搜索结果，无需 API Key
  - `mode=baidu` 无 SK 时作为主引擎；有 SK 时 SK 失败自动回退网页搜索
  - `mode=engine` 默认与 Bing 并发搜索
  - `mode=hybrid` 自动参与混合搜索
- **Google 搜索引擎**：参考 SearXNG google.py 实现，HTML 解析 Google 搜索结果，需代理访问
  - `proxy.enabled=true` 时自动启用，支持 CAPTCHA 检测、CONSENT Cookie 绕过
  - `mode=engine` / `mode=hybrid` 代理启用时自动加入并发搜索
- **全局限流配置**：新增 `rate_limit` 配置节（`per_sec` / `per_min`），对所有搜索引擎统一生效（默认 3/s, 60/min）

### 变更
- **`engine` 模式引擎组合**：从仅 Bing 改为百度网页搜索 + Bing 并发（代理启用时加入 Google）
- **`hybrid` 模式百度策略**：有 SK 时使用 `BaiduWithFallback(SK, 网页搜索)`，SK 失败自动回退；无 SK 时直接用网页搜索
- **`baidu` 模式增强**：无 SK 时不再回退到 Bing，而是使用百度网页搜索引擎
- **限流默认值提升**：全引擎默认 3/s, 60/min（原 Bing 1/s, 20/min）
- Bing 限流配置从 `bing.per_sec` / `bing.per_min` 迁移到全局 `rate_limit` 配置

### 新增文件
- `pkg/baidu/` — 百度网页搜索引擎（engine + opts + 15 个单元测试）
- `pkg/google/` — Google 搜索引擎（engine + opts + 17 个单元测试）
- `pkg/search/engine_adapter.go` — 通用引擎适配器（antirobot.Engine → SearchInf）
- `pkg/search/baidu_fallback.go` — 百度 SK 回退包装器

## 2026-05-28

### 新增
- **cleanfetch 增强型网页抓取**：集成 `go-webfetch` 库，无需代理即可抓取网页内容，失败时自动回退到 Jina Reader（需代理）
  - `cleanfetch.enabled` 控制开关（默认 false，旧配置不启用）
  - 大内容自动存储到临时文件，支持配置输出目录、TTL、内联阈值
- **pdf_parser PDF 解析工具**：将本地 PDF 文件转换为 Markdown（`pdf_parser.enabled` 控制，默认 false）
- **hybrid 模式 Bing 混合搜索**：hybrid 模式下 Bing 作为原生引擎与 API 引擎（Baidu/Tavily）并发搜索

### 变更
- cleanfetch 工具现在只需配置 `cleanfetch.enabled: true` 即可使用，不再强制要求代理和 Jina API Key
- Go 版本升级至 1.26（go-webfetch 依赖要求）

## 2026-05-26

### 新增
- **Windows 开机自启动**：`install` / `uninstall` 命令，使用 COM API (ole32.dll) 创建快捷方式，无需依赖 PowerShell
- **PubMed 学术引擎**：生物医学文献权威数据库，国内直连
- **Google Scholar 学术引擎**：全学科学术搜索，需代理
- **MCP 工具拆分**：`smartsearch`（通用搜索）+ `academicsearch`（学术搜索）独立工具，`academicsearch` 支持 `engines` / `time_range` / `page` 参数
- **学术搜索并行化**：多引擎并发请求，结果按 URL 去重 + 分组归一化排序
- **BingFallback 配置**：`academic.bing_fallback` 控制学术搜索时是否用 Bing 兜底
- **proxy 配置**：仅海外学术引擎（Semantic Scholar、Google Scholar）走代理
- **CI 自动发版**：GitHub Actions workflow，tag 推送后自动构建 linux/windows 二进制并发布 Release，附带 SHA256 校验

### 重构
- **提取 server 包**：`RunServer`、admin handlers、引用计数逻辑从 `cmd/main.go` 提取到可导出的 `server` 包，支持作为 Go 模块嵌入
- **学术引擎独立模块**：新增 `pkg/academic`（6 个引擎独立实现）和 `pkg/antirobot`（共享引擎框架：Engine 接口、Searcher 编排器、限流器）
- **Bing 包精简**：`pkg/bing` 仅保留 Bing 通用搜索引擎 + 反爬逻辑

### 文档
- 新增 [docs/api.md](docs/api.md)：Go Module API 和 HTTP API 完整文档
- 新增 [docs/configuration.md](docs/configuration.md)：配置参考、默认值速查、环境变量覆盖
- README 全面重写：精简结构化，补充特性亮点、运维参考、排障指南

## 2026-05-23

### 新增
- **cleanfetch 网页抓取工具**：通过 Jina Reader API 获取指定 URL 的干净网页内容，降低反爬拦截风险
  - 仅在配置 `jina.api_key` 后注册，不影响现有功能
  - 对常见 HTTP 错误（403/404/429 等）返回简明中文提示
  - 新增 SSRF 防护：URL 协议校验、内网地址黑名单
  - 新增客户端超时（30s）防止 goroutine 泄漏

### 优化
- **学术搜索结果增强**：保留论文元数据（作者、DOI、类型），格式化时自动区分论文和网页结果
- **缓存系统改进**：
  - 支持 `academic` 参数区分，防止学术/非学术缓存混用
  - 数据库自动迁移，兼容旧版缓存
  - 查询优化：两步查询充分利用索引
- **站点屏蔽统一**：`black_list_host` 和 `bing.blocked` 自动合并，SearXNG 后端同步生效
- **字符串拼接优化**：`MergeContent` 改用 `strings.Builder`，复杂度从 O(n²) 降为 O(n)
- **排序优化**：`HybridSearchImpl` 冒泡排序改为 `sort.Slice`

### 修复
- 学术搜索失败时不再静默回退到通用搜索，返回明确错误信息
- Tavily 搜索正确使用 `exclude_domains` 过滤站点
- `describeHTTPError` 使用 `fmt.Sprintf` 替代不必要的 `fmt.Errorf`

---

## 2026-05-20

### 新增
- LLM 摘要未启用时，`smartsearch` 工具自动移除 `intent` 参数，节省客户端上下文 token
- MCP 服务增加 30s 心跳 + 5 分钟空闲 session 自动清理
- HTTP Server 增加超时配置（ReadHeader 10s / Read 60s / Idle 120s）
- 异步摘要 goroutine 增加 panic recover

### 修复
- Dockerfile 启动参数缺失导致容器立即退出

---

## 2026-05-15

### 新增
- `engine` 搜索模式：无需 API Key，使用 Bing 通用搜索 + 学术搜索引擎
- 学术搜索引擎集成：arXiv、Crossref、OpenAlex、Semantic Scholar
- MCP 工具新增 `academic` 参数
- `black_list_host` 屏蔽站点配置（对 Bing 和 Tavily 生效）

### 优化
- LLM 摘要提示词：主动过滤低质量内容、合并重复结果、保留关键原文并标注引用

---

## 2026-05-01

### 新增
- Tavily 搜索 API 支持
- LLM 摘要支持（建议使用快速模型）
- SQLite 缓存管理

---

## 2026-04-15

### 初始版本
- 百度千帆 AI Search API 支持
- 基础 MCP 服务框架
