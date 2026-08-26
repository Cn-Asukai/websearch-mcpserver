package academic

import (
	"strconv"
	"strings"
	"testing"

	"websearch/pkg/antirobot"
)

func TestGoogleScholarSearch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network integration test")
	}
	engine := NewGoogleScholar(antirobot.GoogleScholarOpts{Enabled: true}, nil)
	resp, err := engine.Search("chain of thought prompting", 1, antirobot.TimeRangeNone)
	if err != nil {
		t.Skipf("Google Scholar 不可达（国内网络预期行为）: %v", err)
	}
	if resp.Engine != "google_scholar" {
		t.Errorf("engine = %q, want google_scholar", resp.Engine)
	}
	if len(resp.Results) == 0 {
		t.Fatal("expected results, got 0")
	}

	r := resp.Results[0]
	t.Logf("Title: %s", r.Title)
	t.Logf("URL: %s", r.URL)
	t.Logf("Authors: %s", r.Authors)
	t.Logf("Journal: %s", r.Journal)
	t.Logf("PublishedAt: %s", r.PublishedAt)
	t.Logf("CitedBy: %d", r.CitedBy)
	t.Logf("PDFURL: %s", r.PDFURL)

	if r.Type != antirobot.ResultPaper {
		t.Errorf("type = %q, want paper", r.Type)
	}
	if r.Title == "" {
		t.Error("title is empty")
	}
	if r.Authors == "" {
		t.Error("authors is empty")
	}

	// 验证 markdown 格式
	md := r.Markdown()
	t.Logf("\n--- Markdown Output ---\n%s", md)
	if !strings.Contains(md, r.Title) {
		t.Error("markdown missing title")
	}
	if r.CitedBy > 0 && !strings.Contains(md, strconv.Itoa(r.CitedBy)) {
		t.Error("markdown missing cited_by count")
	}
}

func TestParseScholarMeta(t *testing.T) {
	tests := []struct {
		input        string
		wantAuthors  string
		wantJournal  string
		wantPubDate  string
		wantPubisher string
	}{
		{
			input:        "A Vaswani, N Shazeer, N Parmar - Advances in neural …, 2017 - proceedings.neurips.cc",
			wantAuthors:  "A Vaswani, N Shazeer, N Parmar",
			wantJournal:  "Advances in neural …",
			wantPubDate:  "2017",
			wantPubisher: "proceedings.neurips.cc",
		},
		{
			input:        "J Devlin, MW Chang, K Lee - arXiv preprint arXiv …, 2018 - arxiv.org",
			wantAuthors:  "J Devlin, MW Chang, K Lee",
			wantJournal:  "arXiv preprint arXiv …",
			wantPubDate:  "2018",
			wantPubisher: "arxiv.org",
		},
		{
			input:        "A Author - Some Publisher",
			wantAuthors:  "A Author",
			wantJournal:  "",
			wantPubDate:  "",
			wantPubisher: "Some Publisher",
		},
		{
			input:       "",
			wantAuthors: "",
		},
	}

	for _, tt := range tests {
		authors, journal, publisher, pubDate := parseScholarMeta(tt.input)
		if authors != tt.wantAuthors {
			t.Errorf("parseScholarMeta(%q): authors = %q, want %q", tt.input, authors, tt.wantAuthors)
		}
		if journal != tt.wantJournal {
			t.Errorf("parseScholarMeta(%q): journal = %q, want %q", tt.input, journal, tt.wantJournal)
		}
		if pubDate != tt.wantPubDate {
			t.Errorf("parseScholarMeta(%q): pubDate = %q, want %q", tt.input, pubDate, tt.wantPubDate)
		}
		if publisher != tt.wantPubisher {
			t.Errorf("parseScholarMeta(%q): publisher = %q, want %q", tt.input, publisher, tt.wantPubisher)
		}
	}
}

func TestParseCitedByText(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"Cited by 1234", 1234},
		{"Cited by 1", 1},
		{"", 0},
		{"Cited by ", 0},
	}
	for _, tt := range tests {
		got := parseCitedByText(tt.input)
		if got != tt.want {
			t.Errorf("parseCitedByText(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
