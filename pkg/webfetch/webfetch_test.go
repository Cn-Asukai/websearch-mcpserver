package webfetch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"websearch/pkg/config"
)

func newTestFetcher(t *testing.T) *Fetcher {
	t.Helper()
	fetcher, err := NewFromConfig(config.CleanFetchConfig{
		Enabled:        true,
		FileTTL:        1,
		MaxInlineLines: 100,
	}, config.PDFParserConfig{}, config.ProxyConfig{}.GetProxyEndpoint())
	if err != nil {
		t.Fatalf("NewFromConfig failed: %v", err)
	}
	return fetcher
}

func TestFetchWebPage(t *testing.T) {
	fetcher := newTestFetcher(t)
	defer fetcher.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := fetcher.Fetch(ctx, "https://wmyskxz.cn/weekly/177/")
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if result.Title == "" {
		t.Error("expected non-empty title")
	}
	if result.Mode == "" {
		t.Error("expected non-empty mode")
	}
	if result.Mode == "inline" && result.Markdown == "" {
		t.Error("inline mode but markdown is empty")
	}
	if result.Mode == "saved_to_file" && result.FilePath == "" {
		t.Error("saved_to_file mode but file path is empty")
	}

	t.Logf("Title: %s", result.Title)
	t.Logf("Mode: %s", result.Mode)
	if result.Mode == "inline" {
		t.Logf("Markdown length: %d chars", len(result.Markdown))
	} else {
		t.Logf("File: %s (%d lines, %d chars)", result.FilePath, result.TotalLines, result.TotalChars)
	}
}

func TestFetchPDF(t *testing.T) {
	// 需要本地 PDF 文件时通过环境变量传入，避免硬编码路径
	pdfPath := os.Getenv("TEST_PDF_PATH")
	if pdfPath == "" {
		t.Skip("TEST_PDF_PATH not set, skipping PDF test")
	}
	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		t.Skipf("PDF file not found: %s", pdfPath)
	}

	fetcher := newTestFetcher(t)
	defer fetcher.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	absPath, _ := filepath.Abs(pdfPath)
	// Windows file:// URL 需要三斜杠 + 正斜杠
	fileURL := "file:///" + strings.ReplaceAll(absPath, `\`, "/")

	result, err := fetcher.Fetch(ctx, fileURL)
	if err != nil {
		t.Fatalf("Fetch PDF failed: %v", err)
	}

	if result.Title == "" {
		t.Error("expected non-empty title")
	}
	if result.Mode == "inline" && result.Markdown == "" {
		t.Error("inline mode but markdown is empty")
	}

	t.Logf("Title: %s", result.Title)
	t.Logf("Mode: %s", result.Mode)
	if result.Mode == "inline" {
		t.Logf("Markdown length: %d chars", len(result.Markdown))
	} else {
		t.Logf("File: %s (%d lines, %d chars)", result.FilePath, result.TotalLines, result.TotalChars)
	}
}

func TestClassifyErrors(t *testing.T) {
	tests := []struct {
		name     string
		input    error
		expected string
	}{
		{"nil error", nil, ""},
	}
	for _, tt := range tests {
		if tt.input != nil {
			t.Run(tt.name, func(t *testing.T) {
				got := classifyError(tt.input)
				if !strings.Contains(got, tt.expected) {
					t.Errorf("classifyError(%v) = %q, want contains %q", tt.input, got, tt.expected)
				}
			})
		}
	}
}

func TestNeedsOCRFallback(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"no text extracted", fmt.Errorf("PDF 解析失败: no text extracted — possible causes: scanned"), true},
		{"scanned mention", fmt.Errorf("scanned/image-based PDF (no text layer)"), true},
		{"file not found", fmt.Errorf("PDF 解析失败: file not found: /tmp/x.pdf"), false},
		{"other error", fmt.Errorf("PDF 解析失败: open pdf: corrupt"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := needsOCRFallback(tt.err); got != tt.want {
				t.Errorf("needsOCRFallback(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestNewFromConfig_MinerUOCR(t *testing.T) {
	fetcher, err := NewFromConfig(config.CleanFetchConfig{
		Enabled: true,
	}, config.PDFParserConfig{
		Enabled:   true,
		MinerUOcr: true,
	}, "")
	if err != nil {
		t.Fatalf("NewFromConfig failed: %v", err)
	}
	defer fetcher.Close()

	if fetcher.mineru == nil {
		t.Error("expected mineru client when mineru_ocr=true")
	}
	if !fetcher.mineruOCR {
		t.Error("expected mineruOCR=true")
	}
}

func TestNewFromConfig_NoMinerUWithoutOCROrToken(t *testing.T) {
	fetcher, err := NewFromConfig(config.CleanFetchConfig{
		Enabled: true,
	}, config.PDFParserConfig{
		Enabled: true,
	}, "")
	if err != nil {
		t.Fatalf("NewFromConfig failed: %v", err)
	}
	defer fetcher.Close()

	if fetcher.mineru != nil {
		t.Error("expected no mineru client when neither token nor ocr")
	}
	if fetcher.mineruOCR {
		t.Error("expected mineruOCR=false")
	}
}

func TestParseLocalPDF_OCRHintWithoutConfig(t *testing.T) {
	fetcher := newTestFetcher(t)
	defer fetcher.Close()

	_, err := fetcher.parseLocalPDF(context.Background(), filepath.Join(os.TempDir(), "nonexistent-webfetch-ocr-test.pdf"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if strings.Contains(err.Error(), "mineru_ocr") {
		t.Errorf("file-not-found should not suggest mineru_ocr: %v", err)
	}
}

func TestNewFromConfigDefaults(t *testing.T) {
	fetcher, err := NewFromConfig(config.CleanFetchConfig{
		Enabled: true,
	}, config.PDFParserConfig{}, config.ProxyConfig{}.GetProxyEndpoint())
	if err != nil {
		t.Fatalf("NewFromConfig with defaults failed: %v", err)
	}
	defer fetcher.Close()
}

func TestFetchRuanyifengBlog(t *testing.T) {
	fetcher := newTestFetcher(t)
	defer fetcher.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := fetcher.Fetch(ctx, "https://www.ruanyifeng.com/blog/2026/07/weekly-issue-406.html")
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if result.Title == "" {
		t.Error("expected non-empty title")
	}
	t.Logf("Title: %s", result.Title)
	t.Logf("Mode: %s", result.Mode)
	if result.Mode == "inline" {
		t.Logf("Markdown length: %d chars", len(result.Markdown))
	} else {
		t.Logf("File: %s (%d lines, %d chars)", result.FilePath, result.TotalLines, result.TotalChars)
	}
}
