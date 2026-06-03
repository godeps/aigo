package material

import (
	"encoding/base64"
	"encoding/json"
)

// PaginationState encodes per-backend pagination cursors for MultiSearcher.
// Serialized as base64 JSON in NextToken to allow transparent continuation.
type PaginationState map[string]string // source → next_token or page

// EncodePagination serializes pagination state into a single opaque token.
func EncodePagination(state PaginationState) string {
	if len(state) == 0 {
		return ""
	}
	data, err := json.Marshal(state)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(data)
}

// DecodePagination deserializes an opaque token back into pagination state.
// Returns empty state (not an error) if the token is empty or invalid.
func DecodePagination(token string) PaginationState {
	if token == "" {
		return nil
	}
	data, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil
	}
	var state PaginationState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil
	}
	return state
}
