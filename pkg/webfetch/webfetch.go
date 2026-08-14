package webfetch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"websearch/pkg/config"
	"websearch/pkg/log"
	"websearch/pkg/mineru"

	webfetch "github.com/daidaiJ/go-webfetch"
)

// Result 封装 go-webfetch 的返回结果。
type Result struct {
	Title      string
	Mode       string // "inline" 或 "saved_to_file"
	Markdown   string
	FilePath   string
	TotalLines int
	TotalChars int
	AgentHint  string
}

// Fetcher 封装 go-webfetch Engine。
type Fetcher struct {
	engine    *webfetch.Engine
	mineru    *mineru.Client
	mineruOCR bool // 本地 PDF 库读不到文本时是否回退 MinerU OCR
}

// NewFromConfig 根据配置创建 Fetcher。proxyURL 为代理地址，空字符串表示不使用代理（仍回退到环境变量）。
func NewFromConfig(cfg config.CleanFetchConfig, pdfCfg config.PDFParserConfig, proxyURL string) (*Fetcher, error) {
	outputDir := cfg.FileOutputDir
	if outputDir == "" {
		outputDir = filepath.Join(os.TempDir(), "webfetch")
	}

	fileTTL := time.Duration(cfg.FileTTL) * time.Hour
	if fileTTL <= 0 {
		fileTTL = 24 * time.Hour
	}

	maxInlineLines := cfg.MaxInlineLines
	if maxInlineLines <= 0 {
		maxInlineLines = 100
	}

	timeout := time.Duration(cfg.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	engine, err := webfetch.New(webfetch.Config{
		BlockPrivateIP:  true,
		Timeout:         timeout,
		MaxInlineLines:  maxInlineLines,
		MaxInlineChars:  cfg.MaxInlineChars,
		FileOutputDir:   outputDir,
		FileTTL:         fileTTL,
		ProxyURL:        proxyURL,
		UseSystemProxy:  cfg.UseSystemProxy,
		MaxRetries:      cfg.MaxRetries,
	})
	if err != nil {
		return nil, fmt.Errorf("webfetch engine init failed: %w", err)
	}

	log.Infof("WebFetch 引擎已启用 (output_dir=%s, ttl=%s, max_inline_lines=%d, timeout=%s)", outputDir, fileTTL, maxInlineLines, timeout)

	f := &Fetcher{engine: engine, mineruOCR: pdfCfg.MinerUOCREnabled()}

	// 初始化 MinerU 客户端（有 Token 或开启 OCR 回退时）
	if pdfCfg.MinerUEnabled() {
		f.mineru = mineru.NewFromConfig(
			pdfCfg.MinerUToken,
			pdfCfg.GetMinerUModel(),
			pdfCfg.GetMinerULang(),
			pdfCfg.MinerUOcr,
			pdfCfg.GetMinerUFormula(),
			pdfCfg.GetMinerUTable(),
			proxyURL,
		)
		if pdfCfg.MinerUOcr {
			log.Infof("MinerU OCR 回退已启用 (本地 PDF 库读不到文本时使用, model=%s)", pdfCfg.GetMinerUModel())
		} else if pdfCfg.MinerUToken != "" {
			log.Infof("MinerU 精准解析 API 已启用 (远程 URL, model=%s)", pdfCfg.GetMinerUModel())
		}
	}

	return f, nil
}

// Fetch 抓取网页或解析 PDF（自动检测 file:// 路径）。
func (f *Fetcher) Fetch(ctx context.Context, rawURL string) (*Result, error) {
	// 本地 PDF 文件：先本地 PDF 库抽文本，读不到再按需走 MinerU OCR
	if strings.HasPrefix(rawURL, "file://") {
		localPath := strings.TrimPrefix(rawURL, "file://")
		// 处理 Windows 三斜杠格式 file:///C:/...
		if len(localPath) > 0 && localPath[0] == '/' && len(localPath) > 2 && localPath[2] == ':' {
			localPath = localPath[1:]
		}
		localPath = strings.ReplaceAll(localPath, "/", string(os.PathSeparator))

		return f.parseLocalPDF(ctx, localPath)
	}

	// 远程 URL：有 Token 时优先尝试 MinerU 精准 API
	if f.mineru != nil && f.mineru.HasToken() {
		md, err := f.mineru.ParseURL(ctx, rawURL)
		if err == nil {
			return &Result{
				Mode:     "inline",
				Markdown: md,
			}, nil
		}
		log.Infof("MinerU 精准 API 解析失败(%v)，回退到 webfetch", err)
	}

	res, err := f.engine.Fetch(ctx, rawURL)
	if err != nil {
		return nil, fmt.Errorf("%s", classifyError(err))
	}
	return &Result{
		Title:      res.Title,
		Mode:       res.Mode,
		Markdown:   res.Markdown,
		FilePath:   res.FilePath,
		TotalLines: res.TotalLines,
		TotalChars: res.TotalChars,
		AgentHint:  cleanAgentHint(res.AgentHint),
	}, nil
}

// parseLocalPDF 本地 PDF：优先 ledongthuc/pdf 文本提取；无文本且开启 mineru_ocr 时回退 MinerU。
func (f *Fetcher) parseLocalPDF(ctx context.Context, localPath string) (*Result, error) {
	result, err := f.parsePDFFile(ctx, localPath)
	if err == nil {
		return result, nil
	}

	if !needsOCRFallback(err) {
		return nil, err
	}

	if f.mineru == nil || !f.mineruOCR {
		return nil, fmt.Errorf("%w（可能是扫描件/图片型 PDF，本地库无法提取文本；可在配置中开启 pdf_parser.mineru_ocr 以使用 MinerU OCR）", err)
	}

	log.Infof("本地 PDF 库未提取到文本，尝试 MinerU OCR: %s", localPath)
	md, mineruErr := f.mineru.ParseFile(ctx, localPath)
	if mineruErr == nil {
		return &Result{
			Title:    filepath.Base(localPath),
			Mode:     "inline",
			Markdown: md,
		}, nil
	}
	if errors.Is(mineruErr, mineru.ErrFileTooLarge) {
		return nil, fmt.Errorf("%w；MinerU OCR 回退失败: 文件超过 Agent API 限制(10MB)", err)
	}
	return nil, fmt.Errorf("%w；MinerU OCR 回退失败: %v", err, mineruErr)
}

// needsOCRFallback 判断本地解析失败是否因无文本层（扫描件等），适合 OCR 回退。
func needsOCRFallback(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "no text extracted") ||
		strings.Contains(msg, "scanned/image-based PDF")
}

