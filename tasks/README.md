# Tasks — Issue #2

按 [`plans/issue-2-overview.md`](../plans/issue-2-overview.md) 拆出的可执行任务。每个文件可单独交给一次 agent 会话。

## 执行顺序

| 顺序 | ID | 标题 | 优先级 | 依赖 |
|------|----|------|--------|------|
| 1 | [T01](T01-bind-and-auth.md) | 默认绑 127.0.0.1 + 业务端点可选 token | P0 | — |
| 2 | [T02](T02-upstream-timeout.md) | DefaultClient 30s 超时 | P0 | — |
| 3 | [T04](T04-explicit-preset-config.md) | 首次启动写出可编辑预设 config.yaml | P1 | — |
| 4 | [T12](T12-viper-env-backfill.md) | Load() 调用 applyKnownEnv | P2 | 可与 T04 同 PR |
| 5 | [T03](T03-mineru-pdf-only.md) | MinerU 仅处理 PDF URL | P1 | — |
| 6 | [T13](T13-network-tests-docs.md) | 集成测试 Short skip + 文档标注 | P2 | — |
| 7 | [T07](T07-share-searchgroup.md) | 共用 SearchGroup + SearXNG nil 守卫 | P2 | — |
| 8 | [T06](T06-keypool-mark-invalid.md) | 按实际 key 标记失效 | P2 | T02 |
| 9 | [T08](T08-single-engine-maxsize.md) | 去掉默认截 4 条 | P2 | — |
| 10 | [T09](T09-hybrid-error-log.md) | hybrid 记录引擎失败 | P2 | — |
| 11 | [T10](T10-proxy-transport-cache.md) | transport 缓存 + 代理检测 TTL | P2 | 与 T11 同 PR |
| 12 | [T11](T11-winhttp-utf16-bounds.md) | WinHTTP UTF-16 按 bufLen 扫描 | P2 | 与 T10 同 PR |
| 13 | [T14](T14-p3-hygiene.md) | P3 六项收尾 | P3 | T07 先合更顺（关机顺序碰 server.go） |

T01 与 T02 无相互依赖，可并行。[T05](T05-ci-quality-gate.md) 已取消（不建 CI workflow）。

## 状态图例

任务文件头部 `状态` 字段：`待办` / `进行中` / `完成`。落地时改这一行即可。
