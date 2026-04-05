// Package rt 提供网关 BaseURL 规范化与路径拼接。
package rt

import "strings"

// NormalizeOrigin 将 BaseURL 规范为「仅含 scheme+host（及非 /v1 的路径前缀）」的 origin。
// 若用户配置以 /v1 结尾（常见网关写法），会剥掉该后缀，以便同时拼接 /v1/... 与 /kling/... 等路径。
func NormalizeOrigin(base string) string {
	b := strings.TrimRight(strings.TrimSpace(base), "/")
	b = strings.TrimSuffix(b, "/v1")
	return b
}

// Join 将 origin 与必须以 / 开头的 path 拼接。
func Join(origin, path string) string {
	if path == "" {
		return origin
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return origin + path
}
