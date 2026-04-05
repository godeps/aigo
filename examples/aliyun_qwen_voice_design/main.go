package main

import (
	"context"
	"fmt"
	"log"
	"time"

	aigo "github.com/godeps/aigo"
	"github.com/godeps/aigo/engine/aliyun"
)

func main() {
	client := aigo.NewClient()
	if err := client.RegisterEngine("vd", aliyun.New(aliyun.Config{
		Model: aliyun.ModelQwenVoiceDesign,
	})); err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// 返回 JSON：voice、target_model；默认含 preview_audio data URI。可设 OmitVoiceDesignPreview: true 省略大段 base64。
	out, err := client.ExecuteTask(ctx, "vd", aigo.AgentTask{
		VoicePrompt:               "沉稳的中年男性新闻播报风格，语速平稳，吐字清晰。",
		PreviewText:               "各位听众晚上好，欢迎收听本期节目。",
		TargetModel:               "qwen3-tts-vd-2026-01-26",
		VoiceDesignPreferredName:  "demo_news",
		VoiceDesignLanguage:       "zh",
		VoiceDesignSampleRate:     24000,
		VoiceDesignResponseFormat: "wav",
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(out)
}
