package main

import (
	"context"
	"fmt"
	"log"
	"time"

	aigo "github.com/godeps/aigo"
	"github.com/godeps/aigo/engine/alibabacloud"
	"github.com/godeps/aigo/workflow"
)

func main() {
	client := aigo.NewClient()
	err := client.RegisterEngine("alibabacloud-qwen-image", alibabacloud.New(alibabacloud.Config{
		Model:             alibabacloud.ModelQwenImage,
		WaitForCompletion: true,
		PollInterval:      10 * time.Second,
	}))
	if err != nil {
		log.Fatal(err)
	}

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "一只戴着飞行员护目镜的柴犬坐在复古摩托上，电影感，超清细节"}},
		"2": {ClassType: "EmptyLatentImage", Inputs: map[string]any{"width": 1664, "height": 928}},
		"3": {ClassType: "NegativePrompt", Inputs: map[string]any{"negative_prompt": "模糊，低清晰度，手部畸形，过曝"}},
		"4": {ClassType: "ImageOptions", Inputs: map[string]any{"watermark": false, "n": 1}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	url, err := client.Execute(ctx, "alibabacloud-qwen-image", graph)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(url)
}
