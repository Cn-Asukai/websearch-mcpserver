package mcpserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"websearch/pkg/cache"
	"websearch/pkg/config"
	"websearch/pkg/jina"
	"websearch/pkg/log"
	"websearch/pkg/search"
	"websearch/pkg/summarizer"
	"websearch/pkg/webfetch"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// SearchParamsWithIntent LLM 摘要启用时使用的参数（含 intent）。
type SearchParamsWithIntent struct {
	Query     string `json:"query" jsonschema:"description,搜索关键词：需精准凝练地表达核心检索意图（建议 2-6 个关键词或一个短句），只保留最能定位目标的词；不要把同义词、过程词、修饰词一股脑堆砌成关键词列表，否则会稀释相关性。例如用 'Go 泛型 性能' 而非 'Go 泛型 类型参数 编译 运行时 性能 基准 对比 优化 使用方法'"`
	Intent    string `json:"intent" jsonschema:"description,搜索意图，描述你希望通过搜索解决什么问题或获取什么信息。例如 '了解goroutine调度原理' '对比React和Vue的生态差异' '查找某API的用法示例'。提供意图后可获得更精准的结构化摘要"`
	TimeRange int    `json:"time_range,omitempty" jsonschema:"description,搜索时间范围（月），限制搜索最近N个月的内容。例如 1=近1个月，3=近3个月，6=近半年，12=近一年。默认3，0表示不限"`
}

// SearchParamsNoIntent LLM 摘要未启用时使用的参数（无 intent，节省上下文 token）。
type SearchParamsNoIntent struct {
	Query     string `json:"query" jsonschema:"description,搜索关键词：需精准凝练地表达核心检索意图（建议 2-6 个关键词或一个短句），只保留最能定位目标的词；不要把同义词、过程词、修饰词一股脑堆砌成关键词列表，否则会稀释相关性。例如用 'Go 泛型 性能' 而非 'Go 泛型 类型参数 编译 运行时 性能 基准 对比 优化 使用方法'"`
	TimeRange int    `json:"time_range,omitempty" jsonschema:"description,搜索时间范围（月），限制搜索最近N个月的内容。例如 1=近1个月，3=近3个月，6=近半年，12=近一年。默认3，0表示不限"`
}

// AcademicSearchParams 学术搜索参数。
type AcademicSearchParams struct {
	Query     string   `json:"query" jsonschema:"description,学术搜索关键词，例如 'transformer attention mechanism' 或 'CRISPR gene editing'"`
	Engines   []string `json:"engines,omitempty" jsonschema:"description,指定引擎子集（为空则使用全部已启用引擎）。示例: 医学论文用 [\"pubmed\"], CS预印本用 [\"arxiv\"], 物理/数学用 [\"arxiv\",\"crossref\"]"`
	TimeRange string   `json:"time_range,omitempty" jsonschema:"description,时间范围过滤。可选值: year（近一年）, month（近一月）, week（近一周）, day（近一天）。为空则不限"`
	Page      int      `json:"page,omitempty" jsonschema:"description,结果页码（默认 1），每页约 10 条"`
}

// CleanFetchParams cleanfetch 工具参数。
type CleanFetchParams struct {
	URL string `json:"url" jsonschema:"description,要抓取的网页 URL，例如 'https://example.com/article'"`
}

var (
	searchapi          search.SearchInf
	fallbackSearch     *search.BingSearchAdapter
	summarizerInst     *summarizer.Summarizer
	cacheInst          *cache.Cache
	jinaInst           *jina.Reader
	webfetchInst       *webfetch.Fetcher
	academicSearcher   search.AcademicSearcher
	smartSearchConf    config.SmartSearchConfig
	cleanFetchMaxSizeMB int
)

// Init 初始化 MCP 服务组件，通过 Option 模式按需加载。
func Init(conf config.Config, opts ...ServerOption) error {
	for _, opt := range opts {
		opt()
	}

	if searchapi == nil {
		return fmt.Errorf("搜索引擎未初始化，请检查配置")
	}
	return nil
}

func GetCache() *cache.Cache {
	return cacheInst
}

// GetWebFetch 返回 WebFetch 引擎实例（供 server 包关闭时清理）。
func GetWebFetch() *webfetch.Fetcher {
	return webfetchInst
}

// ── WebSearch 处理函数（两个版本适配不同 Params） ─────────────────────────────

// WebSearchWithIntent LLM 启用时的 tool handler。
func WebSearchWithIntent(ctx context.Context, req *mcp.CallToolRequest, params *SearchParamsWithIntent) (*mcp.CallToolResult, any, error) {
	return doWebSearch(ctx, req, params.Query, params.Intent, params.TimeRange)
}

