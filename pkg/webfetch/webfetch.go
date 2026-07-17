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
	engine *webfetch.Engine
	mineru *mineru.Client
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

	f := &Fetcher{engine: engine}

	// 初始化 MinerU 客户端（可选增强）
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
		if pdfCfg.MinerUToken != "" {
			log.Infof("MinerU 增强已启用 (精准解析 API, model=%s)", pdfCfg.GetMinerUModel())
		} else {
			log.Info("MinerU 增强已启用 (Agent 轻量 API, 无 Token)")
		}
	}

	return f, nil
}

// Fetch 抓取网页或解析 PDF（自动检测 file:// 路径）。
func (f *Fetcher) Fetch(ctx context.Context, rawURL string) (*Result, error) {
	// 本地 PDF 文件
	if strings.HasPrefix(rawURL, "file://") {
		localPath := strings.TrimPrefix(rawURL, "file://")
		// 处理 Windows 三斜杠格式 file:///C:/...
		if len(localPath) > 0 && localPath[0] == '/' && len(localPath) > 2 && localPath[2] == ':' {
			localPath = localPath[1:]
		}
		localPath = strings.ReplaceAll(localPath, "/", string(os.PathSeparator))

		// 优先尝试 MinerU Agent API（本地文件签名上传）
		if f.mineru != nil {
			md, err := f.mineru.ParseFile(ctx, localPath)
			if err == nil {
				return &Result{
					Title:    filepath.Base(localPath),
					Mode:     "inline",
					Markdown: md,
				}, nil
			}
			if errors.Is(err, mineru.ErrFileTooLarge) {
				log.Infof("文件超过 MinerU Agent API 限制(10MB)，使用本地解析: %s", localPath)
			} else {
				log.Infof("MinerU 解析失败，回退本地解析: %v", err)
			}
		}

		return f.parsePDFFile(ctx, localPath)
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

// parsePDFFile 解析本地 PDF 文件。
func (f *Fetcher) parsePDFFile(ctx context.Context, filePath string) (*Result, error) {
	res, err := f.engine.ParsePDFFile(ctx, filePath)
	if err != nil {
		return nil, fmt.Errorf("PDF 解析失败: %w", err)
	}
	if res.Error != "" {
		return nil, fmt.Errorf("PDF 解析失败: %s", res.Error)
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
