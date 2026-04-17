package main

import (
	"context"
	"fmt"
	"log"
	"time"

	aigo "github.com/godeps/aigo"
	"github.com/godeps/aigo/engine/alibabacloud"
)

func main() {
	client := aigo.NewClient()
	if err := client.RegisterEngine("vd", alibabacloud.New(alibabacloud.Config{
		Model: alibabacloud.ModelQwenVoiceDesign,
	})); err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	out, err := client.ExecuteTask(ctx, "vd", aigo.AgentTask{
		Prompt: "design a voice",
		VoiceDesign: &aigo.VoiceDesignOptions{
			VoicePrompt:    "沉稳的中年男性新闻播报风格，语速平稳，吐字清晰。",
			PreviewText:    "各位听众晚上好，欢迎收听本期节目。",
			TargetModel:    "qwen3-tts-vd-2026-01-26",
			PreferredName:  "demo_news",
			Language:       "zh",
			SampleRate:     24000,
			ResponseFormat: "wav",
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(out)
}
