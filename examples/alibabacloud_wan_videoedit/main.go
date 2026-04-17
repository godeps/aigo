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
	err := client.RegisterEngine("alibabacloud-wan-videoedit", alibabacloud.New(alibabacloud.Config{
		Model:             alibabacloud.ModelWanVideoEdit,
		WaitForCompletion: true,
		PollInterval:      15 * time.Second,
	}))
	if err != nil {
		log.Fatal(err)
	}

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "将视频中女孩的衣服替换为参考图中的服装风格，并保持动作自然"}},
		"2": {ClassType: "LoadVideo", Inputs: map[string]any{"url": "https://help-static-aliyun-doc.aliyuncs.com/file-manage-files/zh-CN/20260403/nlspwm/T2VA_22.mp4"}},
		"3": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://help-static-aliyun-doc.aliyuncs.com/file-manage-files/zh-CN/20260402/fwjpqf/wan2.7-videoedit-change-clothes.png"}},
		"4": {ClassType: "VideoOptions", Inputs: map[string]any{"resolution": "720P", "prompt_extend": true, "watermark": true}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	url, err := client.Execute(ctx, "alibabacloud-wan-videoedit", graph)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(url)
}
