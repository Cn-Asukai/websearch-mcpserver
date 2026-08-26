package mcpserver

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"websearch/pkg/config"
	"websearch/pkg/log"
	"websearch/pkg/search"
	"websearch/pkg/webfetch"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mockSearch struct {
	name    string
	results []search.SearchResult
	merged  string
	err     error
	lastQ   string
}

func (m *mockSearch) Name() string { return m.name }
func (m *mockSearch) Search(query string) (string, error) {
	return m.merged, m.err
}
func (m *mockSearch) SearchRaw(query string) ([]search.SearchResult, error) {
	m.lastQ = query
	if m.err != nil {
		return nil, m.err
	}
	return m.results, nil
}
func (m *mockSearch) MergeContent(query string, results []search.SearchResult) (string, error) {
	if m.merged != "" {
		return m.merged, nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "query=%s", query)
	for _, r := range results {
		fmt.Fprintf(&b, "\n%s %s", r.Title, r.Url)
	}
	return b.String(), nil
}

type mockAcademic struct {
	engines []string
	results []search.SearchResult
	err     error
	lastQ   string
	lastOpt search.AcademicSearchOptions
}

func (m *mockAcademic) AcademicEngines() []string { return m.engines }
func (m *mockAcademic) SearchAcademicRaw(query string, opts ...search.AcademicSearchOptions) ([]search.SearchResult, error) {
	m.lastQ = query
	if len(opts) > 0 {
		m.lastOpt = opts[0]
	}
	if m.err != nil {
		return nil, m.err
	}
	return m.results, nil
}

func initTestLogger() {
	log.NewLoggerTo(io.Discard, "", config.LogConfig{})
}

func restoreGlobals(t *testing.T) {
	t.Helper()
	oldSearch := searchapi
	oldAcad := academicSearcher
	oldWF := webfetchInst
	oldCache := cacheInst
	oldSum := summarizerInst
	oldJina := jinaInst
	oldFallback := fallbackSearch
	oldSmart := smartSearchConf
	t.Cleanup(func() {
		searchapi = oldSearch
		academicSearcher = oldAcad
		webfetchInst = oldWF
		cacheInst = oldCache
		summarizerInst = oldSum
		jinaInst = oldJina
		fallbackSearch = oldFallback
		smartSearchConf = oldSmart
	})
}

func listToolNames(t *testing.T, conf config.Config) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	t1, t2 := mcp.NewInMemoryTransports()
	server := NewMCPServer(conf, nil)
	ss, err := server.Connect(ctx, t1, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer ss.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0"}, nil)
	cs, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer cs.Close()

	res, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	names := make([]string, 0, len(res.Tools))
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
	}
	slices.Sort(names)
	return names
}

func toolByName(t *testing.T, conf config.Config, name string) *mcp.Tool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	t1, t2 := mcp.NewInMemoryTransports()
	ss, err := NewMCPServer(conf, nil).Connect(ctx, t1, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "c", Version: "0"}, nil).Connect(ctx, t2, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	res, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range res.Tools {
		if tool.Name == name {
			return tool
		}
	}
	t.Fatalf("tool %s not found", name)
	return nil
}

