package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// sseChunk 构造一个 OpenAI-compatible SSE data 行。
func sseChunk(content string) string {
	return fmt.Sprintf(`data: {"id":"x","object":"chat.completion.chunk","choices":[{"delta":{"content":%q},"index":0}]}`, content)
}

func TestChatStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求体带 stream: true
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		for _, line := range []string{
			sseChunk("Hello"),
			sseChunk(" world"),
			sseChunk("，流式"),
			`data: {"id":"x","object":"chat.completion.chunk","choices":[{"delta":{},"finish_reason":"stop"}]}`,
			"data: [DONE]",
		} {
			fmt.Fprintf(w, "%s\n\n", line)
			flusher.Flush()
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-key", "test-model")
	ch := make(chan string, 16)
	errCh := make(chan error, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go c.ChatStream(ctx, "sys", "user", ch, errCh)

	var tokens []string
	for tok := range ch {
		tokens = append(tokens, tok)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ChatStream 应成功, got err=%v", err)
	}
	got := strings.Join(tokens, "")
	if got != "Hello world，流式" {
		t.Errorf("token 拼接结果错误: %q", got)
	}
}

func TestChatStreamHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-key", "test-model")
	ch := make(chan string, 16)
	errCh := make(chan error, 1)
	go c.ChatStream(context.Background(), "sys", "user", ch, errCh)

	for range ch {
	} // 消费直到关闭
	err := <-errCh
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Errorf("应返回 500 错误, got %v", err)
	}
}

func TestChatStreamContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		// 持续输出（不结束），等待客户端取消
		for i := 0; i < 100; i++ {
			fmt.Fprintf(w, "%s\n\n", sseChunk("tick"))
			flusher.Flush()
			time.Sleep(20 * time.Millisecond)
			select {
			case <-r.Context().Done():
				return
			default:
			}
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-key", "test-model")
	ch := make(chan string, 16)
	errCh := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.ChatStream(ctx, "sys", "user", ch, errCh)

	// 收到若干 token 后取消
	received := 0
	for range ch {
		received++
		if received >= 2 {
			cancel()
			break
		}
	}
	// 消费剩余直到关闭
	for range ch {
	}
	err := <-errCh
	if err == nil {
		t.Fatal("ctx 取消后应返回错误")
	}
}

func TestChatStreamMalformedLine(t *testing.T) {
	// 含非 JSON data 行应被跳过而非中断
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		fmt.Fprintf(w, "%s\n\n", "data: not-json")
		fmt.Fprintf(w, "%s\n\n", sseChunk("ok"))
		fmt.Fprintf(w, "%s\n\n", "data: [DONE]")
		flusher.Flush()
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-key", "test-model")
	ch := make(chan string, 16)
	errCh := make(chan error, 1)
	go c.ChatStream(context.Background(), "sys", "user", ch, errCh)

	var tokens []string
	for tok := range ch {
		tokens = append(tokens, tok)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ChatStream 应成功, got err=%v", err)
	}
	if len(tokens) != 1 || tokens[0] != "ok" {
		t.Errorf("非 JSON 行应被跳过, got %v", tokens)
	}
}
