// Package embed defines the EmbedEngine interface for vector embedding backends.
//
// Unlike the generation-oriented engine.Engine (workflow.Graph → Result),
// EmbedEngine converts raw content (text, image bytes, video bytes) into
// dense float32 vectors suitable for similarity search.
package embed

import "context"

// ContentType identifies the kind of content being embedded.
type ContentType int

const (
	ContentText  ContentType = iota // plain text
	ContentImage                    // image bytes (JPEG, PNG, WebP)
	ContentVideo                    // video bytes (MP4, MOV)
)

// Request describes what to embed.
type Request struct {
	// Content holds the raw data: a string for text, or []byte for image/video.
	Content any
	// Type classifies the content.
	Type ContentType
	// TaskType hints how the embedding will be used (e.g., "RETRIEVAL_DOCUMENT",
	// "RETRIEVAL_QUERY"). Backends that support task-type-aware embedding
	// (Gemini, Voyage) use this; others ignore it.
	TaskType string
}

// TextRequest is a convenience constructor for text embedding.
func TextRequest(text string, taskType string) Request {
	return Request{Content: text, Type: ContentText, TaskType: taskType}
}

// ImageRequest is a convenience constructor for image embedding.
func ImageRequest(data []byte, taskType string) Request {
	return Request{Content: data, Type: ContentImage, TaskType: taskType}
}

// VideoRequest is a convenience constructor for video embedding.
func VideoRequest(data []byte, taskType string) Request {
	return Request{Content: data, Type: ContentVideo, TaskType: taskType}
}

// Result is the output of an embedding operation.
type Result struct {
	// Vector is the dense embedding.
	Vector []float32
	// Dimensions is the vector length (len(Vector)).
	Dimensions int
	// Model identifies which model produced this embedding.
	Model string
}

// EmbedEngine converts content into vector embeddings.
type EmbedEngine interface {
	// Embed produces a vector for the given request.
	Embed(ctx context.Context, req Request) (Result, error)

	// EmbedBatch produces vectors for multiple requests.
	// Backends that support batch APIs can implement this efficiently;
	// the default implementation calls Embed sequentially.
	EmbedBatch(ctx context.Context, reqs []Request) ([]Result, error)

	// Dimensions returns the output vector dimensionality.
	Dimensions() int
}

// Capability describes what an embedding engine supports.
type Capability struct {
	// SupportedTypes lists content types this engine can embed.
	SupportedTypes []ContentType
	// Models lists available model identifiers.
	Models []string
	// MaxDimensions is the maximum output dimensionality.
	MaxDimensions int
	// SupportsMRL indicates Matryoshka Representation Learning support
	// (embeddings can be truncated to fewer dimensions).
	SupportsMRL bool
}

// Describer is an optional interface for engines that advertise capabilities.
type Describer interface {
	EmbedCapabilities() Capability
}
