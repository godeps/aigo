package aigo

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/godeps/aigo/workflow"
)

// OutputKind 表示引擎返回字符串的启发式分类（Execute 仍返回原始字符串，本类型供调用方解析）。
type OutputKind int

const (
	OutputUnknown OutputKind = iota
	OutputURL
	OutputDataURI
	OutputJSON
	OutputPlainText
)

// OutputHint 包含原始输出与 InterpretOutputKind 的推断类别。
type OutputHint struct {
	Kind OutputKind
	Raw  string
}

// InterpretOutputKind 根据内容前缀与 JSON 合法性做轻量分类；任务 id、纯数字等会落在 OutputPlainText。
func InterpretOutputKind(s string) OutputKind {
	s = strings.TrimSpace(s)
	if s == "" {
		return OutputUnknown
	}
	if strings.HasPrefix(s, "data:") {
		return OutputDataURI
	}
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return OutputURL
	}
	var tmp any
	if json.Unmarshal([]byte(s), &tmp) == nil {
		return OutputJSON
	}
	return OutputPlainText
}

// ExecuteWithHint 等价于 Execute，并附带 InterpretOutputKind 结果。
func (c *Client) ExecuteWithHint(ctx context.Context, engineName string, graph workflow.Graph) (OutputHint, error) {
	raw, err := c.Execute(ctx, engineName, graph)
	if err != nil {
		return OutputHint{}, err
	}
	return OutputHint{Kind: InterpretOutputKind(raw), Raw: raw}, nil
}
