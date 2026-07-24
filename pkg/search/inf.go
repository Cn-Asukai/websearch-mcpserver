package search

import (
	"fmt"
	"websearch/pkg/antirobot"
)

var DefaultSearchInf SearchInf

// ShowMeta 控制 MergeContent 输出中是否显示引擎来源和 score。
// 由工厂函数根据 smartsearch.show_meta 配置设置，默认 true。
var ShowMeta = true

const defaultEngineMaxSize = 4 // 单引擎默认最大结果数

type SearchResult struct {
	Title       string  `json:"title"`
	Url         string  `json:"url"`
	Content     string  `json:"content"`
	PublishDate string  `json:"publishedDate"`
	Score       float64 `json:"score,omitempty"`       // 搜索相关性分数（Tavily 等引擎回传，0 表示无分数）
	Engine      string  `json:"engine,omitempty"`      // 结果来源引擎名（首个返回该 URL 的引擎）
	Engines     []string `json:"engines,omitempty"`    // 返回该 URL 的全部引擎（Wigolo 评分增强的共识 Boost 使用）
	Type        string  `json:"type,omitempty"`        // "paper" 或 "web"，学术搜索时为 "paper"
	Authors     string  `json:"authors,omitempty"`     // 论文作者
	DOI         string  `json:"doi,omitempty"`         // 论文 DOI
	Journal     string  `json:"journal,omitempty"`     // 期刊/会议名
	CitedBy     int     `json:"cited_by,omitempty"`    // 被引次数
	PDFURL      string  `json:"pdf_url,omitempty"`     // PDF 链接
}

type SearchInf interface {
	Name() string
	Search(query string) (string, error)
	SearchRaw(query string) ([]SearchResult, error)
	MergeContent(query string, results []SearchResult) (string, error)
}

// SearchTimeRanger 支持按时间范围搜索的引擎可实现此可选接口。
// lookbackDays 控制搜索最近多少天的结果，0 表示使用引擎默认值。
type SearchTimeRanger interface {
	SearchRawWithTimeRange(query string, lookbackDays int) ([]SearchResult, error)
}

// AcademicSearchOptions 学术搜索可选参数。
type AcademicSearchOptions struct {
	Page      int      // 页码（默认 1）
	TimeRange string   // 时间范围: "year", "all"（默认 "all"）
	Engines   []string // 指定引擎子集（为空则使用全部已启用引擎）
}

// AcademicSearcher 支持学术搜索的引擎可实现此接口。
type AcademicSearcher interface {
	SearchAcademicRaw(query string, opts ...AcademicSearchOptions) ([]SearchResult, error)
	AcademicEngines() []string // 返回可用的学术引擎列表
}

// ParseTimeRange 将字符串转换为 antirobot.TimeRange。
func ParseTimeRange(s string) antirobot.TimeRange {
	switch s {
	case "day":
		return antirobot.TimeRangeDay
	case "week":
		return antirobot.TimeRangeWeek
	case "month":
		return antirobot.TimeRangeMonth
	case "year":
		return antirobot.TimeRangeYear
	default:
		return antirobot.TimeRangeNone
	}
}

// formatScore 将 score 格式化为显示字符串，score <= 0 时返回空。
func formatScore(score float64) string {
	if score <= 0 {
		return ""
	}
	return fmt.Sprintf("%.4f", score)
}
