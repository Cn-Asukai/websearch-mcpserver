package daemon

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// TestPostShutdown PostShutdown 应 POST 到 /__admin/shutdown 并成功返回。
func TestPostShutdown(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	port, err := strconv.Atoi(strings.TrimPrefix(srv.URL, "http://127.0.0.1:"))
	if err != nil {
		t.Fatalf("解析测试端口失败: %v", err)
	}

	if err := PostShutdown(port); err != nil {
		t.Fatalf("PostShutdown 应成功: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("请求方法应为 POST，实际 %s", gotMethod)
	}
	if gotPath != "/__admin/shutdown" {
		t.Errorf("请求路径应为 /__admin/shutdown，实际 %s", gotPath)
	}
}

// TestPostShutdownConnectionError 连接被拒绝时应返回错误而非 panic。
func TestPostShutdownConnectionError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("获取空闲端口失败: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	if err := PostShutdown(port); err == nil {
		t.Error("连接被拒绝时应返回错误")
	}
}
