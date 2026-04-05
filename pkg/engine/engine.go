package engine

import (
	"context"

	"github.com/godeps/aigo/pkg/workflow"
)

// Engine executes a workflow graph against a concrete backend.
type Engine interface {
	Execute(ctx context.Context, graph workflow.Graph) (string, error)
}
