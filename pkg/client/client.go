package client

import (
	"time"

	"resty.dev/v3"
	"websearch/pkg/config"
)

// DefaultTimeout 上游 API 请求默认超时时间（30s）。
// 覆盖单次 API RTT；apipool 多 SK 最坏约 N*30s，可接受。
const DefaultTimeout = 30 * time.Second

// New 创建带超时的 resty 客户端。timeout 可注入（测试用 200ms 等短值）。
func New(timeout time.Duration) *resty.Client {
	return resty.New().SetTimeout(timeout)
}

// DefaultClient 全局共享客户端，所有 API 适配器（baidu/tavily/exa/llm 等）使用。
// 超时来自配置 upstream_timeout_sec：默认 30s，显式 0 = 不设超时（有挂起风险）。
var DefaultClient = newDefaultClient()

// newDefaultClient 按配置初始化全局客户端。
// 配置加载失败时回退默认 30s。
func newDefaultClient() *resty.Client {
	conf, err := config.LoadOrDefault("")
	if err != nil {
		return New(DefaultTimeout)
	}
	if conf.UpstreamTimeoutSec <= 0 {
		return resty.New() // 显式 0 = 不设超时
	}
	return New(time.Duration(conf.UpstreamTimeoutSec) * time.Second)
}
