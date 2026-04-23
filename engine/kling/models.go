package kling

import "github.com/godeps/aigo/engine"

// ModelInfos returns i18n metadata for all Kling models.
func ModelInfos() []engine.ModelInfo {
	return []engine.ModelInfo{
		{
			Name:        ModelKlingV2,
			Provider:    "kling",
			DisplayName: engine.DisplayName{"en": "Kling V2", "zh": "可灵 V2"},
			Description: engine.DisplayName{"en": "Video and image generation", "zh": "视频和图片生成"},
			Intro:       engine.DisplayName{"en": "Kling V2 from Kuaishou delivers high-fidelity text-to-video and image-to-video generation with realistic physics simulation and strong motion coherence for creative and commercial use.", "zh": "快手可灵 V2 提供高保真文生视频和图生视频，具备真实物理模拟和强大动作一致性，适用于创意和商业场景。"},
			DocURL:      "https://docs.qingque.cn/d/home/eZQCm3mMOoGUqbJH0MrPnVXknYg",
			Capability:  "video",
		},
		{
			Name:        ModelKlingV2Master,
			Provider:    "kling",
			DisplayName: engine.DisplayName{"en": "Kling V2 Master", "zh": "可灵 V2 大师版"},
			Description: engine.DisplayName{"en": "Highest quality video and image generation", "zh": "最高画质视频和图片生成"},
			Intro:       engine.DisplayName{"en": "Kling V2 Master is Kuaishou's flagship model offering cinematic-quality video generation with superior motion expressiveness, fine detail preservation, and extended scene complexity support.", "zh": "可灵 V2 大师版是快手旗舰模型，提供影视级视频生成，具备卓越动作表现力、精细细节保留和扩展场景复杂度支持。"},
			DocURL:      "https://docs.qingque.cn/d/home/eZQCm3mMOoGUqbJH0MrPnVXknYg",
			Capability:  "video",
		},
		{
			Name:        ModelKlingV1,
			Provider:    "kling",
			DisplayName: engine.DisplayName{"en": "Kling V1", "zh": "可灵 V1"},
			Description: engine.DisplayName{"en": "Video and image generation", "zh": "视频和图片生成"},
			Intro:       engine.DisplayName{"en": "Kling V1 is the first-generation Kuaishou video model with proven reliability for text-to-video and image-to-video tasks, offering a cost-effective option for standard creative workflows.", "zh": "可灵 V1 是快手第一代视频模型，在文生视频和图生视频任务中经过验证，为标准创意工作流提供具有成本效益的选择。"},
			DocURL:      "https://docs.qingque.cn/d/home/eZQCm3mMOoGUqbJH0MrPnVXknYg",
			Capability:  "video",
		},
	}
}