func TestNewMCPServer_RegisterTools(t *testing.T) {
	initTestLogger()
	restoreGlobals(t)

	tests := []struct {
		name    string
		setup   func()
		conf    config.Config
		want    []string
		wantNot []string
	}{
		{
			name: "bing on, academic mock, optional off",
			setup: func() {
				academicSearcher = &mockAcademic{engines: []string{"arxiv"}}
				webfetchInst = nil
			},
			conf: config.Config{
				Bing:     config.BingConfig{Enabled: true},
				Academic: config.AcademicConfig{Enabled: true},
			},
			want:    []string{"academicsearch", "smartsearch"},
			wantNot: []string{"cleanfetch", "pdf_parser"},
		},
		{
			name: "academic enabled but searcher nil",
			setup: func() {
				academicSearcher = nil
			},
			conf: config.Config{
				Bing:     config.BingConfig{Enabled: true},
				Academic: config.AcademicConfig{Enabled: true},
			},
			want:    []string{"smartsearch"},
			wantNot: []string{"academicsearch"},
		},
		{
			name: "bing off, optional on with dummy webfetch",
			setup: func() {
				academicSearcher = nil
				webfetchInst = &webfetch.Fetcher{}
			},
			conf: config.Config{
				Bing:       config.BingConfig{Enabled: false},
				CleanFetch: config.CleanFetchConfig{Enabled: true},
				PDFParser:  config.PDFParserConfig{Enabled: true},
			},
			want:    []string{"cleanfetch", "pdf_parser"},
			wantNot: []string{"smartsearch", "academicsearch"},
		},
		{
			name: "pdf enabled but webfetch nil",
			setup: func() {
				webfetchInst = nil
			},
			conf: config.Config{
				PDFParser: config.PDFParserConfig{Enabled: true},
			},
			wantNot: []string{"pdf_parser"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			academicSearcher = nil
			webfetchInst = nil
			searchapi = nil
			if tt.setup != nil {
				tt.setup()
			}
			names := listToolNames(t, tt.conf)
			for _, w := range tt.want {
				if !slices.Contains(names, w) {
					t.Errorf("expected %s, got %v", w, names)
				}
			}
			for _, n := range tt.wantNot {
				if slices.Contains(names, n) {
					t.Errorf("did not expect %s, got %v", n, names)
				}
			}
		})
	}
}

func TestNewMCPServer_SmartsearchIntentDescription(t *testing.T) {
	initTestLogger()
	restoreGlobals(t)
	academicSearcher = nil
	webfetchInst = nil

	noLLM := toolByName(t, config.Config{Bing: config.BingConfig{Enabled: true}}, "smartsearch")
	if strings.Contains(noLLM.Description, "intent") {
		t.Errorf("no-LLM description should not mention intent, got %q", noLLM.Description)
	}

	withLLM := toolByName(t, config.Config{
		Bing: config.BingConfig{Enabled: true},
		LLM:  config.LLMConfig{BaseURL: "http://x", APIKey: "k", ModelId: "m"},
	}, "smartsearch")
	if !strings.Contains(withLLM.Description, "intent") {
		t.Errorf("LLM description should mention intent, got %q", withLLM.Description)
	}
}

func TestBuildAcademicToolDescription_MockEngines(t *testing.T) {
	restoreGlobals(t)
	academicSearcher = &mockAcademic{engines: []string{"arxiv", "custom_engine"}}
	desc := buildAcademicToolDescription()
	if !strings.Contains(desc, "arxiv") {
		t.Errorf("missing arxiv: %s", desc)
	}
	if !strings.Contains(desc, "CS/物理/数学") {
		t.Errorf("missing known engine desc: %s", desc)
	}
	if !strings.Contains(desc, "custom_engine") {
		t.Errorf("unknown engine should still be listed: %s", desc)
	}
}