// WebSearchNoIntent LLM 未启用时的 tool handler。
func WebSearchNoIntent(ctx context.Context, req *mcp.CallToolRequest, params *SearchParamsNoIntent) (*mcp.CallToolResult, any, error) {
	return doWebSearch(ctx, req, params.Query, "", params.TimeRange)
}

// AcademicSearchHandler 学术搜索 tool handler。
func AcademicSearchHandler(ctx context.Context, req *mcp.CallToolRequest, params *AcademicSearchParams) (*mcp.CallToolResult, any, error) {
	return doAcademicSearch(params.Query, params.Engines, params.TimeRange, params.Page)
}

// doWebSearch 通用网页搜索逻辑。
// timeRangeMonths 控制搜索时间范围（月），默认 3，0 表示不限。
// 摘要阶段优先流式推送（MCP progress notification），客户端可实时看到生成过程。
func doWebSearch(ctx context.Context, req *mcp.CallToolRequest, query, intent string, timeRangeMonths int) (*mcp.CallToolResult, any, error) {
	if searchapi == nil {
		return nil, nil, fmt.Errorf("api 初始化未完成")
	}

	// 默认3个月
	if timeRangeMonths == 0 {
		timeRangeMonths = 3
	}
	lookbackDays := timeRangeMonths * 30

	// ---- 缓存查询 ----
	if cacheInst != nil {
		rec, hitType, err := cacheInst.Lookup(query, intent, false)
		if err != nil {
			log.Errf("缓存查询异常，跳过缓存: %v", err)
		} else if rec != nil && !rec.Academic {
			switch hitType {
			case "exact_intent":
				if rec.Summary != "" {
					log.Infof("缓存命中(exact_intent+summary): query=%s", query)
					return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: rec.Summary}}}, nil, nil
				}
			case "query_only":
				results, parseErr := rec.GetRawResults()
				if parseErr == nil {
					log.Infof("缓存命中(query_only): query=%s", query)
					ret, mergeErr := formatRawResults(query, results)
					if mergeErr != nil {
						return nil, nil, mergeErr
					}
					if intent != "" && summarizerInst != nil && rec.Summary == "" {
						go func() {
							defer func() {
								if r := recover(); r != nil {
									log.Errf("异步摘要 panic: %v", r)
								}
							}()
							output, sumErr := summarizerInst.Summarize(query, intent, results)
							if sumErr == nil {
								_ = cacheInst.UpdateSummary(query, intent, output)
								log.Infof("后台异步摘要完成: query=%s, intent=%s", query, intent)
							}
						}()
					}
					return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: ret}}}, nil, nil
				}
			}
		}
	}

	// ---- 搜索 ----
	engineName := searchapi.Name()
	var results []search.SearchResult
	var err error

	// 优先使用支持时间范围的接口
	if timeRanger, ok := searchapi.(search.SearchTimeRanger); ok {
		results, err = timeRanger.SearchRawWithTimeRange(query, lookbackDays)
	} else {
		results, err = searchapi.SearchRaw(query)
	}
	if err != nil {
		if fallbackSearch != nil && searchapi != fallbackSearch {
			log.Errf("主搜索引擎失败(%v)，回退到 Bing 引擎", err)
			results, err = fallbackSearch.SearchRaw(query)
			engineName = "bing"
		}
		if err != nil {
			return nil, nil, err
		}
	}

	// 单引擎模式下应用 smartsearch 过滤（HybridSearchImpl 已在 SearchRaw 内处理）
	if _, isHybrid := searchapi.(*search.HybridSearchImpl); !isHybrid {
		results = postSearchFilter(results, engineName)
	}

	// 有 intent 且 LLM 可用 → 生成摘要（优先流式推送，失败回退非流式）
	if intent != "" && summarizerInst != nil {
		var output string
		var sumErr error
		if req != nil && req.Session != nil {
			output, sumErr = streamSummarize(ctx, req, query, intent, results)
			if sumErr != nil {
				log.Errf("LLM 流式摘要失败，回退到非流式摘要: %v", sumErr)
			}
		}
		if sumErr != nil {
			output, sumErr = summarizerInst.Summarize(query, intent, results)
		}
		if sumErr == nil {
			if cacheInst != nil {
				_ = cacheInst.Store(query, intent, false, results, output)
			}
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: output}}}, nil, nil
		}
		log.Errf("LLM 摘要失败，回退到原始结果: %v", sumErr)
	}

	ret, err := formatRawResults(query, results)
	if err != nil {
		return nil, nil, err
	}
	if cacheInst != nil {
		_ = cacheInst.Store(query, intent, false, results, "")
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: ret}}}, nil, nil
}

