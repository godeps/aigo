package aigo

import (
	"context"
	"fmt"
)

// PipelineStep transforms a previous result into the next task and target engine.
type PipelineStep func(prev Result) (AgentTask, string)

type pipelineEntry struct {
	engine string
	task   AgentTask
	step   PipelineStep // nil for the first entry
}

// Pipeline chains multiple engine executions where each step's input depends on the previous output.
type Pipeline struct {
	steps []pipelineEntry
}

// NewPipeline starts a pipeline with an initial engine and task.
func NewPipeline(engineName string, task AgentTask) *Pipeline {
	return &Pipeline{
		steps: []pipelineEntry{{engine: engineName, task: task}},
	}
}

// Then appends a step that transforms the previous result into the next task.
func (p *Pipeline) Then(fn PipelineStep) *Pipeline {
	p.steps = append(p.steps, pipelineEntry{step: fn})
	return p
}

// ExecutePipeline runs each step in sequence, feeding results forward.
func (c *Client) ExecutePipeline(ctx context.Context, p *Pipeline) ([]Result, error) {
	if len(p.steps) == 0 {
		return nil, nil
	}

	results := make([]Result, 0, len(p.steps))

	// First step uses the pre-defined task.
	r, err := c.ExecuteTask(ctx, p.steps[0].engine, p.steps[0].task)
	if err != nil {
		return results, fmt.Errorf("pipeline step 0 (engine %q): %w", p.steps[0].engine, err)
	}
	results = append(results, r)

	// Subsequent steps derive their task from the previous result.
	for i, s := range p.steps[1:] {
		task, engineName := s.step(r)
		r, err = c.ExecuteTask(ctx, engineName, task)
		if err != nil {
			return results, fmt.Errorf("pipeline step %d (engine %q): %w", i+1, engineName, err)
		}
		results = append(results, r)
	}

	return results, nil
}
