package hailuo

import "github.com/godeps/aigo/engine"

// ModelInfos returns i18n metadata for all Hailuo models.
func ModelInfos() []engine.ModelInfo {
	return []engine.ModelInfo{
		{
			Name:        ModelT2V01,
			Provider:    "hailuo",
			DisplayName: engine.DisplayName{"en": "Hailuo T2V-01", "zh": "海螺文生视频 01"},
			Description: engine.DisplayName{"en": "Text-to-video generation", "zh": "文本生成视频"},
			Intro:       engine.DisplayName{"en": "Hailuo T2V-01 generates high-fidelity video from text prompts with smooth motion dynamics and realistic scene rendering, suitable for creative storytelling and marketing content.", "zh": "海螺文生视频 01 从文字提示生成高保真视频，具备流畅动作动态和真实场景渲染，适用于创意叙事和营销内容。"},
			DocURL:      "https://intl.minimaxi.com/document/video-generation",
			Capability:  "video",
		},
		{
			Name:        ModelI2V01,
			Provider:    "hailuo",
			DisplayName: engine.DisplayName{"en": "Hailuo I2V-01", "zh": "海螺图生视频 01"},
			Description: engine.DisplayName{"en": "Image-to-video generation", "zh": "图片生成视频"},
			Intro:       engine.DisplayName{"en": "Hailuo I2V-01 animates still images into coherent video sequences with natural motion synthesis, preserving the original image's style and subject while adding lifelike movement.", "zh": "海螺图生视频 01 将静态图片动画化为连贯视频序列，通过自然动作合成在保留原图风格和主体的同时添加真实运动。"},
			DocURL:      "https://intl.minimaxi.com/document/video-generation",
			Capability:  "video",
		},
		{
			Name:        ModelS2V01,
			Provider:    "hailuo",
			DisplayName: engine.DisplayName{"en": "Hailuo S2V-01", "zh": "海螺主体视频 01"},
			Description: engine.DisplayName{"en": "Subject-driven video generation", "zh": "主体驱动视频生成"},
			Intro:       engine.DisplayName{"en": "Hailuo S2V-01 specializes in subject-consistent video generation, maintaining character or object identity across frames for use cases like product showcases and character animations.", "zh": "海螺主体视频 01 专注于主体一致的视频生成，跨帧保持角色或物体身份，适用于产品展示和角色动画等场景。"},
			DocURL:      "https://intl.minimaxi.com/document/video-generation",
			Capability:  "video",
		},
		{
			Name:        ModelT2V01Director,
			Provider:    "hailuo",
			DisplayName: engine.DisplayName{"en": "Hailuo T2V-01 Director", "zh": "海螺文生视频导演版"},
			Description: engine.DisplayName{"en": "Director-mode text-to-video", "zh": "导演模式文生视频"},
			Intro:       engine.DisplayName{"en": "Hailuo T2V-01 Director extends text-to-video with cinematic camera control instructions, enabling users to specify shot types, camera movements, and scene transitions for professional video production.", "zh": "海螺文生视频导演版在文生视频基础上增加影视摄像控制指令，允许用户指定镜头类型、摄像机运动和场景转换，用于专业视频制作。"},
			DocURL:      "https://intl.minimaxi.com/document/video-generation",
			Capability:  "video",
		},
	}
}