// doAcademicSearch 学术搜索逻辑。
func doAcademicSearch(query string, engines []string, timeRange string, page int) (*mcp.CallToolResult, any, error) {
	if academicSearcher == nil {
		return nil, nil, fmt.Errorf("学术搜索引擎未启用，请检查配置 bing.academic 是否为 true")
	}

	// ---- 缓存查询 ----
	enginesKey := strings.Join(engines, ",")
	cacheKey := query + "|" + timeRange + "|" + enginesKey
	if cacheInst != nil {
		rec, hitType, err := cacheInst.Lookup(cacheKey, "", true)
		if err != nil {
			log.Errf("缓存查询异常，跳过缓存: %v", err)
		} else if rec != nil && rec.Academic && hitType == "query_only" {
			results, parseErr := rec.GetRawResults()
			if parseErr == nil {
				log.Infof("学术缓存命中: query=%s", query)
				ret, mergeErr := formatAcademicResults(query, results)
				if mergeErr == nil {
					return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: ret}}}, nil, nil
				}
			}
		}
	}

	// ---- 学术搜索 ----
	opts := search.AcademicSearchOptions{
		Page:      page,
		TimeRange: timeRange,
		Engines:   engines,
	}

	log.Infof("学术搜索: query=%s, engines=%v, timeRange=%s, page=%d", query, engines, timeRange, page)
	results, err := academicSearcher.SearchAcademicRaw(query, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("学术搜索失败: %w", err)
	}

	ret, err := formatAcademicResults(query, results)
	if err != nil {
		return nil, nil, err
	}
	if cacheInst != nil {
		_ = cacheInst.Store(cacheKey, "", true, results, "")
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: ret}}}, nil, nil
}

