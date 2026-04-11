// Package vidgen 实现阿里云百炼「视频生成 / 编辑」类能力（Wan 系列异步接口）。
package vidgen

import (
	"github.com/godeps/aigo/engine/aliyun/internal/graphx"
	"github.com/godeps/aigo/workflow"
)

// BuildParameters 为 video-synthesis 构建 parameters。
// 文生视频 / 参考生视频使用 size；视频编辑使用 resolution（preferResolution=true）。
func BuildParameters(graph workflow.Graph, preferResolution bool) map[string]any {
	parameters := map[string]any{}

	if preferResolution {
		if resolution, ok := graphx.Resolution(graph); ok {
			parameters["resolution"] = resolution
		}
	} else {
		if size, ok := graphx.StringOption(graph, "size"); ok {
			parameters["size"] = graphx.NormalizeSize(size)
		} else if size, ok := graphx.WidthHeightSize(graph); ok {
			parameters["size"] = size
		}
	}

	if preferResolution {
		if size, exists := parameters["resolution"]; !exists {
			if resolution, ok := graphx.DeriveResolution(graph); ok {
				parameters["resolution"] = resolution
			}
		} else if _, ok := size.(string); !ok {
			delete(parameters, "resolution")
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

	if len(parameters) == 0 {
		if preferResolution {
			parameters["resolution"] = "720P"
		} else {
			parameters["size"] = "1280*720"
		}
	}

	return parameters
}
