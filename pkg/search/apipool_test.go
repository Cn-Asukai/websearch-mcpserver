package search

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
)

// ── poolAwareEngine mock：调用 pool.Next() 模拟真实引擎行为 ──────────────

type poolAwareEngine struct {
	name   string
	pool   *KeyPool
	result []SearchResult
	errFn  func(key string) error // 根据 key 决定是否报错
}

func (p *poolAwareEngine) Name() string { return p.name }
func (p *poolAwareEngine) Search(query string) (string, error) {
	return "", nil
}
func (p *poolAwareEngine) SearchRaw(query string) ([]SearchResult, error) {
	key := p.pool.Next()
	if p.errFn != nil {
		if err := p.errFn(key); err != nil {
			return nil, err
		}
	}
	return p.result, nil
}
func (p *poolAwareEngine) MergeContent(query string, results []SearchResult) (string, error) {
	return "", nil
}

// ── ApipoolSearchImpl ──────────────────────────────────────────────────────

func TestApipool_FirstProviderSucceeds(t *testing.T) {
	pool1, _ := NewKeyPool([]string{"k1"})
	pool2, _ := NewKeyPool([]string{"k2"})
	e1 := &poolAwareEngine{name: "baidu", pool: pool1, result: []SearchResult{{Title: "b1", Url: "http://b1.com", Engine: "baidu"}}}
	e2 := &poolAwareEngine{name: "tavily", pool: pool2, result: []SearchResult{{Title: "t1", Url: "http://t1.com", Engine: "tavily"}}}
	ap := NewApipoolSearch("round-robin",
		apipoolProvider{engine: e1, pool: pool1},
		apipoolProvider{engine: e2, pool: pool2},
	)
	results, err := ap.SearchRaw("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 || results[0].Title != "b1" {
		t.Errorf("expected b1, got %v", results)
	}
}

// ── 同供应商 SK 重试 ──────────────────────────────────────────────────────

func TestApipool_IntraPool_RetryNextSK(t *testing.T) {
	pool1, _ := NewKeyPool([]string{"k1_bad", "k2_good"})
	// k1_bad 报错，k2_good 成功
	e1 := &poolAwareEngine{
		name:   "baidu",
		pool:   pool1,
		result: []SearchResult{{Title: "b1", Url: "http://b1.com", Engine: "baidu"}},
		errFn: func(key string) error {
			if key == "k1_bad" {
				return fmt.Errorf("auth fail")
			}
			return nil
		},
	}
	ap := NewApipoolSearch("round-robin",
		apipoolProvider{engine: e1, pool: pool1},
	)
	results, err := ap.SearchRaw("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].Title != "b1" {
		t.Errorf("expected b1, got %s", results[0].Title)
	}
	// k1_bad 被标记失效，k2_good 仍可用
	if pool1.Available() != 1 {
		t.Errorf("expected 1 available key, got %d", pool1.Available())
	}
}

func TestApipool_IntraPool_AllSKsFail_FallbackToNext(t *testing.T) {
	pool1, _ := NewKeyPool([]string{"k1", "k2"})
	pool2, _ := NewKeyPool([]string{"k3"})
	// baidu 全部 key 都失败
	e1 := &poolAwareEngine{
		name: "baidu", pool: pool1,
		errFn: func(key string) error { return fmt.Errorf("fail") },
	}
	e2 := &poolAwareEngine{
		name: "tavily", pool: pool2,
		result: []SearchResult{{Title: "t1", Url: "http://t1.com", Engine: "tavily"}},
	}
	ap := NewApipoolSearch("round-robin",
		apipoolProvider{engine: e1, pool: pool1},
		apipoolProvider{engine: e2, pool: pool2},
	)
	results, err := ap.SearchRaw("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].Title != "t1" {
		t.Errorf("expected t1, got %s", results[0].Title)
	}
	if pool1.Available() != 0 {
		t.Errorf("expected pool1 exhausted, got %d available", pool1.Available())
	}
}

// ── priority 策略 ─────────────────────────────────────────────────────────

