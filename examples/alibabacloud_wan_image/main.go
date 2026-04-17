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
	err := client.RegisterEngine("alibabacloud-wan-image", alibabacloud.New(alibabacloud.Config{
		Model: alibabacloud.ModelWanImage,
	}))
	if err != nil {
		log.Fatal(err)
	}

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "一间清晨刚开门的街角花店，门口摆满鲜花，暖色调，写实摄影风格"}},
		"2": {ClassType: "ImageOptions", Inputs: map[string]any{"size": "2K", "watermark": false}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	url, err := client.Execute(ctx, "alibabacloud-wan-image", graph)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(url)
}