// streamSummarize 流式生成摘要：通过 MCP progress notification 逐 token 推送，
// 同时累积全文，流结束后返回完整格式化摘要（含引用）。
// 客户端断开（ctx 取消）时自动中止。
func streamSummarize(ctx context.Context, req *mcp.CallToolRequest, query, intent string, results []search.SearchResult) (string, error) {
	notifyProgress(ctx, req, 1, 2, fmt.Sprintf("搜索完成，共 %d 条结果，正在生成摘要...", len(results)))

	ch := make(chan string, 64)
	errCh := make(chan error, 1)
	go summarizerInst.SummarizeStream(ctx, query, intent, results, ch, errCh)

	var sb strings.Builder
	for {
		select {
		case token, ok := <-ch:
			if !ok {
				// 流结束；errCh 可能已发送结果（缓冲 1，非阻塞读取）
				select {
				case err := <-errCh:
					if err != nil {
						return "", err
					}
				default:
				}
				return summarizer.FormatCitation(query, sb.String(), results), nil
			}
			sb.WriteString(token)
			notifyProgress(ctx, req, 2, 2, token)
		case err := <-errCh:
			return "", err
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
}

// notifyProgress 向 MCP 客户端推送进度通知（StreamableHTTP 传输下实时到达）。
// 推送失败仅记录日志，不影响主流程。
func notifyProgress(ctx context.Context, req *mcp.CallToolRequest, progress, total float64, message string) {
	if req == nil || req.Session == nil {
		return
	}
	params := &mcp.ProgressNotificationParams{
		ProgressToken: req.Params.GetProgressToken(),
		Progress:      progress,
		Total:         total,
		Message:       message,
	}
	if err := req.Session.NotifyProgress(ctx, params); err != nil {
		log.Debugf("progress notification 推送失败: %v", err)
	}
}

func formatAcademicResults(query string, results []search.SearchResult) (string, error) {
	if adapter, ok := academicSearcher.(*search.AcademicAdapter); ok {
		return adapter.MergeContent(query, results)
	}
	return searchapi.MergeContent(query, results)
}

func formatRawResults(query string, results []search.SearchResult) (string, error) {
	return searchapi.MergeContent(query, results)
}

// ── CleanFetch 工具 ──────────────────────────────────────────────────────────

// PDFParserParams pdf_parser 工具参数。
type PDFParserParams struct {
	Path string `json:"path" jsonschema:"description,本地 PDF 文件路径（支持绝对路径或 file:// 前缀）"`
}

// CleanFetch 通过 go-webfetch 抓取网页，失败时回退到 Jina Reader。
func CleanFetch(ctx context.Context, req *mcp.CallToolRequest, params *CleanFetchParams) (*mcp.CallToolResult, any, error) {
	if params.URL == "" {
		return nil, nil, fmt.Errorf("url 参数不能为空")
	}

	// ── 安全预检：DNS rebinding 防护 ──
	if err := validateURLSecurity(params.URL); err != nil {
		return nil, nil, err
	}

	// ── HEAD 预检：检测文件大小和类型 ──
	if err := headCheck(ctx, params.URL); err != nil {
		return nil, nil, err
	}

	// ── 第一层：go-webfetch（无需代理）──
	if webfetchInst != nil {
		result, err := webfetchInst.Fetch(ctx, params.URL)
		if err == nil {
			return formatWebFetchResult(result), nil, nil
		}
		log.Infof("webfetch 抓取失败(%v)，尝试回退到 Jina Reader", err)

		// ── 第二层：Jina Reader（需代理，jinaInst != nil 即表示代理已开启）──
		if jinaInst != nil {
			jinaResult, jinaErr := jinaInst.Fetch(params.URL)
			if jinaErr == nil {
				return formatJinaResult(jinaResult), nil, nil
			}
			return nil, nil, fmt.Errorf("webfetch: %v; Jina 兜底: %w", err, jinaErr)
		}
		return nil, nil, fmt.Errorf("webfetch 抓取失败: %v", err)
	}

	// webfetch 未初始化，仅用 Jina（兼容旧模式：仅代理+Jina Key）
	if jinaInst != nil {
		jinaResult, jinaErr := jinaInst.Fetch(params.URL)
		if jinaErr != nil {
			return nil, nil, fmt.Errorf("jina reader 抓取失败: %w", jinaErr)
		}
		return formatJinaResult(jinaResult), nil, nil
	}

	return nil, nil, fmt.Errorf("webfetch 和 jina reader 均未初始化")
}

// PDFParserHandler 本地 PDF 解析 tool handler。
func PDFParserHandler(ctx context.Context, req *mcp.CallToolRequest, params *PDFParserParams) (*mcp.CallToolResult, any, error) {
	if params.Path == "" {
		return nil, nil, fmt.Errorf("path 参数不能为空")
	}
	if webfetchInst == nil {
		return nil, nil, fmt.Errorf("webfetch 未初始化，请先启用 cleanfetch")
	}

	// 确保路径带 file:// 前缀，Windows 路径需三斜杠
	pdfPath := params.Path
	if !strings.HasPrefix(pdfPath, "file://") {
		pdfPath = "file:///" + strings.ReplaceAll(pdfPath, `\`, "/")
	}

	result, err := webfetchInst.Fetch(ctx, pdfPath)
	if err != nil {
		return nil, nil, fmt.Errorf("PDF 解析失败: %v", err)
	}
	return formatWebFetchResult(result), nil, nil
}

// formatJinaResult 将 Jina Reader 结果格式化为 MCP 返回。
func formatJinaResult(result *jina.FetchResult) *mcp.CallToolResult {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# %s\n\n", result.Title)
	if result.Description != "" {
		fmt.Fprintf(&sb, "> %s\n\n", result.Description)
	}
	if result.PublishedTime != "" {
		fmt.Fprintf(&sb, "**发布时间**: %s\n\n", result.PublishedTime)
	}
	sb.WriteString(result.Content)
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}}}
}

// formatWebFetchResult 将 go-webfetch 结果格式化为 MCP 返回。
func formatWebFetchResult(result *webfetch.Result) *mcp.CallToolResult {
	var sb strings.Builder
	if result.Mode == "inline" {
		if result.Title != "" {
			fmt.Fprintf(&sb, "# %s\n\n", result.Title)
		}
		sb.WriteString(result.Markdown)
	} else {
		// 大文本已存储到文件
		if result.Title != "" {
			fmt.Fprintf(&sb, "# %s\n\n", result.Title)
		}
		fmt.Fprintf(&sb, "内容已保存到文件（共 %d 行，%d 字符）\n\n", result.TotalLines, result.TotalChars)
		fmt.Fprintf(&sb, "**文件路径**: `%s`\n\n", result.FilePath)
		if result.AgentHint != "" {
			fmt.Fprintf(&sb, "**读取提示**: %s\n", result.AgentHint)
		}
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}}}
}

// postSearchFilter 对单引擎搜索结果应用 smartsearch 配置的 score 过滤和 maxsize 截断。
// HybridSearchImpl 已在 SearchRaw 内处理，此函数仅用于单引擎模式。
func postSearchFilter(results []search.SearchResult, engineName string) []search.SearchResult {
	if len(results) == 0 {
		return results
	}
	ec := smartSearchConf.Engines[engineName]

	// score 过滤
	results = search.FilterByScore(results, ec.MinScore)

	// 单引擎 maxsize 截断
	engineMax := ec.MaxSize
	if engineMax <= 0 {
		engineMax = 4 // defaultEngineMaxSize
	}
	// 引擎不回传 score 时，取 min(engineMax, ceil(globalMax/1))
	if smartSearchConf.MaxSize > 0 {
		hasScore := false
		for _, r := range results {
			if r.Score > 0 {
				hasScore = true
				break
			}
		}
		if !hasScore {
			perEngineCap := smartSearchConf.MaxSize // 单引擎时 ceil(maxSize/1) = maxSize
			if perEngineCap < engineMax {
				engineMax = perEngineCap
			}
		}
	}
	if engineMax > 0 && len(results) > engineMax {
		results = results[:engineMax]
	}

	// 全局 maxsize 截断
	if smartSearchConf.MaxSize > 0 && len(results) > smartSearchConf.MaxSize {
		search.SortByScore(results)
		results = results[:smartSearchConf.MaxSize]
	}

	return results
}

// ── CleanFetch 安全预检 ──────────────────────────────────────────────────────

// validateURLSecurity DNS rebinding 防护：解析域名并检查所有 IP 是否为内网地址。
// 与 go-webfetch 的 BlockPrivateIP 形成双重防护（MCP 层预检 + 库层连接时检查）。
func validateURLSecurity(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("URL 格式错误: %w", err)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("不支持的协议: %s（仅支持 http/https）", scheme)
	}

	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("URL 缺少主机名")
	}

	// 已知内网主机名直接拒绝
	if isPrivateHostFast(host) {
		return fmt.Errorf("不允许访问内网地址: %s", host)
	}

	// DNS 解析后检查 IP（防 DNS rebinding）
	ips, err := net.LookupHost(host)
	if err != nil {
		// DNS 解析失败不阻断（可能是临时 DNS 问题，由后续 fetch 报具体错误）
		log.Infof("DNS 解析失败（跳过安全检查）: %s: %v", host, err)
		return nil
	}
	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			continue
		}
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
			ip.IsLinkLocalMulticast() || ip.IsUnspecified() ||
			isCloudMetadata(ip) {
			return fmt.Errorf("不允许访问内网地址: %s → %s", host, ipStr)
		}
	}
	return nil
}

// isPrivateHostFast 快速检查主机名是否为已知内网地址（无需 DNS 解析）。
func isPrivateHostFast(host string) bool {
	switch host {
	case "localhost", "127.0.0.1", "::1", "0.0.0.0",
		"169.254.169.254", "metadata.google.internal":
		return true
	}
	// IPv6 回环
	if host == "[::1]" {
		return true
	}
	return false
}

// isCloudMetadata 检查 IP 是否为云厂商元数据地址。
func isCloudMetadata(ip net.IP) bool {
	// 169.254.169.254 (AWS/GCP/Azure/阿里云等)
	if ip.Equal(net.IPv4(169, 254, 169, 254)) {
		return true
	}
	// fd00::ec2:e4a:c2fe (AWS IPv6 元数据)
	if ip.IsLinkLocalUnicast() && ip.To4() == nil {
		return true
	}
	return false
}

// headCheck HEAD 预检：检查 Content-Length 防止下载过大文件。
func headCheck(ctx context.Context, rawURL string) error {
	maxSizeMB := cleanFetchMaxSizeMB
	if maxSizeMB <= 0 {
		maxSizeMB = 10
	}

	req, err := http.NewRequestWithContext(ctx, "HEAD", rawURL, nil)
	if err != nil {
		return nil // URL 构造失败不阻断，由后续 fetch 报错
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		// HEAD 失败不阻断（某些服务器不支持 HEAD）
		return nil
	}
	resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil // 状态码异常不阻断，由后续 fetch 报具体错误
	}

	if cl := resp.Header.Get("Content-Length"); cl != "" {
		size, err := strconv.ParseInt(cl, 10, 64)
		if err == nil && size > int64(maxSizeMB)*1024*1024 {
			return fmt.Errorf("文件过大（%.1fMB），超过限制（%dMB），如需抓取请调大 cleanfetch.max_fetch_size_mb",
				float64(size)/1024/1024, maxSizeMB)
		}
	}
	return nil
}
