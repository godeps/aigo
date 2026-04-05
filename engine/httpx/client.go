// Package httpx 提供各引擎共用的 HTTP Client 默认值（超时等）。
package httpx

import (
	"net/http"
	"time"
)

// DefaultTimeout 为未显式配置 Timeout 时的默认请求超时（含连接+TLS+首包+整体）。
const DefaultTimeout = 3 * time.Minute

// OrDefault 返回可用的 *http.Client：nil 或 Timeout==0 时使用 defaultTimeout。
// 若 c 已有 Timeout>0，原样返回 c。
func OrDefault(c *http.Client, defaultTimeout time.Duration) *http.Client {
	if defaultTimeout <= 0 {
		defaultTimeout = DefaultTimeout
	}
	if c == nil {
		return &http.Client{Timeout: defaultTimeout}
	}
	if c.Timeout > 0 {
		return c
	}
	nc := *c
	nc.Timeout = defaultTimeout
	return &nc
}
