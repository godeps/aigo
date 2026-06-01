// Package vidgen 实现阿里云百炼「视频生成 / 编辑」类能力（Wan 系列异步接口）。
package vidgen

import (
	"github.com/godeps/aigo/engine/alibabacloud/internal/graphx"
	"github.com/godeps/aigo/workflow"
)

// BuildParameters 为 video-synthesis 构建 parameters。
//
// wan2.7 系列 API 统一使用 resolution（"720P"/"1080P"）。当 graph 中设置了
// resolution 时优先使用；仅在没有 resolution 时才回退到 size（兼容 wan2.6）。
// preferResolution=true 时（video-edit）只使用 resolution。
func BuildParameters(graph workflow.Graph, preferResolution bool) map[string]any {
	parameters := map[string]any{}

	if preferResolution {
		if resolution, ok := graphx.Resolution(graph); ok {
			parameters["resolution"] = resolution
		}
		if _, exists := parameters["resolution"]; !exists {
			if resolution, ok := graphx.DeriveResolution(graph); ok {
				parameters["resolution"] = resolution
			}
		}
	} else {
		if resolution, ok := graphx.StringOption(graph, "resolution"); ok {
			parameters["resolution"] = resolution
		} else if size, ok := graphx.StringOption(graph, "size"); ok {
			parameters["size"] = graphx.NormalizeSize(size)
		} else if size, ok := graphx.WidthHeightSize(graph); ok {
			parameters["size"] = size
		}
	}

	if duration, ok := graphx.IntOption(graph, "duration"); ok {
		parameters["duration"] = duration
	}
	if watermark, ok := graphx.BoolOption(graph, "watermark"); ok {
		parameters["watermark"] = watermark
	}
	if audio, ok := graphx.BoolOption(graph, "audio"); ok && !preferResolution {
		parameters["audio"] = audio
	}
	if shotType, ok := graphx.StringOption(graph, "shot_type"); ok && !preferResolution {
		parameters["shot_type"] = shotType
	}
	if promptExtend, ok := graphx.BoolOption(graph, "prompt_extend"); ok {
		parameters["prompt_extend"] = promptExtend
	}
	if seed, ok := graphx.IntOption(graph, "seed"); ok {
		parameters["seed"] = seed
	}
	if ratio, ok := graphx.StringOption(graph, "ratio"); ok {
		parameters["ratio"] = ratio
	}
	if preferResolution {
		if audioSetting, ok := graphx.StringOption(graph, "audio_setting"); ok {
			parameters["audio_setting"] = audioSetting
		}
	}

	if len(parameters) == 0 {
		if preferResolution {
			parameters["resolution"] = "720P"
		} else {
			parameters["size"] = "1280*720"
		}
	}

	return parameters
}
