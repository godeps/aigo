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
	model := os.Getenv("NEWAPI_IMAGE_MODEL")
	if model == "" {
		model = "dall-e-3"
	}

	client := aigo.NewClient()
	if err := client.RegisterEngine("newapi-img", newapi.New(newapi.Config{
		BaseURL: base,
		Model:   model,
		Kind:    newapi.KindImage,
	})); err != nil {
		log.Fatal(err)
	}

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "一只在火星上散步的机械猫，电影光"}},
		"2": {ClassType: "EmptyLatentImage", Inputs: map[string]any{"width": 1024, "height": 1024}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	out, err := client.Execute(ctx, "newapi-img", graph)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(out)
}
