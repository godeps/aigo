package aigo

import "github.com/godeps/aigo/engine"

// OutputKind is an alias for engine.OutputKind.
type OutputKind = engine.OutputKind

const (
	OutputUnknown   = engine.OutputUnknown
	OutputURL       = engine.OutputURL
	OutputDataURI   = engine.OutputDataURI
	OutputJSON      = engine.OutputJSON
	OutputPlainText = engine.OutputPlainText
)

// InterpretOutputKind 根据内容前缀做轻量分类；任务 id、纯数字等会落在 OutputPlainText。
func InterpretOutputKind(s string) OutputKind {
	return engine.ClassifyOutput(s)
}
