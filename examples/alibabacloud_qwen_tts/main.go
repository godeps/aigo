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
	if err := client.RegisterEngine("tts", alibabacloud.New(alibabacloud.Config{
		Model: alibabacloud.ModelQwenTTSFlash,
	})); err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	url, err := client.ExecuteTask(ctx, "tts", aigo.AgentTask{
		Prompt: "你好，这是通义千问语音合成示例。",
		TTS: &aigo.TTSOptions{
			Voice:        "Cherry",
			LanguageType: "Chinese",
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(url)
}
