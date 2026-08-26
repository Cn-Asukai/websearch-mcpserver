# Wave 4 — P3 杂项

> 对应任务：[T14](../tasks/T14-p3-hygiene.md)
> 六项都小，一个 PR 即可；也可并进 Wave 3 最后一个 PR。

---

## P3-1 PostShutdown 未 Close body

`pkg/daemon/daemon.go:138-141`：`http.Post` 返回的 `resp` 丢弃。不 Close 会泄漏连接。

```go
resp, err := http.Post(url, "application/json", nil)
if err != nil {
    return err
}
resp.Body.Close()
return nil
```

同文件 `PostRefCount` / `GetHealth` 已 defer Close，照抄。

---

## P3-2 CleanupScheduler.Stop 注释撒谎

`pkg/cache/cleanup.go:50-53`：注释写「阻塞等待协程退出」，实现只 `close(s.stop)`。

二选一（推荐 B，关机更干净）：

- A：改注释为「发停止信号，不保证立刻退出」。
- B：`stop` 后再用 `sync.WaitGroup` 或 `<-done` 等待；`Stop(ctx)` 尊重 `ctx.Done()`。

选 B 时 `server.go` 关机已 `Stop(context.Background())`，协程若卡在 `EvictStale` 可能多等一轮 SQL。可接受。

---

## P3-3 关机先关 DB 再 Shutdown

`server/server.go:188-204`：先 `cache.Close()`，再 `srv.Shutdown`。5s 收尾窗口里还在飞的 `/mcp` 会 `sql: database is closed`。

顺序改为：

1. `cleanup.Stop`
2. `srv.Shutdown(ctx)`  （不再接新连接，等进行中请求结束）
3. `webfetch.Close`
4. `cache.Close`
5. `RemovePID`

进行中的搜索可能仍占用上游，Shutdown 超时 5s 保持不变。

---

## P3-4 runStart TOCTOU + 脏 PID

`cmd/main.go:19-43`：health 失败就认为没人听，写 PID 再 `Run`。两个并发 `start` 都能过探活；第二个 `ListenAndServe` panic。PID 写在监听成功前，端口占用失败会留脏文件。

**方案（够用即可，不做分布式锁）：**

1. 监听成功后再 `WritePID`。需要 `Run` 返回「已在听」信号：例如 `http.Server` 配 `BaseContext` / 在 goroutine 里 Listen 前用 `net.Listen`，`ln` 成功再写 PID、再 `Serve`。
2. `Listen` 失败：不写 PID，error 返回而不是 panic（至少端口占用不要 panic）。
3. TOCTOU 无法 100% 消除；`net.Listen` 失败（`EADDRINUSE`）时打印「已有进程在该端口」，exit 1。这比 panic 好。

最小实现：`server.Run` 改 `net.Listen("tcp", addr)` → 成功回调/返回后 main 写 PID → `Serve`。health 探活保留作 refcount 快路径。

---

## P3-5 Jina `172.3` 前缀误伤公网

`pkg/jina/reader.go:176-184`：`HasPrefix("172.2")` / `"172.3"` 会把 `172.32.0.0/11` 等公网段当内网（fail-closed，抓取被拒，不是 SSRF 洞）。`172.2.x.x` 也会误伤。

正确：`net.ParseIP` + `ip.IsPrivate()`（Go 1.17+ 覆盖 RFC1918 / 链路本地），或 `ip.IsLoopback()`。hostname 先 `net.ParseIP`，解析失败再 DNS（已有 rebinding 逻辑，复用）。

不要用字符串前缀判断 IP 段。

---

## P3-6 cache Store 只 INSERT

`pkg/cache/sqlite.go:160-175`：无 UNIQUE，同 query+intent+academic 在 6h 窗口重复插入。Lookup 靠索引取一条，表会胀。

```sql
CREATE UNIQUE INDEX IF NOT EXISTS idx_query_intent_academic_uniq
  ON search_cache(query, intent, academic);
```

Store 改为 `INSERT ... ON CONFLICT DO UPDATE` 刷新 `raw_results/summary/last_hit_at`。旧库用 `CREATE UNIQUE INDEX` 可能因历史重复行失败：先 `DELETE` 保留 `MAX(last_hit_at)` 再加唯一索引（迁移写在 `New()` 里，一次即可）。
