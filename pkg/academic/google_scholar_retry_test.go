package academic

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"websearch/pkg/antirobot"
)

// withScholarTestServer 启动 httptest 服务并让 GS 引擎指向它，返回收到的请求 UA 列表。
func withScholarTestServer(t *testing.T, handler func(w http.ResponseWriter, hits int)) (uas *[]string, hits *int) {
	t.Helper()
	var seenUAs []string
	n := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenUAs = append(seenUAs, r.Header.Get("User-Agent"))
		handler(w, n)
		n++
	}))
	t.Cleanup(ts.Close)

	oldFormat := scholarURLFormat
	scholarURLFormat = ts.URL + "/scholar?%[2]s" // %[2]s：跳过 domain 参数
	t.Cleanup(func() { scholarURLFormat = oldFormat })
	return &seenUAs, &n
}

// withShortScholarBackoff 缩短退避等待；固定随机序列让 UA 轮换可断言。
func withShortScholarBackoff(t *testing.T, randSeq []int) {
	t.Helper()
	oldBase := scholarRetryBase
	scholarRetryBase = time.Millisecond
	t.Cleanup(func() { scholarRetryBase = oldBase })

	oldRand := scholarRandIntn
	i := 0
	scholarRandIntn = func(_ int) int {
		v := randSeq[i%len(randSeq)]
		i++
		return v
	}
	t.Cleanup(func() { scholarRandIntn = oldRand })
}

// noRedirectClient 收到 3xx 时不跟随跳转，便于断言 Location。
func noRedirectClient() *http.Client {
	return &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}}
}

func TestGoogleScholarRetryUARotation(t *testing.T) {
	withShortScholarBackoff(t, []int{0, 1}) // 第一次选 UA[0]，重试换 UA[1]
	uas, _ := withScholarTestServer(t, func(w http.ResponseWriter, hits int) {
		if hits == 0 {
			w.WriteHeader(429)
			return
		}
		fmt.Fprint(w, `<div data-rp="1"><h3><a href="https://arxiv.org/abs/1">Paper</a></h3><div class="gs_a">A - 2024</div></div>`)
	})

	e := NewGoogleScholar(antirobot.GoogleScholarOpts{Enabled: true}, noRedirectClient())
	resp, err := e.Search("q", 1, antirobot.TimeRangeNone)
	if err != nil {
		t.Fatalf("429 后重试应成功: %v", err)
	}
	if len(resp.Results) != 1 || resp.Results[0].Title != "Paper" {
		t.Errorf("解析结果异常: %+v", resp.Results)
	}
	got := *uas
	if len(got) != 2 || got[0] != scholarUserAgents[0] || got[1] != scholarUserAgents[1] {
		t.Errorf("两次请求应轮换 UA, got %q", got)
	}
}

func TestGoogleScholarCaptchaNoRetry(t *testing.T) {
	withShortScholarBackoff(t, []int{0})
	_, hits := withScholarTestServer(t, func(w http.ResponseWriter, _ int) {
		w.Header().Set("Location", "/sorry/index")
		w.WriteHeader(302)
	})

	e := NewGoogleScholar(antirobot.GoogleScholarOpts{Enabled: true}, noRedirectClient())
	_, err := e.Search("q", 1, antirobot.TimeRangeNone)
	if err == nil || !strings.Contains(err.Error(), "CAPTCHA") {
		t.Fatalf("CAPTCHA 跳转应报错且信息含 CAPTCHA, got %v", err)
	}
	if *hits != 1 {
		t.Errorf("CAPTCHA 不应重试, 请求数 = %d", *hits)
	}
}

func TestGoogleScholarExhaustedRetries(t *testing.T) {
	withShortScholarBackoff(t, []int{0})
	_, hits := withScholarTestServer(t, func(w http.ResponseWriter, _ int) {
		w.WriteHeader(403)
	})

	e := NewGoogleScholar(antirobot.GoogleScholarOpts{Enabled: true}, noRedirectClient())
	_, err := e.Search("q", 1, antirobot.TimeRangeNone)
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("重试耗尽应返回最后一次错误, got %v", err)
	}
	if *hits != 3 {
		t.Errorf("403 应重试 2 次（共 3 次请求）, got %d", *hits)
	}
}

// TestGoogleScholarCaptchaForm 拦截页以 gs_captcha_f 表单返回（HTTP 200）时同样识别。
func TestGoogleScholarCaptchaForm(t *testing.T) {
	withShortScholarBackoff(t, []int{0})
	_, hits := withScholarTestServer(t, func(w http.ResponseWriter, _ int) {
		fmt.Fprint(w, `<form id="gs_captcha_f" action="/sorry">captcha</form>`)
	})

	e := NewGoogleScholar(antirobot.GoogleScholarOpts{Enabled: true}, noRedirectClient())
	_, err := e.Search("q", 1, antirobot.TimeRangeNone)
	if err == nil || !strings.Contains(err.Error(), "CAPTCHA") {
		t.Fatalf("CAPTCHA 表单应报错, got %v", err)
	}
	if *hits != 1 {
		t.Errorf("CAPTCHA 不应重试, 请求数 = %d", *hits)
	}
}
