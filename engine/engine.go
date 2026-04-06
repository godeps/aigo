package engine

import (
	"context"

	"github.com/godeps/aigo/workflow"
)

// OutputKind classifies the string returned by an engine.
type OutputKind int

const (
	OutputUnknown   OutputKind = iota
	OutputURL
	OutputDataURI
	OutputJSON
	OutputPlainText
)

// Result is the structured output of Engine.Execute.
type Result struct {
	Value string
	Kind  OutputKind
}

// Engine executes a workflow graph against a concrete backend.
type Engine interface {
	Execute(ctx context.Context, graph workflow.Graph) (Result, error)
}

// Capability describes what an engine can do.
type Capability struct {
	MediaTypes   []string // e.g. ["image", "video", "audio"]
	Models       []string
	Sizes        []string // e.g. ["1024x1024", "1280x720"]
	MaxDuration  int      // max video/audio duration in seconds; 0 = not applicable
	SupportsSync bool
	SupportsPoll bool
}

// Describer is an optional interface that engines can implement to advertise capabilities.
type Describer interface {
	Capabilities() Capability
}

// DryRunResult is the outcome of a dry-run estimation.
type DryRunResult struct {
	WillPoll      bool
	EstimatedTime string   // human-readable estimate, e.g. "30s-2m"
	Warnings      []string // potential issues with the request
}

// DryRunner is an optional interface for engines that support dry-run estimation.
type DryRunner interface {
	DryRun(graph workflow.Graph) (DryRunResult, error)
}
