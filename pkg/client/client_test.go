package client

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

// TestDefaultClientTimeout 默认客户端应带 30s 超时。
func TestDefaultClientTimeout(t *testing.T) {
	if got := DefaultClient.Timeout(); got != DefaultTimeout {
		t.Errorf("expected DefaultClient timeout %v, got %v", DefaultTimeout, got)
	}
}

// TestTimeout_BlockingServer 阻塞不响应的上游应在注入的 timeout 内返回错误。
// 用不 Accept 的原始 listener 模拟黑洞上游：连接进入 backlog 后永不响应，
// 避免 httptest 的 Close 等待挂起的 handler（POST+body 超时后连接不会关闭）。
func TestTimeout_BlockingServer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	c := New(200 * time.Millisecond)
	start := time.Now()
	_, err = c.R().Get("http://" + ln.Addr().String() + "/")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(strings.ToLower(err.Error()), "timeout") {
		t.Errorf("expected timeout error, got: %v", err)
	}
	if elapsed >= time.Second {
		t.Errorf("timeout 应 < 1s, got %v", elapsed)
	}
}
