package aigo

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/workflow"
)

// OutputKind is an alias for engine.OutputKind.
type OutputKind = engine.OutputKind

const (
	OutputUnknown   = engine.OutputUnknown
	OutputURL       = engine.OutputURL
	OutputDataURI   = engine.OutputDataURI
	OutputJSON      = engine.OutputJSON
	OutputPlainText = engine.OutputPlainText
)

// OutputHint 包含原始输出与 InterpretOutputKind 的推断类别。
//
// Deprecated: Use Result instead; Execute now returns Result with Kind populated.
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
//
// Deprecated: Use Execute directly; Result now includes Kind.
func (c *Client) ExecuteWithHint(ctx context.Context, engineName string, graph workflow.Graph) (OutputHint, error) {
	r, err := c.Execute(ctx, engineName, graph)
	if err != nil {
		return OutputHint{}, err
	}
	return OutputHint{Kind: r.Kind, Raw: r.Value}, nil
}
