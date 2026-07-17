# 配置参考

[English](config.en.md) | [中文](config.md)

## 配置文件路径

优先级（从高到低）：
1. 环境变量 `WEBSEARCH_CONFIG`
2. CLI 参数 `-c / --config`
3. 当前目录 `config.yaml`

> 通过 `-c` 指定后，PID 文件和日志文件自动写到配置文件所在目录。

## 完整配置

```yaml
port: 8338                  # MCP HTTP 端口
log_level: info             # debug / info / warn / error
mode: engine                # baidu / apipool / tavily / exa / hybrid / engine
network: china              # china（跳过海外引擎） / international

# 屏蔽站点（对所有搜索引擎生效）
black_list_host:
  - "csdn.net"
  - "baidu.com"

# 百度千帆（mode=baidu/apipool/hybrid 时需要）
baidu:
  api_key: ""               # 环境变量: BAIDU_SK（sk_list 为空时自动作为单元素列表）
  sk_list: []               # 多 Key 轮询列表（优先级高于 api_key）
  enable_ai_search: true    # true=智能搜索 chat/completions（默认），false=网页搜索 web_search
  model: ""                 # 智能搜索模型名，不传=免费搜索（不产生 LLM 费用），传入=LLM 智能搜索
  search_source: "baidu_search_v2" # 搜索引擎版本
  enable_reasoning: false   # 深度思考
  enable_deep_search: false # 深搜索
  search_mode: "auto"       # auto / required / disabled

# Tavily（mode=tavily/apipool/hybrid 时需要）
tavily:
  api_key: ""               # 环境变量: TAVILY_SK（sk_list 为空时自动作为单元素列表）
  sk_list: []               # 多 Key 轮询列表（优先级高于 api_key）

# Exa（mode=exa/apipool/hybrid 时需要）
exa:
  api_key: ""               # 环境变量: EXA_API_KEY（sk_list 为空时自动作为单元素列表）
  sk_list: []               # 多 Key 轮询列表（优先级高于 api_key）
  num_results: 5            # 单次搜索结果数量（默认 5）
  lookback_days: 90         # 搜索时间范围（天），默认 90

# Bing 引擎（兜底 + engine 模式主力，无需 Key）
bing:
  enabled: true             # 总开关
  blocked: []               # Bing 专用屏蔽（与 black_list_host 合并）
  per_sec: 1                # 每秒限流
  per_min: 20               # 每分钟限流

# 学术引擎（无需 Key）
academic:
  enabled: true             # 总开关，开启后注册 academicsearch 工具
  bing_fallback: true       # 学术搜索用 Bing 兜底
  disable_arxiv: false
  disable_crossref: false
  disable_openalex: false
  disable_pubmed: false
  disable_semantic_scholar: true    # 默认禁用（开启后自动通过代理访问）
  disable_google_scholar: true      # 默认禁用（开启后自动通过代理访问）

# 代理（默认自动检测系统代理，无需手动配置）
proxy:
  enabled: false          # 留空→自动检测；true→使用 endpoint；false→禁用
  endpoint: "http://127.0.0.1:7897"  # 仅 enabled: true 时生效

# LLM 摘要（可选）
llm:
  base_url: "https://api.openai.com/v1"   # 环境变量: LLM_BASE_URL
  api_key: ""                               # 环境变量: LLM_API_KEY
  model_id: "gpt-4o-mini"

# 缓存
cache:
  # enabled: true            # 不设置时按 storage_path 判断；显式 false 强制禁用
  storage_path: "./data/search_cache.db"
  cleanup_interval: 30      # 清理间隔（分钟），最大 360

# Jina Reader（可选，cleanfetch 失败时回退）
jina:
  api_key: ""               # 留空则不启用 Jina 回退
  base_url: ""              # 默认 https://r.jina.ai

# 增强型网页抓取（默认关闭）
cleanfetch:
  enabled: false            # 显式 true 才启用
  file_output_dir: ""       # 默认 系统临时目录/webfetch/
  file_ttl_hours: 24        # 临时文件保留时长（小时）
  max_inline_lines: 100     # 超过此行数存文件
  max_inline_chars: 0       # 超过此字符数存文件，0=不限
  use_system_proxy: false   # 自动使用系统代理（环境变量+注册表），默认 false
  max_retries: 3            # 最大重试次数（仅 429/502/503），默认 3

# PDF 解析工具（默认关闭，独立于 cleanfetch）
# MinerU AI 增强（可选）：有 Token 用精准 API（远程 URL，≤200MB），无 Token 用 Agent 轻量 API（本地文件，≤10MB）
# 获取 Token: https://mineru.net/apiManage | 环境变量: MINERU_TOKEN
pdf_parser:
  enabled: false            # 显式 true 才启用
  # mineru_token: ""        # JWT Token，有则启用精准 API
  # mineru_model: "pipeline" # pipeline(默认) / vlm(推荐)
  # mineru_ocr: false        # OCR 识别
  # mineru_formula: true     # 公式识别（默认 true）
  # mineru_table: true       # 表格识别（默认 true）
  # mineru_lang: "ch"        # 文档语言（默认 ch）

# 搜索结果过滤与输出格式（可选）
# smartsearch:
#   max_size: 10          # 全局最大结果数（按 score 排序后截断），0 = 不限
#   show_meta: true       # 输出中显示引擎来源和相关性分数（默认 true）
#   engines:              # 按引擎名配置（引擎名: tavily_api, exa, baidu_api, baidu, bing, google, duckduckgo）
#     tavily_api:
#       min_score: 0.5    # Tavily API 最低相关性分数阈值（0 = 不过滤）
#       max_size: 6       # Tavily API 单引擎最大结果数（默认 4）
#     exa:
#       min_score: 0      # Exa 不回传 score，此字段无效
#       max_size: 4       # Exa 单引擎最大结果数
#     baidu_api:
#       min_score: 0      # 百度千帆搜索不回传 score（enable_ai_search 控制端点）
#       max_size: 5       # 百度千帆搜索单引擎最大结果数
#     baidu:
#       min_score: 0      # 百度网页搜索不回传 score
#       max_size: 5       # 百度网页搜索单引擎最大结果数
#     bing:
#       min_score: 0      # Bing 不回传 score，此字段无效
#       max_size: 4       # Bing 单引擎最大结果数
#     google:
#       min_score: 0      # Google 不回传 score，此字段无效
#       max_size: 4       # Google 单引擎最大结果数
#     duckduckgo:
#       min_score: 0      # DuckDuckGo 不回传 score，此字段无效
#       max_size: 4       # DuckDuckGo 单引擎最大结果数

# Apipool 模式配置（可选，mode=apipool 时生效）
# apipool:
#   strategy: round-robin  # round-robin（默认）: 跨请求轮转起始供应商
#                          # priority: 始终从第一个供应商开始
#   engines:               # 供应商优先级顺序（默认 [baidu, tavily, exa]，百度网页搜索兜底始终在末尾）
#     - baidu
#     - tavily
#     - exa

# 日志滚动
log:
  max_size: 1               # 单文件最大 MB
  max_age: 1                # 保留天数
```

