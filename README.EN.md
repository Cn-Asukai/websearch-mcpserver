# websearch-mcpserver

> Lightweight Web Search MCP Server — runs with zero API keys

[English](README.EN.md) | [中文](README.MD)

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev/)
[![Release](https://img.shields.io/github/v/release/daidaiJ/websearch-mcpserver)](https://github.com/daidaiJ/websearch-mcpserver/releases)

---

## Introduction

An MCP search service built in Go with built-in Baidu web search, Bing, DuckDuckGo, and 6 academic search engines. Works with Claude Code, Qwen Code, Cursor, and other MCP clients, or embed as a Go module.

## Design Highlights

- **Zero-config startup** — `engine` mode requires no API keys; built-in Baidu web search + Bing dual-engine concurrent search, DuckDuckGo auto-joins when a proxy is available
- **API Key Pool** — `apipool` mode: one provider per request, auto-fallback on failure; supports `round-robin` (rotating) and `priority` (fixed order) strategies; each provider supports `sk_list` multi-key rotation with 30-min cooldown on failure; auto-retry next SK within the same provider
- **Baidu AI Search** — `baidu` mode defaults to Qianfan `chat/completions` endpoint; free Baidu search when `model` is empty (no LLM cost); LLM-powered search when a model name is set
- **System proxy auto-detection** — reads Windows registry / environment variables by default; Clash and similar proxy software enable DuckDuckGo, academic engines, Jina Reader automatically without manual configuration
- **Smart rate-limit retry** — all HTTP clients handle 429 responses automatically (reads `Retry-After` header); arXiv engine has a built-in 1 req/s limiter
- **Multi-engine parallel orchestration** — academic search fires requests to multiple engines concurrently with URL dedup + normalized grouping; hybrid mode mixes native engines
- **Intelligent relevance scoring** — RRF fusion ranking across engines, combined with lexical alignment, rare-term/phrase contiguity matching, domain-quality penalties (down-weighting brand/e-commerce/dictionary mismatches), and multi-engine consensus / authority-site / recency boosts; a global low-score threshold truly prunes low-quality context (Top-1 protected, at least 2 kept), fully heuristic with no AI model required; **MMR diversity re-ranking** (default λ=0.7) breaks up highly similar same-topic results (mirror/repost sites)
- **Academic search scoring** — six academic engines fused via RRF with citation count (log-compressed), high-impact journal/conference, PDF availability and recency signals; low-score papers auto-filtered (Top-1 + per-engine floor)
- **LLM summary streaming** — the summary stage pushes tokens in real time via MCP progress notifications; auto-cancels on client disconnect and falls back to non-streaming summary on failure
- **Smart fallback** — Baidu SK failure falls back to web search; primary engine failure falls back to Bing; LLM summary failure falls back to raw results; cleanfetch failure falls back to Jina Reader; cache errors are silently skipped
- **Enhanced web fetching** — based on go-webfetch, no proxy needed; TLS fingerprint spoofing (Chrome 131) + retry backoff + system proxy support; built-in SSRF protection, DNS rebinding detection, HEAD pre-check for large files and WAF detection; large content auto-stored to temp files
- **MinerU AI-enhanced PDF parsing** — optional MinerU integration for intelligent table/formula/multi-column/image recognition; with Token uses Standard API (≤200MB), without Token auto-degrades to Agent Lightweight API (≤10MB), silently falls back to local parsing on failure
- **Reference-counted process management** — multiple clients share one instance; auto-exits when count reaches zero
- **Sub-agent extension** — optional companion [web-researcher](https://github.com/daidaiJ/web-researcher) extension offloads web research to a fast model sub-agent, zero context bloat for the main model (see [Qwen Code Sub-Agent Extension](#qwen-code-sub-agent-extension-web-researcher))
- **Pure Go, no CGO** — SQLite via `modernc.org/sqlite`, single-binary deployment

## Feature Overview

| Category | Capabilities |
|----------|-------------|
| **General Search** | Baidu Qianfan, Baidu Web Search (built-in), Tavily, Exa, Bing (built-in), DuckDuckGo (auto-detects proxy), Google (disabled by default, needs explicit enable) |
| **Academic Search** | arXiv, Crossref, OpenAlex, PubMed (direct from China) + Semantic Scholar, Google Scholar (auto-detects proxy); RRF fusion + citation / journal / PDF / recency signal scoring with low-score filtering (per-engine floor) |
| **MCP Tools** | `smartsearch` web search · `academicsearch` paper search · `cleanfetch` web fetch · `pdf_parser` PDF parsing |
| **Caching** | SQLite auto-cache, 6h expiry, background cleanup |
| **LLM Summary** | Optional OpenAI-compatible API integration for structured summaries; token-by-token streaming via MCP progress notifications |
| **Site Blocking** | Global `black_list_host`, auto-filters low-quality sites |
| **Global Rate Limit** | `rate_limit` unified config for all search engines |
| **Relevance Scoring** | RRF fusion ranking + domain-quality / lexical-alignment / consensus / authority / recency boosts, global low-score threshold prunes low-quality context; MMR diversity re-ranking breaks up same-topic similar results; per-engine `min_score` / `max_size`, `show_meta` displays source and score |
| **Time Range** | `smartsearch` tool supports `time_range` parameter (in months); API engines unified time range filtering |
| **SearXNG Compatible** | `/searxng/search` endpoint, works with LiteLLM |

## Quick Start

```bash
# 1. Download: https://github.com/daidaiJ/websearch-mcpserver/releases
# 2. Start (zero config)
./websearch-mcpserver start
# 3. Add to your client (see "Installation > Register MCP Client" below)
```

## Installation

### Binary Download

| Platform | File |
|----------|------|
| Linux x86_64 | `websearch-mcpserver-linux-amd64` |
| Windows x86_64 | `websearch-mcpserver-windows-amd64.exe` |
| macOS Intel | `websearch-mcpserver-darwin-amd64` |
| macOS Apple Silicon | `websearch-mcpserver-darwin-arm64` |

SHA256 checksums: each release includes `SHA256SUMS.txt`.

Release page: https://github.com/daidaiJ/websearch-mcpserver/releases

### Docker

```bash
git clone --depth 1 https://github.com/daidaiJ/websearch-mcpserver.git
cd websearch-mcpserver && docker build -t websearch:v1 .
```

```yaml
# docker-compose.yml
services:
  websearch:
    image: websearch:v1
    restart: always
    volumes:
      - ./config.yaml:/app/config.yaml
    ports:
      - "8338:8338"
```

### Build from Source

```bash
go build -o websearch ./cmd/
# With version injection
go build -ldflags="-X main.version=v1.0.0" -o websearch ./cmd/
```

### Register MCP Client

After starting the service, register it with your client:

```bash
# Claude Code
claude mcp add --transport http websearch http://localhost:8338/mcp

# Qwen Code — add to settings.json mcpServers:
```

```json
{
  "mcpServers": {
    "websearch": {
      "type": "http",
      "url": "http://localhost:8338/mcp"
    }
  }
}
```

```bash
# Cursor / other MCP clients — type=http, url=http://localhost:8338/mcp
```

### Agent Quick Deploy

Agents can use these steps for automated installation:

```bash
# 1. Download latest binary
#    Linux:
curl -sL https://api.github.com/repos/daidaiJ/websearch-mcpserver/releases/latest \
  | grep "browser_download_url.*linux-amd64\"" \
  | cut -d '"' -f 4 \
  | xargs curl -sL -o /usr/local/bin/websearch-mcpserver
chmod +x /usr/local/bin/websearch-mcpserver

#    macOS (Intel):
# curl -sL https://api.github.com/repos/daidaiJ/websearch-mcpserver/releases/latest \
#   | grep "browser_download_url.*darwin-amd64\"" \
#   | cut -d '"' -f 4 \
#   | xargs curl -sL -o /usr/local/bin/websearch-mcpserver
# chmod +x /usr/local/bin/websearch-mcpserver

#    macOS (Apple Silicon):
# curl -sL https://api.github.com/repos/daidaiJ/websearch-mcpserver/releases/latest \
#   | grep "browser_download_url.*darwin-arm64\"" \
#   | cut -d '"' -f 4 \
#   | xargs curl -sL -o /usr/local/bin/websearch-mcpserver
# chmod +x /usr/local/bin/websearch-mcpserver

#    Windows (PowerShell):
# $release = Invoke-RestMethod https://api.github.com/repos/daidaiJ/websearch-mcpserver/releases/latest
# $asset = $release.assets | Where-Object { $_.name -match 'windows-amd64' }
# Invoke-WebRequest -Uri $asset.browser_download_url -OutFile "C:\tools\websearch-mcpserver.exe"

# 2. Write minimal config (zero keys needed)
mkdir -p ~/.config/websearch
cat > ~/.config/websearch/config.yaml << 'EOF'
port: 8338
mode: engine
EOF

# 3. Start and register (see "Register MCP Client" above)
./websearch-mcpserver start
```

Windows auto-start (optional): run `websearch-mcpserver.exe install` after download.

## Search Modes

| Mode | Description | Key Required |
|------|-------------|--------------|
| `baidu` | Baidu Qianfan search (`enable_ai_search` controls endpoint), falls back to Baidu web search on failure; uses Baidu web search directly when no SK | `BAIDU_SK` (optional) |
| **`apipool`** | **API Key pool rotation**: one provider per request, auto-fallback; supports `round-robin` / `priority` strategies; auto-retry next SK within provider; Baidu web search as final fallback | All optional |
| `tavily` | Tavily Search API | `TAVILY_SK` |
| `exa` | Exa Web Search API | `EXA_API_KEY` |
| `hybrid` | Full mixed (Baidu AI + Baidu web + Tavily + Exa + Bing + DuckDuckGo) | All optional |
| **`engine`** | **Baidu web search + Bing** (DuckDuckGo auto-joins when proxy available) | **None** |

> All modes auto-fallback on primary engine failure. Auto-degrades to `engine` mode when keys are missing. `baidu`/`tavily`/`exa` all support `sk_list` multi-key rotation; `sk_list` falls back to `api_key` as single-element list when empty. `apipool` provider order and strategy are configurable via the `apipool` section, see [Apipool Config](#apipool-config).

### SmartSearch Advanced Config

The `smartsearch` section controls result filtering, truncation, and output format:

```yaml
smartsearch:
  max_size: 10           # Global max results (truncated by score), 0 = unlimited
  show_meta: true        # Show engine source and relevance score in output (default true)
  enhance: true          # Local scoring enhancement (RRF fusion + lexical alignment + domain quality + boosts + threshold), default true
  relevance_threshold: 0.05  # Relevance threshold after enhancement; below this is filtered (Top-1 protected), default 0.05
  mmr:                       # MMR diversity re-ranking (breaks up same-topic similar results)
    enabled: true            # Master switch (default true)
    lambda: 0.7              # Relevance weight [0,1]; higher = more relevance, lower = more diversity (default 0.7)
    target_count: 0          # Target count after MMR; 0 = no extra truncation (max_size applies)
  engines:
    tavily_api:        # Tavily API (returns score, supports min_score)
      min_score: 0.5   # Minimum relevance score threshold, 0 = no filter
      max_size: 6      # Per-engine max results (default 4)
    bing:              # Bing (no score, min_score ignored)
      max_size: 4
    baidu_api:         # Baidu Qianfan Search (no score, enable_ai_search controls endpoint)
      max_size: 5
    baidu:             # Baidu web search (no score)
      max_size: 5
    google:            # Google (disabled by default, anti-bot blocked)
      max_size: 4
    duckduckgo:        # DuckDuckGo (no score, needs proxy)
      max_size: 4
```

**Score filtering logic**:
- Engine returns score: filter by `min_score`, keep `max_size` results
- Engine returns no score: ignore `min_score`, take `min(max_size, ⌈global_max_size / engine_count⌉)`
- Global `max_size`: with scores → sort by score and truncate; without scores → round-robin distribution across engines

### Apipool Config

The `apipool` section controls provider selection strategy and priority order for `mode: apipool`:

```yaml
apipool:
  strategy: round-robin   # round-robin (default) / priority
  engines:                # Provider priority order (default [baidu, tavily, exa])
    - baidu
    - tavily
    - exa
```

**Strategy details**:
- **`round-robin`** (default): rotates the starting provider across requests; within a single request, exhausts all available SKs in the current provider before falling back to the next
- **`priority`**: always starts from the first provider in the list; exhausts all SKs → switches to next provider → Baidu web search as final fallback

**Workflow**: select provider → `pool.Next()` → call API → success / mark key cooldown 30 min → retry next SK in same provider → all exhausted → next provider → all failed → Baidu web search fallback

## MCP Tools

### `smartsearch` — General Web Search

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `query` | string | ✅ | Search keyword |
| `intent` | string | ❌ | Search intent (only effective when LLM is enabled) |
| `time_range` | int | ❌ | Search time range in months, default 3. `1`=last month, `6`=last 6 months, `12`=last year, `0`=unlimited |

Results include engine source and relevance score by default (for engines that support scores like Tavily). Disable via `smartsearch.show_meta: false`.

**Engine name reference** (for `smartsearch.engines` config):

| Config Name | Engine | Returns Score |
|-------------|--------|--------------|
| `tavily_api` | Tavily Search API | ✅ |
| `exa` | Exa Web Search API | ❌ |
| `baidu_api` | Baidu Qianfan Search (`enable_ai_search` controls endpoint) | ❌ |
| `baidu` | Baidu Web Search (built-in) | ❌ |
| `bing` | Bing (built-in) | ❌ |
| `google` | Google (disabled by default, anti-bot blocked) | ❌ |
| `duckduckgo` | DuckDuckGo (needs proxy) | ❌ |

### `academicsearch` — Academic Paper Search

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `query` | string | ✅ | Search keyword |
| `engines` | []string | ❌ | Engine subset: `arxiv` `crossref` `openalex` `pubmed` `semantic_scholar` `google_scholar` |
| `time_range` | string | ❌ | `year` / `month` / `week` / `day` |
| `page` | int | ❌ | Page number, default 1 |

Results are ranked by the academic scoring enhancement (enabled by default): RRF fusion ranking + citation / journal authority / PDF availability / recency signals, with low-score papers auto-filtered (Top-1 + per-engine floor). Config: `academic.enhance` (default true), `academic.threshold` (default 0.02).

### `cleanfetch` — Web Content Fetch

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `url` | string | ✅ | Web page URL |

Requires `cleanfetch.enabled: true`. Based on go-webfetch, no proxy needed; built-in DNS rebinding protection and HEAD pre-check for large files (`max_fetch_size_mb` controls threshold, default 10MB); falls back to Jina Reader on failure (requires `jina.api_key`, proxy auto-detected).

### `pdf_parser` — PDF Parsing

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | ✅ | Local PDF file path or remote URL |

Requires `pdf_parser.enabled: true`. Large documents auto-stored to temp files.

**MinerU AI Enhancement** (optional): configure `mineru_token` to enable MinerU parsing with intelligent table/formula/multi-column/image recognition.
- With Token: Standard API, supports remote URLs (≤200MB/200 pages)
- Without Token: Agent Lightweight API, local file signed upload (≤10MB/20 pages)
- Get Token: https://mineru.net/apiManage
- Environment variable: `MINERU_TOKEN`

> Tool registration conditions: `smartsearch` needs `bing.enabled=true`; `academicsearch` needs `academic.enabled=true`; `cleanfetch` needs `cleanfetch.enabled=true`; `pdf_parser` needs `pdf_parser.enabled=true`.

## Subcommands

| Command | Description |
|---------|-------------|
| `start` | Start service (ref=1 or ref+1) |
| `stop` | Decrease reference (ref-1, graceful exit at zero) |
| `kill` | Force terminate (ignores reference count) |
| `status` | Show status, port, reference count |
| `version` | Show version (injected at build time, default `dev`) |
| `install` | Install Windows auto-start |
| `uninstall` | Uninstall Windows auto-start |

CLI flags: `-c, --config` to specify config file path.

## Configuration

See [docs/config.en.md](docs/config.en.md) — full config options, defaults, environment variable overrides.

**Minimal config (zero keys)**:

```yaml
port: 8338
mode: engine
```

## Usage Guide

### Windows Auto-Start

```bash
./websearch-mcpserver.exe install   # Creates VBS script + Startup folder shortcut
./websearch-mcpserver.exe uninstall # Removes shortcut
```

Uses COM API (ole32.dll) to create shortcuts, no PowerShell dependency.

### MCP Hooks Auto Start/Stop

Recommended: use Hooks for automatic session lifecycle (Qwen Code example):

```json
{
  "hooks": {
    "SessionStart": [{ "matcher": "*", "hooks": [{ "type": "command", "command": "/path/to/websearch-mcpserver start", "timeout": 10000 }] }],
    "SessionEnd":   [{ "matcher": "*", "hooks": [{ "type": "command", "command": "/path/to/websearch-mcpserver stop",  "timeout": 10000 }] }]
  }
}
```

Reference counting ensures multi-session sharing; auto-exits when all sessions close.

### Background Service

| Platform | Solution |
|----------|----------|
| Windows | `nssm` register as Windows Service |
| Linux | systemd `Restart=always` |
| macOS | launchd plist |

With a background service, `start` once only — no hooks needed.

#### Linux Auto-Start (systemd)

```bash
# 1. Create systemd service file
sudo tee /etc/systemd/system/websearch.service << 'EOF'
[Unit]
Description=WebSearch MCP Server
After=network.target

[Service]
Type=simple
User=YOUR_USERNAME
ExecStart=/usr/local/bin/websearch-mcpserver start
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# 2. Enable and start
sudo systemctl daemon-reload
sudo systemctl enable websearch
sudo systemctl start websearch

# 3. Check status
sudo systemctl status websearch
```

#### macOS Auto-Start (launchd)

```bash
# 1. Create plist file
tee ~/Library/LaunchAgents/com.websearch.server.plist << 'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.websearch.server</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/websearch-mcpserver</string>
        <string>start</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>WorkingDirectory</key>
    <string>/tmp</string>
</dict>
</plist>
EOF

# 2. Load and start
launchctl load ~/Library/LaunchAgents/com.websearch.server.plist
launchctl start com.websearch.server

# 3. Check status
launchctl list | grep websearch
```

### Academic Search Tips

- Medicine/Biology → `pubmed`; CS/AI → `arxiv` + `semantic_scholar`; All fields → `crossref` + `openalex`
- Keep `network: china` for domestic access; overseas engines auto-skipped
- Semantic Scholar / Google Scholar disabled by default; set `disable_semantic_scholar: false` / `disable_google_scholar: false` to enable; proxy auto-detected

## Qwen Code Sub-Agent Extension: web-researcher

[web-researcher](https://github.com/daidaiJ/web-researcher) is a companion Qwen Code extension for websearch-mcpserver that offloads web research tasks from the main agent to a dedicated sub-agent, **keeping the main model's context window clean**.

### Why

The main model's context window in Qwen Code is precious. Directing the main agent to do web research — searching, fetching pages, filtering information — floods the context with raw content, squeezing out space for coding conversations.

web-researcher solves this: a dedicated fast model sub-agent (default `deepseek-v4-flash`) independently handles the full pipeline of search → filter → fetch → synthesize, generates a structured report saved locally, and returns only a concise summary to the main agent.

### Key Features

| Feature | Description |
|---------|-------------|
| 🔍 **Research Offloading** | `/research:search <query>` dispatches a research task; the sub-agent automatically decomposes queries, searches in parallel, filters, fetches, and generates a report |
| 📊 **Report Management** | `/research:reports` lists historical reports · `/research:read <keyword>` search and read reports by keyword |
| 📐 **Structured Reports** | Reports use `FINDING-*`, `DATA-*`, `CONFLICT-*` anchors; the main agent greps for specific sections on demand without loading the full report |

### Installation

In Qwen Code:

```bash
/extension-creator ~/.qwen/extensions/web-researcher
```

Then place the [web-researcher](https://github.com/daidaiJ/web-researcher) contents into that directory.

Or clone manually:

```bash
git clone https://github.com/daidaiJ/web-researcher.git ~/.qwen/extensions/web-researcher
```

### Usage

```
# One-click research
/research:search MCP protocol specification 2025

# Browse historical reports
/research:reports MCP

# Deep-dive on demand
/research:read MCP protocol
```

Reports are stored in `.qwen/research/` under the project directory and can be referenced across sessions.

## Embed as Go Module

```go
import ("websearch/pkg/config"; "websearch/server")

conf, _ := config.Load("config.yaml")
srv := server.New()
srv.SetRefCount(1)
srv.Run(*conf) // Full hosting

// Or get just the Handler to embed in existing HTTP Server
handler := srv.Handler(*conf)
```

See [docs/api.en.md](docs/api.en.md).

## LiteLLM Integration

```yaml
search_tools:
  - search_tool_name: searxng-search
    litellm_params:
      search_provider: searxng
      api_base: http://localhost:8338/searxng
```

## Operations Reference

| Item | Description |
|------|-------------|
| **Health Check** | `GET /__admin/health` — returns `{"ref_count": N, "message": "running"}`, remotely accessible |
| **Admin Endpoints** | `GET /__admin/status` · `POST /__admin/refcount` · `POST /__admin/shutdown` — local access only |
| **PID File** | `.websearch.pid` (JSON), in config file directory or executable directory |
| **Log File** | `websearch.log`, same directory, size-rotated (default 1MB, 1 day retention) |
| **Cache** | SQLite WAL mode, 6h expiry (based on last hit), 30min scheduled cleanup |

## Troubleshooting

| Problem | Solution |
|---------|----------|
| Tools unavailable after start | Check `mode` and corresponding keys; `cleanfetch` needs `cleanfetch.enabled: true` |
| Academic search timeout | Check `network` setting; overseas engines need proxy (auto-detected by default) |
| Port in use | `status` to check if already running, or `kill` then restart |
| Stale cache results | Cache auto-expires after 6h, or delete `cache.storage_path` file and restart |
| Docker container exits immediately | Confirm `config.yaml` is mounted, check log output |
| Process still running after stop | Wait up to 10s; if still running use `kill` to force terminate |
| No results or rate-limited | Check `rate_limit` config (default 3/s, 60/min); Google etc. auto-skipped when proxy unavailable |

## Changelog

See [CHANGELOG.en.md](CHANGELOG.en.md)