// parsePDFFile 解析本地 PDF 文件。
func (f *Fetcher) parsePDFFile(ctx context.Context, filePath string) (*Result, error) {
	res, err := f.engine.ParsePDFFile(ctx, filePath)
	if err != nil {
		return nil, fmt.Errorf("PDF 解析失败: %w", err)
	}
	if res.Error != "" {
		return nil, fmt.Errorf("PDF 解析失败: %s", res.Error)
	}
	mode := res.Mode
	if mode == "" && res.Markdown != "" {
		mode = "inline"
	}
	return &Result{
		Title:      res.Title,
		Mode:       mode,
		Markdown:   res.Markdown,
		FilePath:   res.FilePath,
		TotalLines: res.TotalLines,
		TotalChars: res.TotalChars,
		AgentHint:  cleanAgentHint(res.AgentHint),
	}, nil
}

// cleanAgentHint 去掉 AgentHint 中的预览部分（空白行和分隔线污染）。
func cleanAgentHint(hint string) string {
	if idx := strings.Index(hint, "预览（"); idx != -1 {
		return strings.TrimRight(hint[:idx], "\n")
	}
	return hint
}

// Close 关闭引擎。
func (f *Fetcher) Close() error {
	return f.engine.Close()
}

// classifyError 将 go-webfetch 的错误分类为用户友好的错误信息。
func classifyError(err error) string {
	var notFound *webfetch.NotFoundError
	var waf *webfetch.WAFError
	var empty *webfetch.EmptyContentError
	var ssrf *webfetch.SSRFError
	var timeout *webfetch.TimeoutError

	switch {
	case errors.As(err, &notFound):
		return fmt.Sprintf("页面不存在(%d)", notFound.StatusCode)
	case errors.As(err, &waf):
		return "被网站反爬机制拦截(WAF)"
	case errors.As(err, &empty):
		return "页面内容为空(可能被反爬)"
	case errors.As(err, &ssrf):
		return "不允许访问内网地址"
	case errors.As(err, &timeout):
		return "请求超时"
	default:
		return fmt.Sprintf("抓取失败: %v", err)
	}
}
