package engine

import (
	"encoding/json"
	"sort"
	"strings"
	"sync"
)

var (
	modelInfoMu sync.RWMutex
	modelInfos  = map[string]ModelInfo{} // key = model API name
)

// RegisterModelInfos registers i18n metadata for one or more models.
// If a model name is already registered, it is silently overwritten.
func RegisterModelInfos(infos []ModelInfo) {
	modelInfoMu.Lock()
	defer modelInfoMu.Unlock()
	for _, info := range infos {
		modelInfos[info.Name] = info
	}
}

// LookupModelInfo returns the i18n metadata for a given model name.
func LookupModelInfo(model string) (ModelInfo, bool) {
	modelInfoMu.RLock()
	defer modelInfoMu.RUnlock()
	info, ok := modelInfos[model]
	return info, ok
}

// AllModelInfos returns all registered model metadata, sorted by name.
func AllModelInfos() []ModelInfo {
	modelInfoMu.RLock()
	defer modelInfoMu.RUnlock()
	result := make([]ModelInfo, 0, len(modelInfos))
	for _, info := range modelInfos {
		result = append(result, info)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

// ModelInfosByCapability returns all registered ModelInfos matching the given capability, sorted by name.
func ModelInfosByCapability(cap string) []ModelInfo {
	modelInfoMu.RLock()
	defer modelInfoMu.RUnlock()
	var result []ModelInfo
	for _, info := range modelInfos {
		if info.Capability == cap {
			result = append(result, info)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

// ModelInfosByProvider returns all registered ModelInfos for the given provider/engine name, sorted by name.
func ModelInfosByProvider(provider string) []ModelInfo {
	modelInfoMu.RLock()
	defer modelInfoMu.RUnlock()
	var result []ModelInfo
	for _, info := range modelInfos {
		if info.Provider == provider {
			result = append(result, info)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

// SearchModelInfos returns models whose Name or DisplayName (any language) contains
// the query string (case-insensitive), sorted by name.
func SearchModelInfos(query string) []ModelInfo {
	q := strings.ToLower(query)
	modelInfoMu.RLock()
	defer modelInfoMu.RUnlock()
	var result []ModelInfo
	for _, info := range modelInfos {
		if strings.Contains(strings.ToLower(info.Name), q) {
			result = append(result, info)
			continue
		}
		for _, v := range info.DisplayName {
			if strings.Contains(strings.ToLower(v), q) {
				result = append(result, info)
				break
			}
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

// ExportModelCatalog returns the complete model catalog as JSON bytes.
func ExportModelCatalog() ([]byte, error) {
	return json.MarshalIndent(AllModelInfos(), "", "  ")
}