func TestApipool_Priority_AlwaysStartFromFirst(t *testing.T) {
	pool1, _ := NewKeyPool([]string{"k1"})
	pool2, _ := NewKeyPool([]string{"k2"})
	e1 := &poolAwareEngine{name: "baidu", pool: pool1, result: []SearchResult{{Title: "b1", Engine: "baidu"}}}
	e2 := &poolAwareEngine{name: "tavily", pool: pool2, result: []SearchResult{{Title: "t1", Engine: "tavily"}}}
	ap := NewApipoolSearch("priority",
		apipoolProvider{engine: e1, pool: pool1},
		apipoolProvider{engine: e2, pool: pool2},
	)
	for i := 0; i < 5; i++ {
		results, err := ap.SearchRaw("test")
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		if results[0].Engine != "baidu" {
			t.Errorf("call %d: expected baidu, got %s", i, results[0].Engine)
		}
	}
}

func TestApipool_Priority_FallbackWhenFirstExhausted(t *testing.T) {
	pool1, _ := NewKeyPool([]string{"k1"})
	pool2, _ := NewKeyPool([]string{"k2"})
	e1 := &poolAwareEngine{name: "baidu", pool: pool1, errFn: func(string) error { return fmt.Errorf("fail") }}
	e2 := &poolAwareEngine{name: "tavily", pool: pool2, result: []SearchResult{{Title: "t1", Engine: "tavily"}}}
	ap := NewApipoolSearch("priority",
		apipoolProvider{engine: e1, pool: pool1},
		apipoolProvider{engine: e2, pool: pool2},
	)
	results, err := ap.SearchRaw("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].Title != "t1" {
		t.Errorf("expected t1, got %s", results[0].Title)
	}
}

// ── round-robin 策略 ──────────────────────────────────────────────────────

func TestApipool_RoundRobin_RotatesProvider(t *testing.T) {
	pool1, _ := NewKeyPool([]string{"k1"})
	pool2, _ := NewKeyPool([]string{"k2"})
	e1 := &poolAwareEngine{name: "baidu", pool: pool1, result: []SearchResult{{Title: "b1", Engine: "baidu"}}}
	e2 := &poolAwareEngine{name: "tavily", pool: pool2, result: []SearchResult{{Title: "t1", Engine: "tavily"}}}
	ap := NewApipoolSearch("round-robin",
		apipoolProvider{engine: e1, pool: pool1},
		apipoolProvider{engine: e2, pool: pool2},
	)
	r1, _ := ap.SearchRaw("test")
	r2, _ := ap.SearchRaw("test")
	r3, _ := ap.SearchRaw("test")
	if r1[0].Engine != "baidu" {
		t.Errorf("call 1: expected baidu, got %s", r1[0].Engine)
	}
	if r2[0].Engine != "tavily" {
		t.Errorf("call 2: expected tavily, got %s", r2[0].Engine)
	}
	if r3[0].Engine != "baidu" {
		t.Errorf("call 3: expected baidu, got %s", r3[0].Engine)
	}
}

func TestApipool_RoundRobin_FallbackOnFail(t *testing.T) {
	pool1, _ := NewKeyPool([]string{"k1"})
	pool2, _ := NewKeyPool([]string{"k2"})
	pool3, _ := NewKeyPool([]string{"k3"})
	e1 := &poolAwareEngine{name: "baidu", pool: pool1, errFn: func(string) error { return fmt.Errorf("fail") }}
	e2 := &poolAwareEngine{name: "tavily", pool: pool2, result: []SearchResult{{Title: "t1", Engine: "tavily"}}}
	e3 := &poolAwareEngine{name: "exa", pool: pool3, result: []SearchResult{{Title: "e1", Engine: "exa"}}}
	ap := NewApipoolSearch("round-robin",
		apipoolProvider{engine: e1, pool: pool1},
		apipoolProvider{engine: e2, pool: pool2},
		apipoolProvider{engine: e3, pool: pool3},
	)
	// start=0, baidu 失败 → fallback tavily
	r1, _ := ap.SearchRaw("test")
	if r1[0].Engine != "tavily" {
		t.Errorf("call 1: expected tavily, got %s", r1[0].Engine)
	}
	// start=1, tavily 直接成功
	r2, _ := ap.SearchRaw("test")
	if r2[0].Engine != "tavily" {
		t.Errorf("call 2: expected tavily, got %s", r2[0].Engine)
	}
}

