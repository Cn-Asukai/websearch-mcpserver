# T14 P3 杂项收尾（6 项）

- Issue：[#2](https://github.com/daidaiJ/websearch-mcpserver/issues/2) P3-1～P3-6
- 优先级：P3
- 状态：完成
- 依赖：关机顺序碰 `server.go`，建议 [T07](T07-share-searchgroup.md) 先合
- 方案：[plans/wave-4-p3.md](../plans/wave-4-p3.md)

六项一起交一个 PR。

## P3-1 daemon.PostShutdown Close body

`pkg/daemon/daemon.go:138-141` 与 `PostRefCount` 一样 `defer resp.Body.Close()`。

## P3-2 CleanupScheduler.Stop 真正等待

`pkg/cache/cleanup.go`：用 WaitGroup/`done` 实现注释承诺的「阻塞等待」；`Stop(ctx)` 可被取消。

## P3-3 关机顺序

`server/server.go`：`Stop cleanup` → `srv.Shutdown` → `webfetch.Close` → `cache.Close` → `RemovePID`。

## P3-4 start TOCTOU / 脏 PID

先 `net.Listen` 成功再 `WritePID`，再 `Serve`。Listen 失败（含端口占用）返回错误，不要 panic。health 探活快路径保留。

## P3-5 Jina 内网判断

`pkg/jina/reader.go` `isPrivateIPString`：改 `net.ParseIP` + `IsPrivate()`/`IsLoopback()`。`172.32.0.1` 不得判私有；`172.16.0.1` 必须判私有。

## P3-6 缓存 upsert

`search_cache` 对 `(query, intent, academic)` 唯一；Store 用 `INSERT ON CONFLICT DO UPDATE`。旧库先删重复行再加 UNIQUE INDEX。

## 验收

- `go test ./pkg/daemon/... ./pkg/cache/... ./pkg/jina/...`
- Jina：`172.32.0.1` false，`192.168.1.1` true
- 同一 query Store 两次，表中 1 行
