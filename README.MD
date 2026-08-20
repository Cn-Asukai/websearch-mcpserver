# websearch-mcpserver

> 轻量级 Web Search MCP Server — 零 API Key 即可运行

[English](README.EN.md) | [中文](README.MD)

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev/)
[![Release](https://img.shields.io/github/v/release/daidaiJ/websearch-mcpserver)](https://github.com/daidaiJ/websearch-mcpserver/releases)

---

## 简介

用 Go 开发的 MCP 搜索服务，内置百度网页搜索、Bing、DuckDuckGo 通用搜索引擎和 6 个学术搜索引擎。支持 Claude Code、Qwen Code、Cursor 等 MCP 客户端，也可作为 Go 模块嵌入。

## 设计亮点

- **零依赖启动** — `engine` 模式无需任何 API Key，内置百度网页搜索 + Bing 双引擎并发，代理可用时自动加入 DuckDuckGo
- **API Key 池** — `apipool` 模式：每次只调一个供应商，失败自动切换下一个，支持 `round-robin`（跨请求轮转）和 `priority`（固定优先级）两种策略；各供应商均支持 `sk_list` 多 Key 轮转，Key 失效 30 分钟自动恢复，同供应商内自动重试下一个 SK
- **百度智能搜索** — `baidu` 模式默认使用千帆 `chat/completions` 端点，不配置 model 时走免费百度搜索（不产生 LLM 费用）；配置 `model` 启用 LLM 智能搜索生成
- **系统代理自动检测** — 默认读取 Windows 注册表 / 环境变量，Clash 等代理软件开启系统代理后 DuckDuckGo、学术引擎、Jina Reader 自动走代理，无需手动配置
- **智能限流重试** — 所有 HTTP 客户端自动处理 429 限流（读取 `Retry-After` 头等待后重试），arXiv 引擎内置 1 req/s 限流器避免触发限流
- **多引擎并行编排** — 学术搜索并发请求多个引擎，结果按 URL 去重 + 分组归一化排序；hybrid 模式下多引擎原生混合
- **智能相关性评分** — 多引擎结果 RRF 融合排名，叠加词汇对齐、稀有词/短语连续匹配、域名品质惩罚（品牌/电商/词典站误匹配降权）与多引擎共识 / 权威站 / 时效性加分；全局低分阈值过滤真正裁剪低质量上下文（Top-1 保护、最少保留 2 条），纯启发式无需 AI 模型；末端 **MMR 多样性重排**（默认 λ=0.7）打散同主题高相似结果（转载/镜像站）
- **学术搜索评分增强** — 六大学术引擎 RRF 融合排名，叠加引用数（对数压缩）、高影响力期刊/会议、PDF 全文可用性、新鲜度信号，低分论文自动过滤（Top-1 + 每引擎保底）
- **LLM 摘要流式推送** — 摘要阶段通过 MCP progress notification 逐 token 实时推送生成过程，客户端断开自动取消，失败自动回退非流式摘要
- **智能回退** — 百度 SK 失败自动回退网页搜索；主引擎失败自动回退 Bing；LLM 摘要失败回退原始结果；cleanfetch 失败回退 Jina Reader；缓存异常跳过缓存
- **增强型网页抓取** — 基于 go-webfetch，无需代理即可抓取网页，TLS 指纹伪装（Chrome 131）+ 重试退避 + 系统代理支持；内置 SSRF 防护、DNS rebinding 检测、HEAD 预检防大文件和 WAF 检测，大内容自动存储到临时文件
- **PDF 本地优先 + MinerU OCR 回退** — `pdf_parser` 优先用 PDF 库提取文本；读不到文本（扫描件/图片型）且开启 `mineru_ocr` 时回退 MinerU OCR；有 Token 时远程 URL 可用精准 API（≤200MB），无 Token 走 Agent 轻量 API（≤10MB）
- **引用计数进程管理** — 多客户端共享同一实例，归零自动优雅退出
- **子智能体扩展** — 可选配套 [web-researcher](https://github.com/daidaiJ/web-researcher) 扩展，将网络调研卸载到 fast model 子智能体，主模型上下文零膨胀（详见 [Qwen Code 子智能体扩展](#qwen-code-子智能体扩展web-researcher)）
- **纯 Go 无 CGO** — SQLite 使用 `modernc.org/sqlite`，单二进制部署

## 特性一览

| 类别 | 能力 |
|------|------|
| **通用搜索** | 百度千帆、百度网页搜索（内置）、Tavily、Exa、Bing（内置）、DuckDuckGo（自动检测代理）、Google（默认禁用，需显式启用） |
| **学术搜索** | arXiv、Crossref、OpenAlex、PubMed（国内直连）+ Semantic Scholar、Google Scholar（自动检测代理）；RRF 融合 + 引用数 / 期刊权威 / PDF / 新鲜度信号评分，低分过滤（每引擎保底） |
| **MCP 工具** | `smartsearch` 网络检索 · `academicsearch` 论文检索 · `cleanfetch` 网页抓取 · `pdf_parser` PDF 解析 |
| **缓存** | SQLite 自动缓存，6h 过期，后台定时清理 |
| **LLM 摘要** | 可选接入 OpenAI 兼容 API，生成结构化摘要；支持 MCP progress notification 逐 token 流式推送生成过程 |
| **站点屏蔽** | 全局 `black_list_host`，自动过滤低质量站点 |
| **全局限流** | `rate_limit` 统一配置所有搜索引擎的请求速率 |
| **相关性评分** | RRF 融合排名 + 域名品质 / 词汇对齐 / 共识 / 权威 / 新鲜度加分，全局低分阈值裁剪低质量上下文；MMR 多样性重排打散同主题高相似结果；per-engine `min_score` / `max_size`，`show_meta` 展示来源与分数 |
| **时间范围** | `smartsearch` 工具支持 `time_range` 参数（月为单位），API 引擎统一支持时间范围过滤 |
| **SearXNG 兼容** | `/searxng/search` 端点，可对接 LiteLLM |

## 快速开始

```bash
# 1. 下载: https://github.com/daidaiJ/websearch-mcpserver/releases
# 2. 启动（零配置）
./websearch-mcpserver start
# 3. 添加到客户端（见下方「安装部署 > 注册 MCP 客户端」）
```

## 安装部署

### 二进制下载

**HTTP daemon**（`start` 后监听 `/mcp`）：

| 平台 | 文件 |
|------|------|
| Linux x86_64 | `websearch-mcpserver-linux-amd64` |
| Windows x86_64 | `websearch-mcpserver-windows-amd64.exe` |
| macOS Intel | `websearch-mcpserver-darwin-amd64` |
| macOS Apple Silicon | `websearch-mcpserver-darwin-arm64` |

**stdio CLI**（由 MCP 客户端拉起，无 HTTP 端口）：

| 平台 | 文件 |
|------|------|
| Linux x86_64 | `websearch-mcp-cli-linux-amd64` |
| Windows x86_64 | `websearch-mcp-cli-windows-amd64.exe` |
| macOS Intel | `websearch-mcp-cli-darwin-amd64` |
| macOS Apple Silicon | `websearch-mcp-cli-darwin-arm64` |

SHA256 校验：每个二进制旁附带对应 `.sha256`。

Release 页面：https://github.com/daidaiJ/websearch-mcpserver/releases

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

### 源码构建

```bash
go build -o websearch ./cmd/
go build -o websearch-mcp-cli ./cmd/cli
# 注入版本号
go build -ldflags="-X main.version=v1.0.0" -o websearch ./cmd/
go build -ldflags="-X main.version=v1.0.0" -o websearch-mcp-cli ./cmd/cli
```

### 注册 MCP 客户端

服务启动后，按需注册到对应客户端：

```bash
# Claude Code
claude mcp add --transport http websearch http://localhost:8338/mcp

# Qwen Code — 在 settings.json 的 mcpServers 中添加：
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
# Cursor / 其他 MCP 客户端 — type=http, url=http://localhost:8338/mcp
```

### stdio CLI

不启动 HTTP 服务时，下载 `websearch-mcp-cli-*`，由客户端直接拉起进程（默认走 stdin/stdout）。无配置文件时使用 `mode: engine` 内存默认值。

```bash
# 可选：写出示例配置
./websearch-mcp-cli init
# 或指定路径
./websearch-mcp-cli -c ~/.config/websearch/config.yaml init
```

```json
{
  "mcpServers": {
    "websearch": {
      "command": "/path/to/websearch-mcp-cli",
      "args": ["-c", "/path/to/config.yaml"]
    }
  }
}
```

无配置文件时可省略 `args`。日志写到 stderr 与配置目录下的 `websearch.log`，不会污染 stdout 上的 JSON-RPC。

CLI 子命令：无参数 = stdio；`init` 写示例配置；`version` 显示版本。

配置与 HTTP 共用同一 YAML；`port` 等差异见 [docs/config.md#stdio-cli-配置说明](docs/config.md#stdio-cli-配置说明)。

### Agent 快速部署

Agent 可按以下步骤自动完成安装：

```bash
# 1. 下载最新二进制
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

# 2. 写入最小配置（零 Key 即可运行）
mkdir -p ~/.config/websearch
cat > ~/.config/websearch/config.yaml << 'EOF'
port: 8338
mode: engine
EOF

# 3. 启动并注册（见上方「注册 MCP 客户端」）
./websearch-mcpserver start
```

Windows 自启动（可选）：下载后执行 `websearch-mcpserver.exe install`。

## 搜索模式

| 模式 | 说明 | 需要 Key |
|------|------|----------|
| `baidu` | 百度千帆搜索（`enable_ai_search` 控制端点），失败自动回退百度网页搜索；无 SK 时直接用百度网页搜索 | `BAIDU_SK`（可选） |
| **`apipool`** | **API Key 池轮转**：每次只调一个供应商，失败自动切换；支持 `round-robin`（轮转）/ `priority`（优先级）策略；同供应商内自动重试下一个 SK；百度网页搜索兜底 | 各 Key 可选 |
| `tavily` | Tavily Search API | `TAVILY_SK` |
| `exa` | Exa Web Search API | `EXA_API_KEY` |
| `hybrid` | 全引擎混合（百度智能搜索 + 百度网页搜索 + Tavily + Exa + Bing + DuckDuckGo） | 各 Key 可选 |
| **`engine`** | **百度网页搜索 + Bing**（代理可用时自动加入 DuckDuckGo） | **无需** |

> 所有模式下主引擎失败均自动回退。无 Key 时自动降级为 `engine`。`baidu`/`tavily`/`exa` 均支持 `sk_list` 多 Key 轮询，`sk_list` 为空时自动用 `api_key` 作为单元素列表。`apipool` 模式的供应商顺序和策略可通过 `apipool` 配置节自定义，详见 [Apipool 配置](#apipool-配置)。

### SmartSearch 高级配置

`smartsearch` 节控制搜索结果的过滤、截断和输出格式：

```yaml
smartsearch:
  max_size: 10           # 全局最大结果数（按 score 排序后截断），0 = 不限
  show_meta: true        # 输出中显示引擎来源和相关性分数（默认 true）
  enhance: true          # 本地评分增强（RRF 融合 + 词汇对齐 + 域名品质 + 多层 Boost + 阀值过滤），默认 true
  relevance_threshold: 0.05  # 增强后相关性阀值，低于此值过滤（Top-1 保护），默认 0.05
  mmr:                       # MMR 多样性重排（打散同主题高相似结果）
    enabled: true            # 总开关（默认 true）
    lambda: 0.7              # 相关性权重 [0,1]，越高越偏相关性，越低越偏多样性（默认 0.7）
    target_count: 0          # MMR 后目标条数，0 = 不额外截断（由 max_size 统一截断）
  engines:
    tavily_api:        # Tavily API（回传 score，可设 min_score）
      min_score: 0.5   # 最低相关性分数阈值，0 = 不过滤
      max_size: 6      # 单引擎最大结果数（默认 4）
    bing:              # Bing（不回传 score，min_score 无效）
      max_size: 4
    baidu_api:         # 百度千帆搜索（不回传 score，enable_ai_search 控制端点）
      max_size: 5
    baidu:             # 百度网页搜索（不回传 score）
      max_size: 5
    google:            # Google（默认禁用，被反爬拦截）
      max_size: 4
    duckduckgo:        # DuckDuckGo（不回传 score，需代理）
      max_size: 4
```

**Score 过滤逻辑**：
- 引擎回传 score 时：按 `min_score` 过滤，保留 `max_size` 条
- 引擎不回传 score 时：忽略 `min_score`，取 `min(max_size, ⌈global_max_size / 引擎数⌉)` 截断
- 全局 `max_size`：有 score 时按 score 排序截断，无 score 时按引擎轮询均匀分配

### Apipool 配置

`apipool` 节控制 `mode: apipool` 时的供应商选择策略和优先级顺序：

```yaml
apipool:
  strategy: round-robin   # round-robin（默认）/ priority
  engines:                # 供应商优先级顺序（默认 [baidu, tavily, exa]）
    - baidu
    - tavily
    - exa
```

**策略说明**：
- **`round-robin`**（默认）：跨请求轮转起始供应商，每次请求内先用完当前供应商所有可用 SK 再 fallback 到下一个
- **`priority`**：始终从列表第一个供应商开始，用完所有 SK → 切换下一个供应商 → 百度网页搜索兜底

**工作流程**：选供应商 → `pool.Next()` 取 key → 调 API → 成功返回 / 失败标记 key 冷却 30 分钟 → 同供应商下一个 SK 重试 → 全部耗尽切下一个供应商 → 全部失败用百度网页搜索兜底

## MCP 工具

### `smartsearch` — 通用网络检索

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `query` | string | ✅ | 搜索关键词 |
| `intent` | string | ❌ | 搜索意图（仅 LLM 启用时生效） |
| `time_range` | int | ❌ | 搜索时间范围（月），默认 3。`1`=近 1 个月，`6`=近半年，`12`=近一年，`0`=不限 |

返回结果默认附带来源引擎和相关性分数（Tavily 等支持 score 的引擎）。可通过 `smartsearch.show_meta: false` 关闭。

**引擎名称对照**（配置 `smartsearch.engines` 时使用）：

| 配置名 | 引擎 | 回传 score |
|--------|------|-----------|
| `tavily_api` | Tavily Search API | ✅ |
| `exa` | Exa Web Search API | ❌ |
| `baidu_api` | 百度千帆搜索（`enable_ai_search` 控制端点选择） | ❌ |
| `baidu` | 百度网页搜索（内置） | ❌ |
| `bing` | Bing（内置） | ❌ |
| `google` | Google（默认禁用，被反爬拦截） | ❌ |
| `duckduckgo` | DuckDuckGo（需代理） | ❌ |

### `academicsearch` — 学术论文检索

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `query` | string | ✅ | 搜索关键词 |
| `engines` | []string | ❌ | 引擎子集：`arxiv` `crossref` `openalex` `pubmed` `semantic_scholar` `google_scholar` |
| `time_range` | string | ❌ | `year` / `month` / `week` / `day` |
| `page` | int | ❌ | 页码，默认 1 |

结果按学术评分增强排序（默认开启）：RRF 融合排名 + 引用数 / 期刊权威 / PDF 全文 / 新鲜度信号，低分论文自动过滤（Top-1 + 每引擎保底）。配置项：`academic.enhance`（默认 true）、`academic.threshold`（默认 0.02）。

### `cleanfetch` — 网页内容抓取

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `url` | string | ✅ | 网页 URL |

需配置 `cleanfetch.enabled: true`。基于 go-webfetch，无需代理；内置 DNS rebinding 防护和 HEAD 预检防大文件（`max_fetch_size_mb` 控制阈值，默认 10MB）；失败时自动回退 Jina Reader（需配置 `jina.api_key`，代理自动检测）。

### `pdf_parser` — PDF 解析

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `path` | string | ✅ | 本地 PDF 文件路径或远程 URL |

需配置 `pdf_parser.enabled: true`。大文档自动存储到临时文件。

**解析策略**：本地 PDF 优先用 PDF 库（ledongthuc/pdf）提取文本；无文本层时若开启 `mineru_ocr` 再回退 MinerU OCR。
- `mineru_ocr: true`：扫描件 / 图片型 PDF 的 OCR 回退（无 Token 走 Agent 轻量 API，≤10MB/20页）
- `mineru_token`：远程 URL 精准解析 API（≤200MB/200页）；也可与 OCR 回退共用
- 获取 Token：https://mineru.net/apiManage
- 环境变量：`MINERU_TOKEN`

> 工具注册条件：`smartsearch` 需 `bing.enabled=true`；`academicsearch` 需 `academic.enabled=true`；`cleanfetch` 需 `cleanfetch.enabled=true`；`pdf_parser` 需 `pdf_parser.enabled=true`。

## 子命令

HTTP daemon 二进制 `websearch-mcpserver`：

| 命令 | 说明 |
|------|------|
| `start` | 启动服务（ref=1 或 ref+1） |
| `stop` | 减少引用（ref-1，归零优雅退出） |
| `kill` | 强制结束（无视引用计数） |
| `status` | 查看状态、端口、引用计数 |
| `version` | 显示版本号（构建时注入，默认 `dev`） |
| `install` | 安装 Windows 开机自启动 |
| `uninstall` | 卸载 Windows 自启动 |

CLI 参数：`-c, --config` 指定配置文件路径。

## 配置

详见 [docs/config.md](docs/config.md) — 完整配置项、默认值、环境变量覆盖；stdio CLI 特殊说明见 [stdio CLI 配置说明](docs/config.md#stdio-cli-配置说明)。

**最小配置（零 Key 启动）**：

```yaml
# HTTP daemon 需要 port；stdio CLI 可只写 mode
port: 8338
mode: engine
```

## 使用指南

### Windows 开机自启动

```bash
./websearch-mcpserver.exe install   # 生成 VBS 脚本 + 启动目录快捷方式
./websearch-mcpserver.exe uninstall # 移除快捷方式
```

使用 COM API（ole32.dll）创建快捷方式，不依赖 PowerShell。

### MCP Hooks 自动启停

推荐通过 Hooks 实现会话自动启停，以 Qwen Code 为例：

```json
{
  "hooks": {
    "SessionStart": [{ "matcher": "*", "hooks": [{ "type": "command", "command": "/path/to/websearch-mcpserver start", "timeout": 10000 }] }],
    "SessionEnd":   [{ "matcher": "*", "hooks": [{ "type": "command", "command": "/path/to/websearch-mcpserver stop",  "timeout": 10000 }] }]
  }
}
```

引用计数确保多会话共享实例，全部关闭后自动退出。

### 后台常驻

| 平台 | 方案 |
|------|------|
| Windows | `nssm` 注册为 Windows Service |
| Linux | systemd `Restart=always` |
| macOS | launchd plist |

常驻模式下 `start` 一次即可，无需 hooks。

#### Linux 开机自启动（systemd）

```bash
# 1. 创建 systemd 服务文件
sudo tee /etc/systemd/system/websearch.service << 'EOF'
[Unit]
Description=WebSearch MCP Server
After=network.target

[Service]
Type=simple
User=你的用户名
ExecStart=/usr/local/bin/websearch-mcpserver start
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# 2. 启用并启动服务
sudo systemctl daemon-reload
sudo systemctl enable websearch
sudo systemctl start websearch

# 3. 查看状态
sudo systemctl status websearch
```

#### macOS 开机自启动（launchd）

```bash
# 1. 创建 plist 文件
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

# 2. 加载并启动
launchctl load ~/Library/LaunchAgents/com.websearch.server.plist
launchctl start com.websearch.server

# 3. 查看状态
launchctl list | grep websearch
```

### 学术搜索建议

- 医学/生物 → `pubmed`；CS/AI → `arxiv` + `semantic_scholar`；全学科 → `crossref` + `openalex`
- 国内保持 `network: china`，海外引擎自动跳过
- Semantic Scholar / Google Scholar 默认禁用，设置 `disable_semantic_scholar: false` / `disable_google_scholar: false` 即可启用，代理自动检测

## Qwen Code 子智能体扩展：web-researcher

[web-researcher](https://github.com/daidaiJ/web-researcher) 是 websearch-mcpserver 的配套 Qwen Code 扩展，将网络调研任务从主智能体卸载到专用子智能体，**保持主模型上下文窗口干净**。

### 为什么需要

Qwen Code 主模型的上下文窗口非常宝贵。直接让主智能体做网络调研——搜索、抓取网页、筛选信息——大量原始内容会涌入上下文，挤占后续编码对话的空间。

web-researcher 的解法：用一个专用的 fast model 子智能体（默认 `deepseek-v4-flash`）独立完成搜索→筛选→抓取→综合的全流程，生成结构化报告存到本地，只把精炼的摘要返回给主智能体。

### 主要功能

| 功能 | 说明 |
|------|------|
| 🔍 **网络调研卸载** | `/research:search <query>` 一键派发调研任务，子智能体自动拆解子问题、并行搜索、筛选抓取、生成报告 |
| 📊 **报告管理与回溯** | `/research:reports` 列出历史报告 · `/research:read <keyword>` 按关键词检索，分层读取 |
| 📐 **结构化报告** | 报告采用 `FINDING-*`、`DATA-*`、`CONFLICT-*` 锚点标记，主智能体按需 grep 定位，无需加载整份报告 |

### 安装

在 Qwen Code 中执行：

```bash
/extension-creator ~/.qwen/extensions/web-researcher
```

然后将 [web-researcher](https://github.com/daidaiJ/web-researcher) 的内容放入该目录。

或手动克隆：

```bash
git clone https://github.com/daidaiJ/web-researcher.git ~/.qwen/extensions/web-researcher
```

### 使用示例

```
# 一键调研
/research:search MCP protocol specification 2025

# 查看历史报告
/research:reports MCP

# 按需深入阅读
/research:read MCP protocol
```

调研结果存放在项目下 `.qwen/research/` 目录，支持跨会话回溯引用。

## 作为 Go 模块嵌入

```go
import ("websearch/pkg/config"; "websearch/server")

conf, _ := config.Load("config.yaml")
srv := server.New()
srv.SetRefCount(1)
srv.Run(*conf) // 完整托管

// 或仅获取 Handler 嵌入已有 HTTP Server
handler := srv.Handler(*conf)
```

详见 [docs/api.md](docs/api.md)。

## LiteLLM 集成

```yaml
search_tools:
  - search_tool_name: searxng-search
    litellm_params:
      search_provider: searxng
      api_base: http://localhost:8338/searxng
```

## 运维参考

| 项目 | 说明 |
|------|------|
| **健康检查** | `GET /__admin/health` — 返回 `{"ref_count": N, "message": "running"}`，远程可访问 |
| **Admin 端点** | `GET /__admin/status` · `POST /__admin/refcount` · `POST /__admin/shutdown` — 仅本地访问 |
| **PID 文件** | `.websearch.pid`（JSON 格式），位于配置文件目录或可执行文件目录 |
| **日志文件** | `websearch.log`，同目录，按大小滚动（默认 1MB，保留 1 天） |
| **缓存** | SQLite WAL 模式，6h 过期（基于最近命中），30min 定时清理 |

## 排障

| 问题 | 排查 |
|------|------|
| 启动后工具不可用 | 检查 `mode` 和对应 Key；`cleanfetch` 需 `cleanfetch.enabled: true` |
| 学术搜索超时 | 检查 `network` 设置；海外引擎需代理（默认自动检测系统代理） |
| 端口被占用 | `status` 查看是否已运行，或 `kill` 后重启 |
| 缓存结果过旧 | 缓存 6h 自动过期，或删除 `cache.storage_path` 文件重启 |
| Docker 容器立即退出 | 确认挂载了 `config.yaml`，检查日志输出 |
| stop 后进程仍在 | 等待最多 10s；若仍在用 `kill` 强制结束 |
| 搜索无结果或限流 | 检查 `rate_limit` 配置（默认 3/s, 60/min）；Google 等引擎代理不可用时自动跳过 |

## 变更日志

详见 [CHANGELOG.md](CHANGELOG.md)