// ── 边界情况 ──────────────────────────────────────────────────────────────

func TestApipool_AllFail(t *testing.T) {
	pool1, _ := NewKeyPool([]string{"k1"})
	pool2, _ := NewKeyPool([]string{"k2"})
	e1 := &poolAwareEngine{name: "baidu", pool: pool1, errFn: func(string) error { return fmt.Errorf("fail1") }}
	e2 := &poolAwareEngine{name: "tavily", pool: pool2, errFn: func(string) error { return fmt.Errorf("fail2") }}
	ap := NewApipoolSearch("round-robin",
		apipoolProvider{engine: e1, pool: pool1},
		apipoolProvider{engine: e2, pool: pool2},
	)
	_, err := ap.SearchRaw("test")
	if err == nil {
		t.Fatal("expected error when all providers fail")
	}
}

func TestApipool_NilPool_NoPanic(t *testing.T) {
	e1 := &mockEngine{name: "baidu", err: fmt.Errorf("fail")}
	e2 := &mockEngine{name: "baiduWeb", results: []SearchResult{{Title: "w1", Url: "http://w1.com", Engine: "baiduWeb"}}}
	ap := NewApipoolSearch("round-robin",
		apipoolProvider{engine: e1, pool: nil},
		apipoolProvider{engine: e2, pool: nil},
	)
	results, err := ap.SearchRaw("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].Title != "w1" {
		t.Errorf("expected w1, got %s", results[0].Title)
	}
}

func TestApipool_MaxSize(t *testing.T) {
	pool1, _ := NewKeyPool([]string{"k1"})
	e1 := &poolAwareEngine{name: "baidu", pool: pool1, result: []SearchResult{
		{Title: "b1", Engine: "baidu"}, {Title: "b2", Engine: "baidu"}, {Title: "b3", Engine: "baidu"},
	}}
	ap := NewApipoolSearch("round-robin", apipoolProvider{engine: e1, pool: pool1})
	ap.SetMaxSize(2)
	results, err := ap.SearchRaw("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2, got %d", len(results))
	}
}

func TestApipool_Name(t *testing.T) {
	ap := NewApipoolSearch("round-robin")
	if ap.Name() != "apipool" {
		t.Errorf("expected apipool, got %s", ap.Name())
	}
}

func TestApipool_DefaultStrategy(t *testing.T) {
	pool1, _ := NewKeyPool([]string{"k1"})
	e := &poolAwareEngine{name: "a", pool: pool1, result: []SearchResult{{Title: "a1", Engine: "a"}}}
	ap := NewApipoolSearch("", apipoolProvider{engine: e, pool: pool1})
	if ap.strategy != "round-robin" {
		t.Errorf("expected round-robin, got %s", ap.strategy)
	}
}

func TestApipool_SearchRawWithTimeRange(t *testing.T) {
	pool1, _ := NewKeyPool([]string{"k1"})
	trEngine := &timeRangeRecorder{name: "baidu", results: []SearchResult{{Title: "b1", Engine: "baidu"}}}
	ap := NewApipoolSearch("round-robin", apipoolProvider{engine: trEngine, pool: pool1})
	results, err := ap.SearchRawWithTimeRange("test", 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1, got %d", len(results))
	}
	if trEngine.lastLookback != 7 {
		t.Errorf("expected lookbackDays=7, got %d", trEngine.lastLookback)
	}
}

// ── KeyError 精确标记 ──────────────────────────────────────────────────────

// keyErrorEngine 模拟真实引擎：Next() 取 key，失败时返回携带该 key 的 KeyError。
type keyErrorEngine struct {
	pool     *KeyPool
	failKeys map[string]bool
	gotKey   chan struct{} // 每次 Next 后发信号（非 nil 时）
	release  chan struct{} // 放行失败（非 nil 时）
}

func (e *keyErrorEngine) Name() string { return "keyerr" }
func (e *keyErrorEngine) Search(query string) (string, error) {
	return "", nil
}
func (e *keyErrorEngine) SearchRaw(query string) ([]SearchResult, error) {
	key := e.pool.Next()
	if e.gotKey != nil {
		e.gotKey <- struct{}{}
	}
	if e.release != nil {
		<-e.release
	}
	if e.failKeys[key] {
		return nil, &KeyError{Key: key, Err: fmt.Errorf("boom")}
	}
	return []SearchResult{{Title: "ok", Url: "http://ok.com", Engine: "keyerr"}}, nil
}
func (e *keyErrorEngine) MergeContent(query string, results []SearchResult) (string, error) {
	return "", nil
}

