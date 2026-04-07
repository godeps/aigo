package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	aigo "github.com/godeps/aigo"
	"github.com/godeps/aigo/engine/ark"
	"github.com/godeps/aigo/workflow"
)

func main() {
	apiKey := os.Getenv("ARK_API_KEY")
	if apiKey == "" {
		log.Fatal("set ARK_API_KEY")
	}
	base := os.Getenv("ARK_BASE_URL")
	if base == "" {
		base = "https://ark.cn-beijing.volces.com"
	}
	model := os.Getenv("ARK_VIDEO_MODEL")
	if model == "" {
		model = "doubao-seedance-2-0-260128"
	}

	client := aigo.NewClient()
	if err := client.RegisterEngine("seedance", ark.New(ark.Config{
		BaseURL:           base,
		Model:             model,
		APIKey:            apiKey,
		WaitForCompletion: true,
		PollInterval:      5 * time.Second,
	})); err != nil {
		log.Fatal(err)
	}

	// text-to-video
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{
			"text": "宇航员在月球上向镜头挥手，背景是蓝色地球",
		}},
		"2": {ClassType: "VideoOptions", Inputs: map[string]any{
			"duration": 5,
		}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	out, err := client.Execute(ctx, "seedance", graph)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(out)
}
