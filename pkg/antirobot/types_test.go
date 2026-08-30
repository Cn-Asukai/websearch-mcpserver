package antirobot

import "testing"

// TestDeduplicateResults_DOIKey 同 DOI 不同 URL 的结果应保留首条（跨源去重）。
func TestDeduplicateResults_DOIKey(t *testing.T) {
	results := []Result{
		{Engine: "openalex", Title: "Paper A", URL: "https://openalex.org/W1", DOI: "10.1/abc"},
		{Engine: "crossref", Title: "Paper A", URL: "https://doi.org/10.1/ABC", DOI: "https://doi.org/10.1/abc"},
		{Engine: "arxiv", Title: "Paper B", URL: "https://arxiv.org/abs/1"},
	}
	got := DeduplicateResults(results)
	if len(got) != 2 {
		t.Fatalf("同 DOI 应合并, want 2 got %d: %+v", len(got), got)
	}
	if got[0].Engine != "openalex" {
		t.Errorf("应保留首次出现, got %s", got[0].Engine)
	}
}

// TestDeduplicateResults_URLFallback 无 DOI 结果按 URL 精确去重，行为与原实现一致。
func TestDeduplicateResults_URLFallback(t *testing.T) {
	results := []Result{
		{Engine: "a", URL: "https://example.com/x"},
		{Engine: "b", URL: "https://example.com/x"},
		{Engine: "c", URL: "https://example.com/y"},
	}
	got := DeduplicateResults(results)
	if len(got) != 2 {
		t.Fatalf("want 2 got %d", len(got))
	}
}
