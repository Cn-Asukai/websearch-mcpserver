# Wave 2 — P1：隐私默认、显式预设配置

> 对应任务：[T03](../tasks/T03-mineru-pdf-only.md) [T04](../tasks/T04-explicit-preset-config.md)
> **不做：** Issue P1-3（CI push/PR 门禁）。提交前本地跑 mock/单测即可，不建 `ci.yml`。

---

## P1-1 MinerU Token 把所有远程 URL 先发给 mineru.net

### 现状

`pkg/webfetch/webfetch.go:112-122`：远程 URL 只要 `HasToken()` 就先 `mineru.ParseURL`，失败再 webfetch。不限 PDF。

这直接打脸 README「查询只发给搜索引擎本身 / 本地优先」。普通网页多一次远程往返，并消耗 MinerU 日额度。

本地 `file://` 路径已经是「本地 PDF 库 → 按需 OCR」，这条是对的，不要改坏。

### 方案

仅当目标像 PDF 时才走精准 API：

1. URL path 以 `.pdf` 结尾（忽略 query/fragment，大小写不敏感）。
2. 可选：webfetch HEAD 得到 `Content-Type: application/pdf`（已有 HEAD 预检的话复用，不要额外打一轮）。

不新增配置项，硬编码「仅 PDF URL」。日志：命中 MinerU 时打 URL 类型（pdf），未命中则静默走 webfetch。

### 测试

- 有 Token + `https://example.com/a.html` → 不调用 ParseURL（mock mineru 或记录调用次数）。
- 有 Token + `https://cdn.example.com/x.PDF?download=1` → 调用 ParseURL。
- 无 Token → 行为不变。

---

## P1-2 「零配置」= 生成可编辑的预设 yaml，不是内存隐式默认值

Issue 建议 `cmd/main.go` 改 `LoadOrDefault`。**不采纳。** 产品意图是用户手里有一份显式 `config.yaml` 可改，而不是进程里一套看不见的 `Default()`。

### 现状（已核实）

这条链路其实已经按「写出预设文件」在做，只是文档把「不用手写配置」说成了「直接 start」：

| 入口 | 行为 |
|------|------|
| `websearch-mcpserver.exe install` | 若 exe 旁无 `config.yaml`，写入 `config.ExampleConfig`；VBS 设 `WEBSEARCH_CONFIG` 指向该文件（`cmd/setup_windows.go:200-210`） |
| `websearch-mcp-cli init` | 同样写出 `ExampleConfig`，已存在则跳过（`cmd/cli/main.go:34-52`） |
| `websearch-mcpserver start` | `config.Load()`，找不到文件直接退出（`cmd/main.go:170`） |
| 非 Windows `install` | 空实现，只打印不支持（`cmd/setup_other.go`） |

`ExampleConfig` 来自 `pkg/config/content.go`，与 `config.example.yaml` 同源。`LoadOrDefault` 给 CLI stdio 无文件时用，daemon 不应走这条。

README「`./websearch-mcpserver start` 零配置」过严：Windows 预期是先 `install` 得到可改 yaml；其它平台是 `cli init`。直接 `start` 失败是缺文件，不是漏接 `LoadOrDefault`。

### 方案

1. **禁止** daemon `start` 使用 `LoadOrDefault` / 静默 `Default()`。
2. 抽出共用 `config.EnsureExampleFile(path)`：文件不存在则写 `ExampleConfig`，已存在不覆盖。`install`、`cli init`、以及下面的 first-run 都走它。
3. `start` 在**未指定** `-c` / `WEBSEARCH_CONFIG`、且搜索路径找不到 yaml 时：在可执行文件目录写出 `config.yaml`，打印绝对路径（「已生成预设配置，可直接编辑」），再 `Load`。用户显式传了路径但文件不存在 → 仍报错，不擅自创建（避免路径打错被静默写成新文件）。
4. 改文档，不要再暗示「没有任何 yaml 也能靠隐式默认跑」：
   - README 快速开始：Windows `install`（生成配置 + 自启动）或任意平台 `websearch-mcp-cli init` / 首次 `start` 会写出预设 → 再按需改 `config.yaml`。
   - `docs/installation.md`：写清「零配置」= 自动生成与 `config.example.yaml` 相同的预设文件，改端口/Key/模式都改这一份。

### 测试

- `EnsureExampleFile`：目标不存在 → 写出且内容含 `mode:`；已存在 → 不覆盖。
- `cli init` 现有测试保持。
- 不必为 `main` 写进程级测试。

---

## P1-3 CI 无 push/PR 门禁 — 不做

Issue 建议加 `ci.yml`（vet + test -race）。**不做。** 当前流程是本地跑完 mock 与单测再提交；仓库只保留 tag 触发的 `release.yml`。不要为这项开 workflow。
