package search

import (
	"net/url"
	"sort"
	"strings"

	"websearch/pkg/log"
)

// ──────────────────────────────────────────────────────────────────────────────
// Wigolo 本地评分增强流水线：RRF 融合 + 局部信号 + 多层 Boost + 阀值过滤。
// 参考 docs/native-engine-optimization.md。整个流水线为纯启发式，不依赖任何 AI 模型。
//
// 评分公式：
//   final = rrf × dq × (0.5 + 0.5×la) × rare
//   final = final + consensus_boost + authority_boost
//   final = final × recency_factor        (仅时间敏感查询)
//   保留条件: final >= threshold（Top-1 / 每引擎保底）
// ──────────────────────────────────────────────────────────────────────────────

const rrfK = 60.0 // RRF 融合常数

// scoreBucket 单引擎按相关性排序的结果列表（提供 RRF 排名）。
type scoreBucket struct {
	name    string
	weight  float64
	results []SearchResult
}

// RRFScore 对单个 URL 在各引擎中的排名做 Reciprocal Rank Fusion。
// ranks: engine -> rank（0 基）；K <= 0 时取默认 60。
func RRFScore(ranks map[string]int, K float64) float64 {
	if K <= 0 {
		K = rrfK
	}
	var score float64
	for _, rank := range ranks {
		score += 1.0 / (K + float64(rank))
	}
	return score
}

// ConsensusBoost 多引擎共识加分（加性）。
func ConsensusBoost(numEngines int) float64 {
	switch {
	case numEngines <= 1:
		return 0
	case numEngines == 2:
		return 0.05
	case numEngines == 3:
		return 0.10
	default:
		return 0.12
	}
}

// normalizeURLKey 归一化 URL 作为跨引擎去重键（忽略 scheme / www / 末尾斜杠）。
func normalizeURLKey(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return strings.TrimRight(s, "/")
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	key := host + strings.TrimRight(u.EscapedPath(), "/")
	if u.RawQuery != "" {
		key += "?" + u.RawQuery
	}
	if key == "" {
		return strings.TrimRight(s, "/")
	}
	return key
}

// urlAgg 单个去重 URL 的聚合信息。
type urlAgg struct {
	res         SearchResult
	rrf         float64
	engines     map[string]struct{}
	engineOrder []string
}

// EnhanceResults 对多引擎按引擎排序的结果桶执行完整评分流水线，
// 返回统一评分、去重、过滤、排序、截断后的结果。
func EnhanceResults(query string, buckets []scoreBucket, threshold float64, maxSize int) []SearchResult {
	hasError := HasErrorToken(query)
	temporal := HasTemporalIntent(query)

	order := make([]string, 0)
	m := make(map[string]*urlAgg)

	for _, b := range buckets {
		w := b.weight
		if w <= 0 {
			w = 1.0
		}
		for rank, r := range b.results {
			key := normalizeURLKey(r.Url)
			if key == "" {
				continue
			}
			a, ok := m[key]
			if !ok {
				a = &urlAgg{res: r, engines: make(map[string]struct{})}
				m[key] = a
				order = append(order, key)
			} else {
				// 合并元数据：保留更完整的内容/标题/日期
				if len(r.Content) > len(a.res.Content) {
					a.res.Content = r.Content
				}
				if a.res.Title == "" && r.Title != "" {
					a.res.Title = r.Title
				}
				if a.res.PublishDate == "" && r.PublishDate != "" {
					a.res.PublishDate = r.PublishDate
				}
			}
			// RRF: 排名 0 基，与 spec 及 RRFScore 一致（顶部结果得 1/K）
			a.rrf += w * (1.0 / (rrfK + float64(rank)))
			if _, dup := a.engines[b.name]; !dup {
				a.engines[b.name] = struct{}{}
				a.engineOrder = append(a.engineOrder, b.name)
			}
		}
	}

	scored := make([]SearchResult, 0, len(order))
	for _, key := range order {
		a := m[key]
		la := LexicalAlignment(query, a.res.Title, a.res.Content)
		dq := DomainQuality(a.res.Url, query, hasError)
		rare := RareTermsFactor(query, a.res.Title, a.res.Content)

		score := a.rrf * dq * (0.5 + 0.5*la) * rare
		score += ConsensusBoost(len(a.engines))
		score += AuthorityBoost(query, a.res.Url, rare)
		score *= RecencyFactor(temporal, a.res.PublishDate)

		a.res.Score = score
		a.res.Engines = a.engineOrder
		scored = append(scored, a.res)
	}

	// 按最终分降序（稳定排序保证同分顺序确定）
	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	before := len(scored)
	scored = ApplyScoreFloor(scored, threshold)

	if maxSize > 0 && len(scored) > maxSize {
		scored = scored[:maxSize]
	}

	log.Infof("Wigolo 评分增强: intent=%s error=%v temporal=%v 去重后=%d 过滤后=%d 返回=%d",
		classifyIntent(query), hasError, temporal, before, len(scored), len(scored))
	return scored
}

// ApplyScoreFloor 相关性阀值过滤，裁剪低质量上下文：
//   - score < threshold 的结果被丢弃（真正的低质量裁剪）
//   - Top-1 永不过滤（保证至少有一条最相关结果）
//   - 至少返回 2 条（若原结果 >= 2），避免过度裁剪导致上下文过空
// 输入需已按 score 降序排序。
func ApplyScoreFloor(results []SearchResult, threshold float64) []SearchResult {
	if threshold <= 0 {
		threshold = 0.05
	}
	if len(results) == 0 {
		return results
	}

	kept := make([]SearchResult, 0, len(results))
	for i, r := range results {
		if i == 0 { // Top-1 永不过滤
			kept = append(kept, r)
			continue
		}
		if r.Score >= threshold {
			kept = append(kept, r)
		}
	}

	// 仅剩 Top-1 时补第二条，保证最少 2 条结果
	if len(kept) < 2 && len(results) >= 2 {
		kept = append(kept, results[1])
	}
	return kept
}