func TestRunWithTransport_CallToolsMocked(t *testing.T) {
	initTestLogger()
	restoreGlobals(t)

	ms := &mockSearch{
		name:    "mock",
		results: []search.SearchResult{{Title: "Go generics", Url: "https://example.com/go", Content: "body"}},
		merged:  "MERGED:Go generics",
	}
	ma := &mockAcademic{
		engines: []string{"arxiv"},
		results: []search.SearchResult{{Title: "Attention paper", Url: "https://arxiv.org/abs/1", Type: "paper"}},
	}
	searchapi = ms
	academicSearcher = ma
	webfetchInst = nil
	cacheInst = nil
	summarizerInst = nil
	fallbackSearch = nil

	conf := config.Config{
		Bing:     config.BingConfig{Enabled: true},
		Academic: config.AcademicConfig{Enabled: true},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	t1, t2 := mcp.NewInMemoryTransports()
	errCh := make(chan error, 1)
	go func() {
		errCh <- runWithTransport(ctx, conf, t1, slog.New(slog.DiscardHandler))
	}()

	cs, err := mcp.NewClient(&mcp.Implementation{Name: "c", Version: "0"}, nil).Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}

	listed, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	var names []string
	for _, tool := range listed.Tools {
		names = append(names, tool.Name)
	}
	if !slices.Contains(names, "smartsearch") || !slices.Contains(names, "academicsearch") {
		t.Fatalf("tools = %v", names)
	}

	web, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "smartsearch",
		Arguments: map[string]any{"query": "Go generics", "time_range": 1},
	})
	if err != nil {
		t.Fatalf("smartsearch: %v", err)
	}
	text := toolText(t, web)
	if !strings.Contains(text, "MERGED:Go generics") {
		t.Errorf("smartsearch result = %q", text)
	}
	if ms.lastQ != "Go generics" {
		t.Errorf("mock SearchRaw query = %q", ms.lastQ)
	}

	acad, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "academicsearch",
		Arguments: map[string]any{
			"query":      "transformer",
			"engines":    []string{"arxiv"},
			"time_range": "year",
			"page":       2,
		},
	})
	if err != nil {
		t.Fatalf("academicsearch: %v", err)
	}
	if ma.lastQ != "transformer" {
		t.Errorf("academic query = %q", ma.lastQ)
	}
	if ma.lastOpt.Page != 2 || ma.lastOpt.TimeRange != "year" {
		t.Errorf("academic opts = %+v", ma.lastOpt)
	}
	if !strings.Contains(toolText(t, acad), "Attention paper") && !strings.Contains(toolText(t, acad), "MERGED") {
		t.Errorf("academic result = %q", toolText(t, acad))
	}

	_ = cs.Close()
	cancel()
	select {
	case <-errCh:
	case <-time.After(3 * time.Second):
		t.Error("runWithTransport did not return after cancel")
	}
}

func TestRunWithTransport_SearchError(t *testing.T) {
	initTestLogger()
	restoreGlobals(t)
	searchapi = &mockSearch{name: "mock", err: fmt.Errorf("engine down")}
	academicSearcher = nil
	fallbackSearch = nil
	cacheInst = nil

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	t1, t2 := mcp.NewInMemoryTransports()
	go func() {
		_ = runWithTransport(ctx, config.Config{Bing: config.BingConfig{Enabled: true}}, t1, slog.New(slog.DiscardHandler))
	}()

	cs, err := mcp.NewClient(&mcp.Implementation{Name: "c", Version: "0"}, nil).Connect(ctx, t2, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "smartsearch",
		Arguments: map[string]any{"query": "x"},
	})
	if err != nil {
		return
	}
	if res == nil || !res.IsError {
		t.Fatalf("expected tool error, got err=%v res=%v", err, res)
	}
}

func TestRegisterRouter_MountsMCP(t *testing.T) {
	initTestLogger()
	restoreGlobals(t)
	academicSearcher = nil
	webfetchInst = nil

	mux := http.NewServeMux()
	RegisterRouter(mux, config.Config{Bing: config.BingConfig{Enabled: true}})

	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code == http.StatusNotFound {
		t.Fatal("/mcp was not registered")
	}
}

func TestAuthMiddleware(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("no token configured -> pass through", func(t *testing.T) {
		h := AuthMiddleware(config.Config{}, ok)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("code = %d, want 200", w.Code)
		}
	})

	t.Run("missing header -> 401", func(t *testing.T) {
		h := AuthMiddleware(config.Config{AuthToken: "secret"}, ok)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("code = %d, want 401", w.Code)
		}
	})

	t.Run("wrong token -> 401", func(t *testing.T) {
		h := AuthMiddleware(config.Config{AuthToken: "secret"}, ok)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer wrong")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("code = %d, want 401", w.Code)
		}
	})

	t.Run("Bearer token -> 200", func(t *testing.T) {
		h := AuthMiddleware(config.Config{AuthToken: "secret"}, ok)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer secret")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("code = %d, want 200", w.Code)
		}
	})

	t.Run("X-API-Key -> 200", func(t *testing.T) {
		h := AuthMiddleware(config.Config{AuthToken: "secret"}, ok)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-API-Key", "secret")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("code = %d, want 200", w.Code)
		}
	})
}

func toolText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if res == nil || len(res.Content) == 0 {
		t.Fatal("empty tool result")
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content type %T", res.Content[0])
	}
	return tc.Text
}
