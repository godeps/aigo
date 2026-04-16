package aigo

import (
	"context"
	"errors"
	"sort"

	"github.com/godeps/aigo/engine"
)

// InferMediaType guesses the required media type from an AgentTask's fields.
func InferMediaType(task AgentTask) string {
	if task.VoiceDesign != nil {
		return "voice_design"
	}
	if task.Music != nil {
		return "music"
	}
	if task.TTS != nil {
		return "audio"
	}

	// Check for video indicators.
	hasVideo := task.Duration > 0
	if !hasVideo {
		for _, ref := range task.References {
			if ref.Type == ReferenceTypeVideo {
				hasVideo = true
				break
			}
		}
	}
	if !hasVideo && task.Structured != nil {
		if task.Structured.VideoDuration > 0 || task.Structured.VideoSize != "" || task.Structured.VideoAspectRatio != "" {
			hasVideo = true
		}
	}
	if hasVideo {
		return "video"
	}

	return "image"
}

// RuleFilter filters candidates based on task constraints.
type RuleFilter struct{}

// Filter returns candidates compatible with the task.
// It checks media type, size, duration, and voice constraints.
func (f *RuleFilter) Filter(task AgentTask, candidates []EngineInfo) []EngineInfo {
	mediaType := InferMediaType(task)
	var result []EngineInfo

	for _, c := range candidates {
		if !f.matchMediaType(c.Capability, mediaType) {
			continue
		}
		if !f.matchDuration(c.Capability, task) {
			continue
		}
		if !f.matchVoice(c.Capability, task) {
			continue
		}
		if !f.matchSize(c.Capability, task) {
			continue
		}
		result = append(result, c)
	}
	return result
}

func (f *RuleFilter) matchMediaType(cap engine.Capability, mediaType string) bool {
	if len(cap.MediaTypes) == 0 {
		return true // no metadata → assume capable
	}
	for _, mt := range cap.MediaTypes {
		if mt == mediaType {
			return true
		}
	}
	return false
}

func (f *RuleFilter) matchDuration(cap engine.Capability, task AgentTask) bool {
	dur := task.Duration
	if task.Structured != nil && task.Structured.VideoDuration > 0 {
		dur = task.Structured.VideoDuration
	}
	if dur <= 0 || cap.MaxDuration <= 0 {
		return true
	}
	return dur <= cap.MaxDuration
}

func (f *RuleFilter) matchVoice(cap engine.Capability, task AgentTask) bool {
	if task.TTS == nil || task.TTS.Voice == "" {
		return true
	}
	if len(cap.Voices) == 0 {
		return true // no voice metadata → assume capable
	}
	for _, v := range cap.Voices {
		if v == task.TTS.Voice {
			return true
		}
	}
	return false
}

func (f *RuleFilter) matchSize(cap engine.Capability, task AgentTask) bool {
	size := task.Size
	if task.Structured != nil {
		if task.Structured.VideoSize != "" {
			size = task.Structured.VideoSize
		} else if task.Structured.ImageSize != "" {
			size = task.Structured.ImageSize
		}
	}
	if size == "" || len(cap.Sizes) == 0 {
		return true
	}
	for _, s := range cap.Sizes {
		if s == size {
			return true
		}
	}
	return false
}

// PrioritySelector picks the first compatible engine from a priority-ordered list.
// It optionally pre-filters candidates using a RuleFilter.
type PrioritySelector struct {
	Priority []string    // Engine names in preference order.
	Filter   *RuleFilter // Optional constraint filter.
}

// SelectEngine implements Selector using the priority list against flat engine names.
func (s *PrioritySelector) SelectEngine(_ context.Context, task AgentTask, engines []string) (Selection, error) {
	set := make(map[string]bool, len(engines))
	for _, e := range engines {
		set[e] = true
	}
	for _, name := range s.Priority {
		if set[name] {
			return Selection{Engine: name, Reason: "priority match"}, nil
		}
	}
	// Fall back to first available engine.
	if len(engines) > 0 {
		sorted := make([]string, len(engines))
		copy(sorted, engines)
		sort.Strings(sorted)
		return Selection{Engine: sorted[0], Reason: "fallback to first available"}, nil
	}
	return Selection{}, errors.New("aigo: no engines available")
}

// SelectEngineFromCandidates implements RichSelector with capability-aware filtering.
func (s *PrioritySelector) SelectEngineFromCandidates(_ context.Context, task AgentTask, candidates []EngineInfo) (Selection, error) {
	filtered := candidates
	if s.Filter != nil {
		filtered = s.Filter.Filter(task, candidates)
	}

	if len(filtered) == 0 {
		return Selection{}, errors.New("aigo: no compatible engines after filtering")
	}

	// Build a set of filtered names.
	available := make(map[string]bool, len(filtered))
	for _, c := range filtered {
		available[c.Name] = true
	}

	// Match against priority list.
	for _, name := range s.Priority {
		if available[name] {
			return Selection{Engine: name, Reason: "priority match (capability-filtered)"}, nil
		}
	}

	// Fall back to first filtered candidate.
	return Selection{Engine: filtered[0].Name, Reason: "fallback to first compatible"}, nil
}
