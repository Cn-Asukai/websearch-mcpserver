package cache

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// newTestScheduler 创建已 Start 的调度器，interval 取 1 小时保证测试期间 ticker 不触发。
func newTestScheduler(t *testing.T) *CleanupScheduler {
	t.Helper()
	c, err := New(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatalf("New 失败: %v", err)
	}
	t.Cleanup(func() { c.Close() })

	s := NewCleanupScheduler(c, time.Hour)
	s.Start()
	return s
}

// TestStopWaitsForGoroutineExit Stop 使用不可取消的 context 时，返回即证明协程已退出
// （done channel 由协程退出时关闭，Background 无法提前返回）。
func TestStopWaitsForGoroutineExit(t *testing.T) {
	s := newTestScheduler(t)

	done := make(chan struct{})
	go func() {
		s.Stop(context.Background())
		close(done)
	}()

	select {
	case <-done:
		// Stop 正常返回
	case <-time.After(5 * time.Second):
		t.Fatal("Stop 应阻塞等待协程退出，5s 内未返回")
	}
}

// TestStopRespectsCancelledCtx Stop 传入已取消的 context 时应立即返回，不阻塞。
func TestStopRespectsCancelledCtx(t *testing.T) {
	s := newTestScheduler(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		s.Stop(ctx)
		close(done)
	}()

	select {
	case <-done:
		// 已取消的 ctx 下 Stop 立即返回
	case <-time.After(5 * time.Second):
		t.Fatal("已取消的 ctx 下 Stop 不应阻塞")
	}
}

// TestStopBeforeStart 未 Start 时 Stop 应仅发停止信号并立即返回，不阻塞。
func TestStopBeforeStart(t *testing.T) {
	c, err := New(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatalf("New 失败: %v", err)
	}
	defer c.Close()

	s := NewCleanupScheduler(c, time.Hour)

	done := make(chan struct{})
	go func() {
		s.Stop(context.Background())
		close(done)
	}()

	select {
	case <-done:
		// 未 Start 时 Stop 立即返回
	case <-time.After(5 * time.Second):
		t.Fatal("未 Start 时 Stop 不应阻塞")
	}

	// 之后再 Start，协程应立刻感知停止信号并退出
	s.Start()
	select {
	case <-s.done:
		// 协程已退出
	case <-time.After(5 * time.Second):
		t.Fatal("Start 后协程应感知已发出的停止信号并退出")
	}
}

// TestStopIdempotent 重复调用 Stop 不应 panic。
func TestStopIdempotent(t *testing.T) {
	s := newTestScheduler(t)

	s.Stop(context.Background())
	s.Stop(context.Background())
}