// TestKeyError_ErrorHidesKey KeyError.Error() 不得泄漏 Key 本身（日志安全）。
func TestKeyError_ErrorHidesKey(t *testing.T) {
	ke := &KeyError{Key: "sk-secret-123", Err: fmt.Errorf("auth fail")}
	if strings.Contains(ke.Error(), "sk-secret-123") {
		t.Errorf("KeyError.Error() 不应包含 Key: %s", ke.Error())
	}
	var got *KeyError
	if !errors.As(fmt.Errorf("wrap: %w", ke), &got) || got.Key != "sk-secret-123" {
		t.Error("errors.As 应取出 KeyError 及 Key")
	}
}

// TestApipool_KeyError_MarksUsedKey 单 goroutine：KeyError 的 key 被精确冷却。
func TestApipool_KeyError_MarksUsedKey(t *testing.T) {
	pool, _ := NewKeyPool([]string{"k1", "k2"})
	e := &keyErrorEngine{pool: pool, failKeys: map[string]bool{"k1": true}}
	ap := NewApipoolSearch("round-robin", apipoolProvider{engine: e, pool: pool})
	results, err := ap.SearchRaw("test")
	if err != nil {
		t.Fatalf("k2 应重试成功, got err=%v", err)
	}
	if len(results) != 1 || results[0].Title != "ok" {
		t.Fatalf("expected ok result, got %v", results)
	}
	// k1 被冷却，k2 仍可用
	if pool.Available() != 1 {
		t.Fatalf("expected 1 available (k2), got %d", pool.Available())
	}
	if got := pool.Next(); got != "k2" {
		t.Errorf("expected next key k2, got %s", got)
	}
}

// TestApipool_ConcurrentKeyError_CoolsOwnKey 并发：两个 goroutine 交错 Next + 失败，
// 各自冷却自己的 key，不误伤对方（旧 MarkLastInvalid 会标错 key）。
func TestApipool_ConcurrentKeyError_CoolsOwnKey(t *testing.T) {
	pool, _ := NewKeyPool([]string{"k1", "k2"})
	e := &keyErrorEngine{
		pool:     pool,
		failKeys: map[string]bool{"k1": true}, // k1 坏，k2 好
		gotKey:   make(chan struct{}, 4),
		release:  make(chan struct{}),
	}
	ap := NewApipoolSearch("round-robin", apipoolProvider{engine: e, pool: pool})

	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			_, _ = ap.SearchRaw("test")
		}()
	}
	// 等两个 goroutine 都完成首次 Next（交错完成），再放行失败
	<-e.gotKey
	<-e.gotKey
	close(e.release)
	wg.Wait()

	// 只有 k1（坏 key）被冷却，k2（好 key）必须保持可用
	if pool.Available() != 1 {
		t.Fatalf("expected only k1 cooled (1 available), got %d", pool.Available())
	}
	if got := pool.Next(); got != "k2" {
		t.Errorf("expected k2 still available, got %s", got)
	}
}

// ── 辅助类型 ──────────────────────────────────────────────────────────────

// timeRangeRecorder 记录 SearchRawWithTimeRange 调用参数。
type timeRangeRecorder struct {
	name         string
	results      []SearchResult
	err          error
	lastLookback int
}

func (t *timeRangeRecorder) Name() string                                   { return t.name }
func (t *timeRangeRecorder) Search(query string) (string, error)            { return "", nil }
func (t *timeRangeRecorder) SearchRaw(query string) ([]SearchResult, error) { return t.results, t.err }
func (t *timeRangeRecorder) MergeContent(_ string, _ []SearchResult) (string, error) {
	return "", nil
}
func (t *timeRangeRecorder) SearchRawWithTimeRange(query string, lookbackDays int) ([]SearchResult, error) {
	t.lastLookback = lookbackDays
	return t.results, t.err
}
