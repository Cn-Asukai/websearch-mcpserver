# Configuration Reference

[English](config.en.md) | [中文](config.md)

## Config File Path

Priority (high to low):
1. Environment variable `WEBSEARCH_CONFIG`
2. CLI flag `-c / --config`
3. Current directory `config.yaml`

> When specified via `-c`, PID file and log file are automatically written to the config file's directory.

## Full Configuration

```yaml
port: 8338                  # MCP HTTP port
log_level: info             # debug / info / warn / error
mode: engine                # baidu / apipool / tavily / exa / hybrid / engine
network: china              # china (skip overseas engines) / international

# Blocked sites (applies to all search engines)
black_list_host:
  - "csdn.net"
  - "baidu.com"

# Baidu Qianfan (required for mode=baidu/apipool/hybrid)
baidu:
  api_key: ""               # Env: BAIDU_SK (falls back to single-element sk_list when empty)
  sk_list: []               # Multi-key rotation list (priority over api_key)
  enable_ai_search: true    # true=AI search chat/completions (default), false=web search web_search
  model: ""                 # AI search model, empty=free Baidu search (no LLM cost), set name=LLM intelligent search
  search_source: "baidu_search_v2" # Search engine version
  enable_reasoning: false   # Deep reasoning
  enable_deep_search: false # Deep search
  search_mode: "auto"       # auto / required / disabled

# Tavily (required for mode=tavily/apipool/hybrid)
tavily:
  api_key: ""               # Env: TAVILY_SK (falls back to single-element sk_list when empty)
  sk_list: []               # Multi-key rotation list (priority over api_key)

# Exa (required for mode=exa/apipool/hybrid)
exa:
  api_key: ""               # Env: EXA_API_KEY (falls back to single-element sk_list when empty)
  sk_list: []               # Multi-key rotation list (priority over api_key)
  num_results: 5            # Results per search (default 5)
  lookback_days: 90         # Search time range (days), default 90

# Bing engine (fallback + engine mode primary, no key needed)
bing:
  enabled: true             # Master switch
  blocked: []               # Bing-specific blocked domains (merged with black_list_host)
  per_sec: 1                # Rate limit per second
  per_min: 20               # Rate limit per minute

# Academic engines (no key needed)
academic:
  enabled: true             # Master switch, registers academicsearch tool
  bing_fallback: true       # Use Bing as fallback for academic search
  disable_arxiv: false
  disable_crossref: false
  disable_openalex: false
  disable_pubmed: false
  disable_semantic_scholar: true    # Disabled by default (auto-proxied when enabled)
  disable_google_scholar: true      # Disabled by default (auto-proxied when enabled)

# Proxy (auto-detects system proxy by default, no manual config needed)
proxy:
  enabled: false          # Empty → auto-detect; true → use endpoint; false → disable
  endpoint: "http://127.0.0.1:7897"  # Only effective when enabled: true

# LLM summary (optional)
llm:
  base_url: "https://api.openai.com/v1"   # Env: LLM_BASE_URL
  api_key: ""                               # Env: LLM_API_KEY
  model_id: "gpt-4o-mini"

# Cache
cache:
  # enabled: true            # Not set → judge by storage_path; explicit false → force disable
  storage_path: "./data/search_cache.db"
  cleanup_interval: 30      # Cleanup interval (minutes), max 360

# Jina Reader (optional, fallback for cleanfetch)
jina:
  api_key: ""               # Empty → Jina fallback disabled
  base_url: ""              # Default https://r.jina.ai

# Enhanced web fetch (disabled by default)
cleanfetch:
  enabled: false            # Must be explicitly true to enable
  file_output_dir: ""       # Default: system temp dir /webfetch/
  file_ttl_hours: 24        # Temp file retention (hours)
  max_inline_lines: 100     # Lines above this threshold stored to file
  max_inline_chars: 0       # Chars above this threshold stored to file, 0=unlimited
  use_system_proxy: false   # Auto-use system proxy (env vars + Windows registry), default false
  max_retries: 3            # Max retries (429/502/503 only), default 3

# PDF parser (disabled by default, independent of cleanfetch)
# MinerU AI enhancement (optional): with Token uses Standard API (remote URL, ≤200MB), without Token uses Agent API (local file, ≤10MB)
# Get Token: https://mineru.net/apiManage | Env: MINERU_TOKEN
pdf_parser:
  enabled: false            # Must be explicitly true to enable
  # mineru_token: ""        # JWT Token; enables Standard API when set
  # mineru_model: "pipeline" # pipeline (default) / vlm (recommended)
  # mineru_ocr: false        # OCR recognition
  # mineru_formula: true     # Formula recognition (default true)
  # mineru_table: true       # Table recognition (default true)
  # mineru_lang: "ch"        # Document language (default ch)

# Search result filtering and output format (optional)
# smartsearch:
#   max_size: 10          # Global max results (truncated by score), 0 = unlimited
#   show_meta: true       # Show engine source and relevance score in output (default true)
#   engines:              # Per-engine config (names: tavily_api, exa, baidu_api, baidu, bing, google, duckduckgo)
#     tavily_api:
#       min_score: 0.5    # Tavily API minimum relevance score threshold (0 = no filter)
#       max_size: 6       # Tavily API per-engine max results (default 4)
#     exa:
#       min_score: 0      # Exa doesn't return score, this field is ignored
#       max_size: 4       # Exa per-engine max results
#     baidu_api:
#       min_score: 0      # Baidu Qianfan doesn't return score (enable_ai_search controls endpoint)
#       max_size: 5       # Baidu Qianfan per-engine max results
#     baidu:
#       min_score: 0      # Baidu web search doesn't return score
#       max_size: 5       # Baidu web search per-engine max results
#     bing:
#       min_score: 0      # Bing doesn't return score, this field is ignored
#       max_size: 4       # Bing per-engine max results
#     google:
#       min_score: 0      # Google doesn't return score, this field is ignored
#       max_size: 4       # Google per-engine max results
#     duckduckgo:
#       min_score: 0      # DuckDuckGo doesn't return score, this field is ignored
#       max_size: 4       # DuckDuckGo per-engine max results

# Apipool mode config (optional, effective when mode=apipool)
# apipool:
#   strategy: round-robin  # round-robin (default): rotate starting provider across requests
#                          # priority: always start from first provider
#   engines:               # Provider priority order (default [baidu, tavily, exa], Baidu web search fallback always last)
#     - baidu
#     - tavily
#     - exa

# Log rotation
log:
  max_size: 1               # Max file size (MB)
  max_age: 1                # Retention (days)
```