## 环境变量覆盖

| 环境变量 | 覆盖字段 | 说明 |
|----------|---------|------|
| `WEBSEARCH_CONFIG` | 配置文件路径 | 最高优先级 |
| `BAIDU_SK` | `baidu.api_key` | |
| `TAVILY_SK` | `tavily.api_key` | |
| `EXA_API_KEY` | `exa.api_key` | Exa Web Search API Key |
| `LLM_BASE_URL` | `llm.base_url` | |
| `LLM_API_KEY` | `llm.api_key` | |
| `MINERU_TOKEN` | `pdf_parser.mineru_token` | MinerU 精准解析 API Token |

> Viper 的 `AutomaticEnv()` 还支持 `APP_` 前缀覆盖任意配置项。

## 默认值速查

| 字段 | 默认值 | 说明 |
|------|--------|------|
| `port` | 8338 | stop/kill/status 无配置时也用此端口 |
| `mode` | baidu | 无 Key 时自动回退 engine；`apipool` 为 API Key 池轮转模式，支持 round-robin / priority 策略 |
| `apipool.strategy` | round-robin | `round-robin` 跨请求轮转供应商 / `priority` 固定优先级顺序 |
| `apipool.engines` | [baidu, tavily, exa] | 供应商优先级顺序，百度网页搜索兜底始终在末尾 |
| `baidu.enable_ai_search` | true | true=智能搜索 chat/completions，false=网页搜索 web_search；不传 model 不产生 LLM 费用 |
| `network` | china | |
| `bing.enabled` | true | |
| `bing.per_sec` | 1 | |
| `bing.per_min` | 20 | |
| `academic.enabled` | true | |
| `academic.bing_fallback` | true | |
| `proxy.enabled` | false | 未设置时自动检测系统代理；显式 false 禁用 |
| `proxy.endpoint` | `http://127.0.0.1:7897` | 仅 `enabled: true` 时生效 |
| `cleanfetch.enabled` | false | 旧配置不启用，需显式开启 |
| `cleanfetch.file_ttl_hours` | 24 | |
| `cleanfetch.max_inline_lines` | 100 | |
| `cleanfetch.use_system_proxy` | false | 自动使用系统代理（环境变量+注册表） |
| `cleanfetch.max_retries` | 3 | 仅对 429/502/503 重试 |
| `pdf_parser.enabled` | false | 独立于 cleanfetch |
| `pdf_parser.mineru_model` | pipeline | pipeline / vlm |
| `pdf_parser.mineru_formula` | true | 公式识别 |
| `pdf_parser.mineru_table` | true | 表格识别 |
| `pdf_parser.mineru_lang` | ch | 文档语言 |
| `cache.enabled` | nil | 不设置时按 storage_path 判断；显式 false 强制禁用；显式 true 强制启用 |
| `cache.cleanup_interval` | 30 (min) | 最大 360 |
| 缓存过期 | 6 小时 | 基于最近命中时间，硬编码不可配置 |
| `log.max_size` | 1 (MB) | |
| `log.max_age` | 1 (day) | |

## 最小配置

```yaml
port: 8338
mode: engine
```

零 API Key 即可运行，使用 Bing + 学术搜索引擎。
