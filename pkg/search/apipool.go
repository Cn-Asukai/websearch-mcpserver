package search

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"

	"websearch/pkg/log"
	md "websearch/pkg/xml"
)

// KeyError 携带请求失败时实际使用的 API Key，供 apipool 精确标记失效。
// 注意：Error() 不输出 Key 本身，避免 API Key 泄漏到日志。
type KeyError struct {
	Key string
	Err error
}

func (e *KeyError) Error() string {
	if e.Err == nil {
		return "key error"
	}
	return e.Err.Error()
}

func (e *KeyError) Unwrap() error { return e.Err }

// apipoolProvider 单个供应商：搜索引擎 + 对应的 KeyPool（免费引擎 pool 为 nil）。
type apipoolProvider struct {
	engine SearchInf
	pool   *KeyPool
}

// ApipoolSearchImpl API Key 池轮转搜索引擎。
//
// 支持两种策略：
//   - round-robin（默认）：跨请求轮转起始供应商，同一次请求内先用完当前供应商所有可用 SK 再 fallback
//   - priority：始终从第一个供应商开始，用完所有 SK → 下一个供应商 → web 兜底
//
// 两种策略都支持同供应商内 SK 重试：当前 key 失败 → 标记 invalid → 尝试同 pool 下一个可用 key。
type ApipoolSearchImpl struct {
	providers []apipoolProvider
	strategy  string // "round-robin" / "priority"
	idx       atomic.Uint64
	maxSize   int // 全局最大结果数，0 = 不限
}

// NewApipoolSearch 创建 ApipoolSearchImpl，providers 顺序即为优先级顺序。
func NewApipoolSearch(strategy string, providers ...apipoolProvider) *ApipoolSearchImpl {
	if strategy != "priority" {
		strategy = "round-robin"
	}
	return &ApipoolSearchImpl{providers: providers, strategy: strategy}
}

// SetMaxSize 设置全局最大结果数。
func (a *ApipoolSearchImpl) SetMaxSize(n int) {
	a.maxSize = n
}

func (a *ApipoolSearchImpl) Name() string { return "apipool" }

func (a *ApipoolSearchImpl) Search(query string) (string, error) {
	results, err := a.SearchRaw(query)
	if err != nil {
		return "", err
	}
	return a.MergeContent(query, results)
}

func (a *ApipoolSearchImpl) SearchRawWithTimeRange(query string, lookbackDays int) ([]SearchResult, error) {
	n := len(a.providers)
	if n == 0 {
		return nil, fmt.Errorf("apipool: 无可用供应商")
	}

	// 确定起始供应商索引
	var start int
	if a.strategy == "priority" {
		start = 0
	} else {
		start = int(a.idx.Add(1)-1) % n
	}

	var lastErr error
	for i := 0; i < n; i++ {
		p := a.providers[(start+i)%n]
		results, err := a.callProviderWithRetry(p, query, lookbackDays)
		if err == nil {
			return results, nil
		}
		log.Infof("apipool: %s 所有 SK 均不可用，切换下一个供应商: %v", p.engine.Name(), err)
		lastErr = err
	}
	return nil, fmt.Errorf("apipool: 所有供应商均失败，最后错误: %w", lastErr)
}

func (a *ApipoolSearchImpl) SearchRaw(query string) ([]SearchResult, error) {
	return a.SearchRawWithTimeRange(query, 0)
}

// callProviderWithRetry 调用单个供应商，自动重试同 pool 内的其他 SK。
// 当前 key 失败 → 标记 invalid → 检查 pool 是否还有可用 key → 有则重试，无则返回错误。
func (a *ApipoolSearchImpl) callProviderWithRetry(p apipoolProvider, query string, lookbackDays int) ([]SearchResult, error) {
	// 免费引擎无 pool，只调一次
	if p.pool == nil {
		return a.callSingle(p, query, lookbackDays)
	}

	var lastErr error
	for {
		results, err := a.callSingle(p, query, lookbackDays)
		if err == nil {
			return results, nil
		}
		// 优先按 KeyError 精确标记本次实际使用的 key（并发安全）；
		// 无 KeyError（如免费引擎/内容为空等非 key 错误）时回退 MarkLastInvalid。
		var ke *KeyError
		if errors.As(err, &ke) {
			p.pool.MarkInvalid(ke.Key)
		} else {
			p.pool.MarkLastInvalid()
		}
		lastErr = err
		if p.pool.Available() == 0 {
			return nil, fmt.Errorf("%s 所有 SK 均失败: %w", p.engine.Name(), lastErr)
		}
		log.Infof("apipool: %s 当前 SK 失败，尝试下一个 SK", p.engine.Name())
	}
}

// callSingle 调用一次搜索引擎。
func (a *ApipoolSearchImpl) callSingle(p apipoolProvider, query string, lookbackDays int) ([]SearchResult, error) {
	var results []SearchResult
	var err error
	if lookbackDays > 0 {
		if tr, ok := p.engine.(SearchTimeRanger); ok {
			results, err = tr.SearchRawWithTimeRange(query, lookbackDays)
		} else {
			results, err = p.engine.SearchRaw(query)
		}
	} else {
		results, err = p.engine.SearchRaw(query)
	}
	if err != nil {
		return nil, err
	}
	if a.maxSize > 0 && len(results) > a.maxSize {
		results = results[:a.maxSize]
	}
	return results, nil
}

func (a *ApipoolSearchImpl) MergeContent(query string, results []SearchResult) (string, error) {
	if len(results) == 0 {
		return "", fmt.Errorf("没有搜索结果可以合并")
	}
	var buf strings.Builder
	buf.Grow(1024 * len(results))
	buf.WriteString(md.MDSearchHeader(query, len(results)))
	for i, val := range results {
		if ShowMeta {
			buf.WriteString(md.FormatMDScore(i+1, val.Title, val.Url, val.Engine, formatScore(val.Score), val.Content))
		} else {
			buf.WriteString(md.FormatMD(i+1, val.Title, val.Url, val.Content))
		}
	}
	return buf.String(), nil
}
