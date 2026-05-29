package resolve

import "github.com/godeps/aigo/workflow"

// ClampDuration constrains a duration value within [min, max] seconds.
// If max <= 0, no upper bound is applied. If min <= 0, no lower bound is applied.
func ClampDuration(value, min, max float64) float64 {
	if min > 0 && value < min {
		return min
	}
	if max > 0 && value > max {
		return max
	}
	return value
}

// ExtractDuration extracts the video/audio duration from the workflow graph.
// Priority: VideoOptions node > global option.
// Returns 0, false if no duration is found.
func ExtractDuration(g workflow.Graph) (float64, bool) {
	// 1. VideoOptions node (float64 or int)
	for _, ref := range g.FindByClassType("VideoOptions") {
		if raw, ok := ref.Node.Input("duration"); ok {
			if d := toFloat64(raw); d > 0 {
				return d, true
			}
		}
	}

	// 2. Global float64 option
	if d, ok := Float64Option(g, "duration"); ok && d > 0 {
		return d, true
	}

	// 3. Global int option
	if d, ok := IntOption(g, "duration"); ok && d > 0 {
		return float64(d), true
	}

	return 0, false
}

func toFloat64(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case float32:
		return float64(t)
	}
	return 0
}
