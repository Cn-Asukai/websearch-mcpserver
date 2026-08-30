package antirobot

import (
	"net/http"
	"testing"
	"time"
)

func TestRateLimiterMinInterval(t *testing.T) {
	l := NewRateLimiter(100, 10000).WithMinInterval(50 * time.Millisecond)
	if got := l.MinInterval(); got != 50*time.Millisecond {
		t.Errorf("MinInterval = %v, want 50ms", got)
	}
	if !l.Allow() {
		t.Fatal("首次请求应放行")
	}
	if l.Allow() {
		t.Fatal("最小间隔未满足应拒绝")
	}
	time.Sleep(60 * time.Millisecond)
	if !l.Allow() {
		t.Fatal("最小间隔满足后应放行")
	}
}

func TestRateLimiterNoMinInterval(t *testing.T) {
	l := NewRateLimiter(100, 10000)
	if l.MinInterval() != 0 {
		t.Errorf("默认最小间隔应为 0, got %v", l.MinInterval())
	}
	if !l.Allow() || !l.Allow() {
		t.Fatal("无最小间隔约束应连续放行")
	}
}

func TestParseRetryAfter(t *testing.T) {
	if d := ParseRetryAfter(""); d != 0 {
		t.Errorf("空串应返回 0, got %v", d)
	}
	if d := ParseRetryAfter("3"); d != 3*time.Second {
		t.Errorf("秒数解析失败: %v", d)
	}
	if d := ParseRetryAfter("garbage"); d != 0 {
		t.Errorf("垃圾值应返回 0, got %v", d)
	}
	future := time.Now().Add(5 * time.Second).UTC().Format(http.TimeFormat)
	if d := ParseRetryAfter(future); d <= 0 || d > 6*time.Second {
		t.Errorf("HTTP-Date 解析异常: %v", d)
	}
}