## Environment Variable Overrides

| Env Var | Overrides | Notes |
|---------|-----------|-------|
| `WEBSEARCH_CONFIG` | Config file path | Highest priority |
| `BAIDU_SK` | `baidu.api_key` | |
| `TAVILY_SK` | `tavily.api_key` | |
| `EXA_API_KEY` | `exa.api_key` | Exa Web Search API Key |
| `LLM_BASE_URL` | `llm.base_url` | |
| `LLM_API_KEY` | `llm.api_key` | |
| `MINERU_TOKEN` | `pdf_parser.mineru_token` | MinerU Standard API Token |

> Viper's `AutomaticEnv()` also supports `APP_` prefix for overriding any config field.

## Default Values Quick Reference

| Field | Default | Notes |
|-------|---------|-------|
| `port` | 8338 | stop/kill/status also use this port when no config |
| `mode` | baidu | Auto-degrades to engine when no keys; `apipool` = API Key pool rotation, supports round-robin / priority strategies |
| `apipool.strategy` | round-robin | `round-robin` rotates provider across requests / `priority` fixed order |
| `apipool.engines` | [baidu, tavily, exa] | Provider priority order, Baidu web search fallback always last |
| `baidu.enable_ai_search` | true | true=AI search chat/completions, false=web search web_search; no LLM cost when model is empty |
| `network` | china | |
| `bing.enabled` | true | |
| `bing.per_sec` | 1 | |
| `bing.per_min` | 20 | |
| `academic.enabled` | true | |
| `academic.bing_fallback` | true | |
| `proxy.enabled` | false | Auto-detects system proxy when not set; explicit false disables |
| `proxy.endpoint` | `http://127.0.0.1:7897` | Only effective when `enabled: true` |
| `cleanfetch.enabled` | false | Old configs don't enable; must be explicit |
| `cleanfetch.file_ttl_hours` | 24 | |
| `cleanfetch.max_inline_lines` | 100 | |
| `cleanfetch.use_system_proxy` | false | Auto-use system proxy (env vars + Windows registry) |
| `cleanfetch.max_retries` | 3 | Only retries on 429/502/503 |
| `pdf_parser.enabled` | false | Independent of cleanfetch |
| `pdf_parser.mineru_model` | pipeline | pipeline / vlm |
| `pdf_parser.mineru_formula` | true | Formula recognition |
| `pdf_parser.mineru_table` | true | Table recognition |
| `pdf_parser.mineru_lang` | ch | Document language |
| `cache.enabled` | nil | Not set → judge by storage_path; explicit false → force disable; explicit true → force enable |
| `cache.cleanup_interval` | 30 (min) | Max 360 |
| Cache expiry | 6 hours | Based on last hit time, hardcoded |
| `log.max_size` | 1 (MB) | |
| `log.max_age` | 1 (day) | |

## Minimal Config

```yaml
port: 8338
mode: engine
```

Runs with zero API keys using Bing + academic search engines.
