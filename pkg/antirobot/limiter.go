package antirobot

import (
	"sync"
	"time"
)

// RateLimiter 滑动窗口限流器。
type RateLimiter struct {
	mu          sync.Mutex
	perSec      int
	perMin      int
	minInterval time.Duration // 相邻两次请求的最小间隔（0 = 不约束）
	last        time.Time
	secWindow   []time.Time
	minWindow   []time.Time
}

// NewRateLimiter 创建限流器。perSec=每秒上限，perMin=每分钟上限。
func NewRateLimiter(perSec, perMin int) *RateLimiter {
	return &RateLimiter{perSec: perSec, perMin: perMin}
}

// WithMinInterval 设置相邻两次请求的最小间隔（如 arXiv 官方 Tou 的 1 req/3s），
// 返回自身以支持链式调用。间隔未满足时 Allow 返回 false 且不记录请求。
func (r *RateLimiter) WithMinInterval(d time.Duration) *RateLimiter {
	r.minInterval = d
	return r
}

// Limits 返回当前限流配置（每秒/每分钟上限）。
func (r *RateLimiter) Limits() (perSec, perMin int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.perSec, r.perMin
}

// MinInterval 返回相邻请求的最小间隔。
func (r *RateLimiter) MinInterval() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.minInterval
}

// Allow 检查并记录一次请求，允许则返回 true。
func (r *RateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	if r.minInterval > 0 && !r.last.IsZero() && now.Sub(r.last) < r.minInterval {
		return false
	}
	r.secWindow = filterAfter(r.secWindow, now.Add(-time.Second))
	r.minWindow = filterAfter(r.minWindow, now.Add(-time.Minute))

	if len(r.secWindow) >= r.perSec || len(r.minWindow) >= r.perMin {
		return false
	}
	r.last = now
	r.secWindow = append(r.secWindow, now)
	r.minWindow = append(r.minWindow, now)
	return true
}

func filterAfter(w []time.Time, cut time.Time) []time.Time {
	i := 0
	for _, t := range w {
		if t.After(cut) {
			w[i] = t
			i++
		}
	}
	return w[:i]
}
