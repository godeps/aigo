package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	aigo "github.com/godeps/aigo"
	"github.com/godeps/aigo/engine/newapi"
	"github.com/godeps/aigo/workflow"
)

func main() {
	base := os.Getenv("NEWAPI_BASE_URL")
	if base == "" {
		log.Fatal("set NEWAPI_BASE_URL, e.g. https://your-gateway/v1")
	}
	model := os.Getenv("NEWAPI_SPEECH_MODEL")
	if model == "" {
		model = "tts-1"
	}

	client := aigo.NewClient()
	if err := client.RegisterEngine("newapi-tts", newapi.New(newapi.Config{
		BaseURL: base,
		Model:   model,
		Kind:    newapi.KindSpeech,
	})); err != nil {
		log.Fatal(err)
	}

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "你好，这是 New API 语音合成示例。"}},
		"2": {ClassType: "AudioOptions", Inputs: map[string]any{"voice": "alloy", "response_format": "mp3"}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	out, err := client.Execute(ctx, "newapi-tts", graph)
	if err != nil {
		log.Fatal(err)
	}
	if strings.HasPrefix(out.Value, "data:") {
		fmt.Println("data URI length:", len(out.Value))
	} else {
		fmt.Println(out)
	}
}
