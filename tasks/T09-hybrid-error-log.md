# T09 hybrid 记录单引擎失败

- Issue：[#2](https://github.com/daidaiJ/websearch-mcpserver/issues/2) P2-4
- 优先级：P2
- 状态：完成
- 依赖：无
- 方案：[plans/wave-3-p2.md](../plans/wave-3-p2.md)

## 要改什么

`pkg/search/hybrid.go` `mergeResults`：`r.err != nil` 时 `log.Warnf("hybrid: engine %s failed: %v", name, err)`，再 continue。全失败 error 可拼接各引擎错误，**禁止**把 API Key 打进日志。

不要在 `pkg/bing` / `pkg/google` / `pkg/ddg` 里加 log（保持 antirobot 层干净）。

## 验收

- 单测或假引擎：一个失败一个成功 → 仍返回成功结果
- 全失败 → 非空 error，且日志路径被覆盖到（可用 hook / 观察现有 zerolog；不强求测日志字符串）
