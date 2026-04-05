package main

import (
	"context"
	"fmt"
	"log"
	"time"

	aigo "github.com/godeps/aigo"
	"github.com/godeps/aigo/pkg/engine/aliyun"
	"github.com/godeps/aigo/pkg/workflow"
)

func main() {
	client := aigo.NewClient()
	err := client.RegisterEngine("aliyun-wan-t2v", aliyun.New(aliyun.Config{
		Model:             aliyun.ModelWanTextToVideo,
		WaitForCompletion: true,
		PollInterval:      15 * time.Second,
	}))
	if err != nil {
		log.Fatal(err)
	}

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "黄昏时分，一艘小型蒸汽飞艇穿过金色云海，镜头缓慢推进，电影级光影"}},
		"2": {ClassType: "VideoOptions", Inputs: map[string]any{"size": "1280*720", "duration": 2, "audio": false, "watermark": false}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	url, err := client.Execute(ctx, "aliyun-wan-t2v", graph)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(url)
}
