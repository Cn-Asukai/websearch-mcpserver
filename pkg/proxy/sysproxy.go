package proxy

import (
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf16"
	"unsafe"
)

// systemProxyDetector 平台相关的系统代理检测函数。
// 在 Windows 上由 sysproxy_windows.go 的 init() 注入。
// 非 Windows 平台保持 nil（仅依赖环境变量）。
var systemProxyDetector func() string

// systemProxyTTL 系统代理检测结果的内存缓存时长。
// 自动代理模式下避免每个请求都读注册表 / WinHTTP。
const systemProxyTTL = 10 * time.Second

// osProxyCacheEntry 系统代理检测缓存条目。
type osProxyCacheEntry struct {
	at time.Time // 检测时间
	ep string    // 检测结果
}

// osProxyCache 系统代理检测结果缓存（进程内 TTL）。
var osProxyCache atomic.Value // osProxyCacheEntry

// DetectSystemProxy 自动检测系统代理。
// 优先检查环境变量（HTTP_PROXY / HTTPS_PROXY / ALL_PROXY），
// 然后读取操作系统级代理设置（Windows 注册表 / WinHTTP），结果带 TTL 缓存。
// 返回代理端点 URL（如 "http://127.0.0.1:7897"），未检测到则返回空字符串。
func DetectSystemProxy() string {
	// 1. 环境变量（优先级最高，读取廉价，不做缓存）
	if ep := proxyFromEnv(); ep != "" {
		return ep
	}
	// 2. 操作系统代理设置（平台相关，由各平台 init() 注入），带 TTL 缓存
	if systemProxyDetector == nil {
		return ""
	}
	now := time.Now()
	if e, ok := osProxyCache.Load().(osProxyCacheEntry); ok && now.Sub(e.at) < systemProxyTTL {
		return e.ep
	}
	ep := systemProxyDetector()
	osProxyCache.Store(osProxyCacheEntry{at: now, ep: ep})
	return ep
}

// utf16PtrToString 在 buf 字节范围内从 p 指向的 UTF-16LE 字符串读取至 NUL。
// p 为 nil 或不在 buf 范围内时返回空串；无 NUL 时按 buf 边界截断，绝不越界读取。
func utf16PtrToString(p *uint16, buf []byte) string {
	if p == nil {
		return ""
	}
	start := uintptr(unsafe.Pointer(p))
	bufStart := uintptr(unsafe.Pointer(unsafe.SliceData(buf)))
	bufEnd := bufStart + uintptr(len(buf))
	if start < bufStart || start > bufEnd {
		return ""
	}
	// 从 p 到 buf 末尾可安全读取的 uint16 数量
	remaining := (bufEnd - start) / 2
	u16 := unsafe.Slice(p, remaining)
	for i, c := range u16 {
		if c == 0 {
			u16 = u16[:i]
			break
		}
	}
	return string(utf16.Decode(u16))
}

// proxyFromEnv 从 HTTP_PROXY / HTTPS_PROXY / ALL_PROXY 环境变量读取代理。
func proxyFromEnv() string {
	for _, key := range []string{"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy", "ALL_PROXY", "all_proxy"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			if u, err := url.Parse(v); err == nil && (u.Scheme == "http" || u.Scheme == "https" || u.Scheme == "socks5") {
				return v
			}
		}
	}
	return ""
}
