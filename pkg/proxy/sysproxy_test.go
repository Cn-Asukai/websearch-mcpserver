package proxy

import (
	"testing"
	"time"
	"unsafe"
)

// utf16LEBytes 将字符串编码为 UTF-16LE 字节（含 NUL 结尾）。
func utf16LEBytes(s string) []byte {
	buf := make([]byte, 0, (len(s)+1)*2)
	for _, r := range s {
		buf = append(buf, byte(r), byte(r>>8))
	}
	buf = append(buf, 0, 0)
	return buf
}

// TestUTF16PtrToString 验证纯函数按缓冲区边界读取 UTF-16，不越界。
func TestUTF16PtrToString(t *testing.T) {
	t.Run("short string with NUL", func(t *testing.T) {
		buf := utf16LEBytes("http://127.0.0.1:7897")
		got := utf16PtrToString((*uint16)(unsafe.Pointer(&buf[0])), buf)
		if got != "http://127.0.0.1:7897" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("exactly fills buffer without NUL", func(t *testing.T) {
		buf := utf16LEBytes("abc")[:6] // 3 个 uint16，无 NUL
		got := utf16PtrToString((*uint16)(unsafe.Pointer(&buf[0])), buf)
		if got != "abc" {
			t.Errorf("got %q, want %q", got, "abc")
		}
	})

	t.Run("no NUL does not panic", func(t *testing.T) {
		buf := make([]byte, 200) // 100 个 uint16，无 NUL
		for i := 0; i < 100; i++ {
			buf[i*2] = byte('a' + i%26)
			buf[i*2+1] = 0
		}
		got := utf16PtrToString((*uint16)(unsafe.Pointer(&buf[0])), buf)
		if len(got) != 100 {
			t.Errorf("got length %d, want 100", len(got))
		}
	})

	t.Run("nil pointer", func(t *testing.T) {
		if got := utf16PtrToString(nil, nil); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("pointer at offset into buffer", func(t *testing.T) {
		buf := utf16LEBytes("xxhttp://p")
		p := (*uint16)(unsafe.Pointer(&buf[4])) // 跳过前 2 个 uint16（"xx"）
		got := utf16PtrToString(p, buf)
		if got != "http://p" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("pointer outside buffer", func(t *testing.T) {
		buf := utf16LEBytes("abc")
		var outside uint16
		got := utf16PtrToString(&outside, buf)
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("pointer at end of buffer", func(t *testing.T) {
		buf := utf16LEBytes("abc")
		end := (*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(&buf[0])) + uintptr(len(buf))))
		got := utf16PtrToString(end, buf)
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

// clearProxyEnv 清空代理环境变量，避免干扰 DetectSystemProxy 测试。
func clearProxyEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy", "ALL_PROXY", "all_proxy"} {
		t.Setenv(k, "")
	}
}

// TestDetectSystemProxy_TTLCache 验证 TTL 内两次调用只探测一次系统代理。
func TestDetectSystemProxy_TTLCache(t *testing.T) {
	clearProxyEnv(t)
	calls := 0
	old := systemProxyDetector
	systemProxyDetector = func() string {
		calls++
		return "http://127.0.0.1:7897"
	}
	defer func() { systemProxyDetector = old }()
	osProxyCache.Store(osProxyCacheEntry{}) // 清缓存

	if ep := DetectSystemProxy(); ep != "http://127.0.0.1:7897" {
		t.Fatalf("first call got %q", ep)
	}
	if ep := DetectSystemProxy(); ep != "http://127.0.0.1:7897" {
		t.Fatalf("second call got %q", ep)
	}
	if calls != 1 {
		t.Errorf("systemProxyDetector calls = %d, want 1 (TTL 内应命中缓存)", calls)
	}
}

// TestDetectSystemProxy_TTLExpiry 验证 TTL 过期后重新探测。
func TestDetectSystemProxy_TTLExpiry(t *testing.T) {
	clearProxyEnv(t)
	calls := 0
	old := systemProxyDetector
	systemProxyDetector = func() string {
		calls++
		return "http://127.0.0.1:7897"
	}
	defer func() { systemProxyDetector = old }()
	osProxyCache.Store(osProxyCacheEntry{})

	DetectSystemProxy() // calls = 1
	// 伪造过期缓存
	osProxyCache.Store(osProxyCacheEntry{at: time.Now().Add(-2 * systemProxyTTL), ep: "http://127.0.0.1:7897"})
	DetectSystemProxy() // calls = 2

	if calls != 2 {
		t.Errorf("systemProxyDetector calls = %d, want 2 (TTL 过期应重新探测)", calls)
	}
}

// TestDetectSystemProxy_EnvTakesPriority 验证环境变量优先且不触发 OS 探测。
func TestDetectSystemProxy_EnvTakesPriority(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://env-proxy:8080")
	old := systemProxyDetector
	systemProxyDetector = func() string {
		t.Error("systemProxyDetector should not be called when env var is set")
		return ""
	}
	defer func() { systemProxyDetector = old }()

	if ep := DetectSystemProxy(); ep != "http://env-proxy:8080" {
		t.Errorf("got %q, want env proxy", ep)
	}
}
