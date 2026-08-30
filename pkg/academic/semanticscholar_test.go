package academic

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"websearch/pkg/antirobot"
)

// withShortSSBackoff 缩短退避等待，避免单测耗时。
func withShortSSBackoff(t *testing.T) {
	t.Helper()
	oldRetry, oldWait := ssRetryBackoff, ssDegradedWait
	ssRetryBackoff, ssDegradedWait = 10*time.Millisecond, 10*time.Millisecond
	t.Cleanup(func() { ssRetryBackoff, ssDegradedWait = oldRetry, oldWait })
}

// withSSTestServer 启动 httptest 服务并让 S2 引擎指向它。
// statusQueue 依序返回状态码（越界时重复最后一个），200 时回合法 JSON 体。
// 返回请求计数指针，供断言重试次数。
func withSSTestServer(t *testing.T, statusQueue []int) (reqCount *int, seenHeaders *[]string) {
	t.Helper()
	count := 0
	var headers []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers = append(headers, r.Header.Get("x-api-key"))
		idx := count
		if idx >= len(statusQueue) {
			idx = len(statusQueue) - 1
		}
		count++
		code := statusQueue[idx]
		w.WriteHeader(code)
		if code == 200 {
			fmt.Fprint(w, `{"total":0,"data":[]}`)
		}
	}))
	t.Cleanup(ts.Close)

	oldEndpoint := ssSearchEndpoint
	ssSearchEndpoint = ts.URL
	t.Cleanup(func() { ssSearchEndpoint = oldEndpoint })
	return &count, &headers
}

func TestSemanticScholarRetryWithKey(t *testing.T) {
	withShortSSBackoff(t)
	count, _ := withSSTestServer(t, []int{429, 200})

	e := NewSemanticScholar(antirobot.SemanticScholarOpts{Enabled: true, APIKey: "k"}, &http.Client{}).(*semanticScholarEngine)
	if _, err := e.Search("q", 1, antirobot.TimeRangeNone); err != nil {
		t.Fatalf("429 后重试应成功: %v", err)
	}
	if *count != 2 {
		t.Errorf("应恰好请求 2 次, got %d", *count)
	}
	if e.keyDisabled.Load() {
		t.Error("单次 429 不应触发降级")
	}
}

func TestSemanticScholarDegradeToAnonymous(t *testing.T) {
	withShortSSBackoff(t)
	count, headers := withSSTestServer(t, []int{429, 429, 200})

	e := NewSemanticScholar(antirobot.SemanticScholarOpts{Enabled: true, APIKey: "k"}, &http.Client{}).(*semanticScholarEngine)
	if _, err := e.Search("q", 1, antirobot.TimeRangeNone); err != nil {
		t.Fatalf("降级后第三次请求应成功: %v", err)
	}
	if *count != 3 {
		t.Errorf("应恰好请求 3 次, got %d", *count)
	}
	want := "[k k ]"
	if fmt.Sprint(*headers) != want {
		t.Errorf("x-api-key 序列 = %q, want %q（第三次应匿名）", fmt.Sprint(*headers), want)
	}
	if !e.keyDisabled.Load() {
		t.Error("连续 429 后应置 keyDisabled")
	}
}

func TestSemanticScholarAnonymousNoRetry(t *testing.T) {
	withShortSSBackoff(t)
	count, _ := withSSTestServer(t, []int{429, 200}) // 第二个 200 不应被请求到

	e := NewSemanticScholar(antirobot.SemanticScholarOpts{Enabled: true}, &http.Client{})
	_, err := e.Search("q", 1, antirobot.TimeRangeNone)
	if err == nil {
		t.Fatal("匿名 429 应立即报错")
	}
	if *count != 1 {
		t.Errorf("匿名 429 不应重试, 请求数 = %d", *count)
	}
}
