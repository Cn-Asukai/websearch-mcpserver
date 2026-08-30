package academic

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"websearch/pkg/antirobot"
)

const okArxivXML = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <id>http://arxiv.org/abs/2401.00001v1</id>
    <title>Test Paper</title>
    <summary>An abstract.</summary>
    <published>2024-01-01T00:00:00Z</published>
    <author><name>A. Author</name></author>
  </entry>
</feed>`

// withShortArxivCooldown 缩短冷却时长，避免单测等待真实窗口。
func withShortArxivCooldown(t *testing.T, base time.Duration) {
	t.Helper()
	oldBase, oldMax := arxivRateCooldown, arxivCooldownMax
	arxivRateCooldown, arxivCooldownMax = base, 10*base
	t.Cleanup(func() { arxivRateCooldown, arxivCooldownMax = oldBase, oldMax })
}

func withArxivTestServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	oldEndpoint := arxivEndpoint
	arxivEndpoint = ts.URL
	t.Cleanup(func() { arxivEndpoint = oldEndpoint })
}

// arxivTestEngine 构造本地限流不干扰的测试引擎。
func arxivTestEngine() *arxivEngine {
	e := NewArxiv(antirobot.ArxivOpts{Enabled: true}, http.DefaultClient).(*arxivEngine)
	e.limiter = antirobot.NewRateLimiter(100, 10000)
	return e
}

func withArxivBudget(t *testing.T, d time.Duration) {
	t.Helper()
	old := arxivSearchBudget
	arxivSearchBudget = d
	t.Cleanup(func() { arxivSearchBudget = old })
}

// TestArxivRateLimitClamp 限流配置钳制：宽松配置钳到内置上限（官方 Tou 1 req/3s），
// 更严格配置保留，未配置用内置默认；最小间隔固定 3s。
func TestArxivRateLimitClamp(t *testing.T) {
	cases := []struct {
		name         string
		perSec, mPer int
		wantSec      int
		wantMin      int
	}{
		{"宽松配置被钳制", 10, 100, arxivMaxPerSec, arxivMaxPerMin},
		{"全局 rate_limit 默认 3/60 被钳制", 3, 60, arxivMaxPerSec, arxivMaxPerMin},
		{"未配置用内置默认", 0, 0, arxivMaxPerSec, arxivMaxPerMin},
		{"更严格配置保留", 1, 3, 1, 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := NewArxiv(antirobot.ArxivOpts{Enabled: true, PerSec: c.perSec, PerMin: c.mPer}, http.DefaultClient).(*arxivEngine)
			gotSec, gotMin := e.limiter.Limits()
			if gotSec != c.wantSec {
				t.Errorf("perSec = %d, want %d", gotSec, c.wantSec)
			}
			if gotMin != c.wantMin {
				t.Errorf("perMin = %d, want %d", gotMin, c.wantMin)
			}
			if got := e.limiter.MinInterval(); got != arxivMinInterval {
				t.Errorf("minInterval = %v, want %v", got, arxivMinInterval)
			}
		})
	}
}

// TestArxivRetryWithinBudget 短 Retry-After 在超时预算内：等待窗口后重试一次，
// 本次调用直接成功返回（不进冷却）。
func TestArxivRetryWithinBudget(t *testing.T) {
	withShortArxivCooldown(t, 30*time.Second)
	var hits int32
	withArxivTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&hits, 1) == 1 {
			w.Header().Set("Retry-After", "1") // 窗口 1s，预算 10s 内
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		fmt.Fprint(w, okArxivXML)
	})

	e := arxivTestEngine()
	resp, err := e.Search("test query", 1, antirobot.TimeRangeNone)
	if err != nil {
		t.Fatalf("预算内应等待重试成功: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Errorf("应解析出 1 条结果, got %d", len(resp.Results))
	}
	if n := atomic.LoadInt32(&hits); n != 2 {
		t.Errorf("应恰好 2 次请求（429 + 重试）, got %d", n)
	}
	if e.cooldownRemaining() != 0 || e.cooldown != 0 {
		t.Errorf("重试成功不应留下冷却状态, remaining=%v cooldown=%v", e.cooldownRemaining(), e.cooldown)
	}
}

// TestArxivCooldownWhenWaitExceedsBudget 等待窗口超出剩余预算（注定超时）：
// 直接放弃进冷却快速失败，不做无意义的等待重试；窗口过后自动恢复。
func TestArxivCooldownWhenWaitExceedsBudget(t *testing.T) {
	withShortArxivCooldown(t, 1*time.Second)
	withArxivBudget(t, 2*time.Second) // 预算 2s：窗口(1s)+重试预估(2s) 注定超时
	var hits int32
	withArxivTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&hits, 1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		fmt.Fprint(w, okArxivXML)
	})

	e := arxivTestEngine()

	// 第一次：放弃并进入冷却
	if _, err := e.Search("q", 1, antirobot.TimeRangeNone); err == nil || !strings.Contains(err.Error(), "cooling down") {
		t.Fatalf("预算不足应报冷却错误, got %v", err)
	}
	if e.cooldownRemaining() <= 0 {
		t.Fatal("应进入冷却期")
	}
	if n := atomic.LoadInt32(&hits); n != 1 {
		t.Errorf("放弃路径不应有第二次请求, hits = %d", n)
	}

	// 冷却期内：快速失败不打上游
	if _, err := e.Search("q2", 1, antirobot.TimeRangeNone); err == nil || !strings.Contains(err.Error(), "cooling down") {
		t.Fatalf("冷却期内应快速失败, got %v", err)
	}
	if n := atomic.LoadInt32(&hits); n != 1 {
		t.Fatalf("冷却期内不应请求上游, hits = %d", n)
	}

	// 窗口过后：自动恢复并成功，冷却清零
	time.Sleep(1100 * time.Millisecond)
	resp, err := e.Search("q3", 1, antirobot.TimeRangeNone)
	if err != nil {
		t.Fatalf("冷却结束后应恢复: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Errorf("应解析出 1 条结果, got %d", len(resp.Results))
	}
	if e.cooldownRemaining() != 0 || e.cooldown != 0 {
		t.Errorf("成功后冷却应清零, remaining=%v cooldown=%v", e.cooldownRemaining(), e.cooldown)
	}
}

// TestArxivCooldownDoubling 连续 429（无 Retry-After）冷却翻倍。
func TestArxivCooldownDoubling(t *testing.T) {
	withShortArxivCooldown(t, 20*time.Millisecond)
	withArxivTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})
	e := arxivTestEngine()

	if _, err := e.Search("q", 1, antirobot.TimeRangeNone); err == nil {
		t.Fatal("expect error")
	}
	if first := e.cooldown; first != 20*time.Millisecond {
		t.Fatalf("首次冷却应取默认值, got %v", first)
	}

	time.Sleep(30 * time.Millisecond) // 等首次冷却过期
	if _, err := e.Search("q", 1, antirobot.TimeRangeNone); err == nil {
		t.Fatal("expect error")
	}
	if e.cooldown != 2*20*time.Millisecond {
		t.Fatalf("连续 429 冷却应翻倍, got %v", e.cooldown)
	}
}

// TestArxivThrottleBudgetExceeded 本地限流等待注定超出预算：直接报错放弃，
// 不打上游（与 DDG『注定要超时就放弃』策略一致）。
func TestArxivThrottleBudgetExceeded(t *testing.T) {
	withArxivBudget(t, 100*time.Millisecond)
	var hits int32
	withArxivTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		fmt.Fprint(w, okArxivXML)
	})

	e := NewArxiv(antirobot.ArxivOpts{Enabled: true}, http.DefaultClient).(*arxivEngine)

	// 第一次正常成功（最小间隔对首次请求不生效）
	if _, err := e.Search("q1", 1, antirobot.TimeRangeNone); err != nil {
		t.Fatalf("首次请求应成功: %v", err)
	}
	// 第二次：3s 最小间隔未满足，而预算仅 100ms → 注定超时直接放弃
	if _, err := e.Search("q2", 1, antirobot.TimeRangeNone); err == nil || !strings.Contains(err.Error(), "would exceed") {
		t.Fatalf("预算不足应放弃等待, got %v", err)
	}
	if n := atomic.LoadInt32(&hits); n != 1 {
		t.Errorf("放弃路径不应请求上游, hits = %d", n)
	}
}
