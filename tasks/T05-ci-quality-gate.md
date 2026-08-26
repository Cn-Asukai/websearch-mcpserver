# T05 push/PR 质量门禁 — 取消

- Issue：[#2](https://github.com/daidaiJ/websearch-mcpserver/issues/2) P1-3
- 优先级：P1
- 状态：取消
- 依赖：无
- 方案：[plans/wave-2-p1.md](../plans/wave-2-p1.md)

## 原因

Issue 建议增加 push/PR 的 vet + `go test -race` workflow。当前开发习惯是本地跑完 mock 与单测再提交，不必每次 push 跑远程门禁。保留 tag 触发的 `release.yml` 即可。

**不要** 新增 `.github/workflows/ci.yml`。后续若有人按 issue 原文再提 CI，以本任务为准。
