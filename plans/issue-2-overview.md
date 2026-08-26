# Issue #2 修复总方案

> 来源：[代码审查问题汇总（20 项）](https://github.com/daidaiJ/websearch-mcpserver/issues/2)
> 复核日期：2026-08-26
> 对照提交：`master` @ `f95745c`
> 原则：按代码现状拆，不照抄 issue 原文；已修的标出来，过时的纠正。

## 审查结论

Issue 骨架判断成立：工程质量不差（SSRF 双层、KeyPool 设计、流式取消、核心单测），但 **P0 两项仍是真实缺口**。20 项中：

| 结论 | 项 |
|------|----|
| 仍成立，按原建议修 | P0-1、P0-2、P1-1、P2-1、P2-2、P2-4、P2-5、P2-6、P3 全部 |
| 仍成立，方案需改方向 | P1-2（不是改 `LoadOrDefault`；零配置 = `install`/`init`/首次 `start` 写出可编辑的预设 yaml） |
| 不做 | P1-3（CI push/PR 门禁；本地跑完 mock/单测再提交） |
| 仍成立，方案需微调 | P2-3（主要打到 `apipool` 与「无 per-engine 配置」路径，不是所有单引擎都被静默截 4） |
| 部分已修，补缺口即可 | P2-7（`applyKnownEnv` 只在 `Default()` 调用，`Load()` 仍漏）、P2-8（Google/Tavily/Exa/Baidu 部分集成测试已 `testing.Short()`，Bing/Crossref/arXiv/Scholar 仍直连） |

Issue 里 `google_test.go:415`「无跳过机制」已过时：`TestGoogleSearch` 在 403 行已 `testing.Short()` skip。

## 不在本次范围

- Wigolo 评分管线（RRF / 域名惩罚等）已在 `feat/wigolo-search-enhancement`，与本 issue 无关。
- 给 `SearchInf` 全链路加 `context.Context`：P0 用全局 Timeout 止血；接口改造单开，避免和超时修复绑死。
- `config.test.yaml` 中的真实 Key：已在 `.gitignore`，本轮不处理。

## 分期

```
Wave 1  P0  ──►  可安全默认启动、上游不会永久挂起
Wave 2  P1  ──►  隐私默认、首次启动写出可编辑预设配置
Wave 3  P2  ──►  并发正确、故障可定位、配置/测试与文档对齐
Wave 4  P3  ──►  关机顺序、泄漏、缓存去重（可并入 Wave 3 尾部 PR）
```

建议落地为 **4 个 PR**（P3 可并入 Wave 3）。不要一个 PR 塞 20 项。

## 依赖

```
T02 上游超时  ──►  T06 Key 失效标记（无超时的重试会把问题放大）
T07 单 SearchGroup  与  T08 maxSize / T09 日志  无硬依赖，可并行，但 T07 先合更干净
T04 预设 yaml  与  T12 applyKnownEnv  都碰配置，可同一 PR 或紧挨着
T01 监听/鉴权  独立，但要改 config + 两个 example yaml + docs
T10 transport 缓存  与  T11 WinHTTP 越界  同属 proxy 包，建议同一 PR
```

## 配置变更约定

新增项必须同步：

- `pkg/config/config.go`
- `config.example.yaml`
- `pkg/config/config.example.yaml`
- `docs/configuration.md` + `docs/configuration.en.md`

环境变量前缀按现有风格（已有 `BAIDU_SK` / `TAVILY_SK` 等，不要强行改成 `WEBCRAWLER_`）。

## 验收总表

| Wave | 命令 / 行为 |
|------|-------------|
| 1 | 默认只绑 `127.0.0.1`；无 token 时局域网打 `/mcp`、`/searxng/search` 失败（若启用了 token）；黑洞上游 30s 内返回错误 |
| 2 | 无 yaml 时 `start`/`install` 写出可编辑的预设 `config.yaml`（不走隐式 Default）；带 MinerU Token 抓普通网页不打 mineru.net |
| 3 | 并发 MarkInvalid 标对 key；MCP 与 SearXNG 共用一套引擎；apipool `max_size` 生效；失败引擎有 Warn 日志 |
| 4 | 关机先 `Shutdown` 再关 DB；同 query 不堆行；`172.32.0.0` 不被当成内网 |

任务索引：[`../tasks/README.md`](../tasks/README.md)
