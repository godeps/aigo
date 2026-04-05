package main

import (
	"context"
	"fmt"
	"log"
	"os"
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
	model := os.Getenv("NEWAPI_VIDEO_MODEL")
	if model == "" {
		model = "kling-v1"
	}

	client := aigo.NewClient()
	if err := client.RegisterEngine("newapi-vid", newapi.New(newapi.Config{
		BaseURL:           base,
		Model:             model,
		Kind:              newapi.KindVideo,
		WaitForCompletion: true,
		PollInterval:      5 * time.Second,
	})); err != nil {
		log.Fatal(err)
	}

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "宇航员在月球上向镜头挥手"}},
		"2": {ClassType: "VideoOptions", Inputs: map[string]any{"duration": 5, "width": 1280, "height": 720}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	out, err := client.Execute(ctx, "newapi-vid", graph)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(out)
}
