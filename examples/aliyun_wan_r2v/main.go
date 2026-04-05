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
	err := client.RegisterEngine("aliyun-wan-r2v", aliyun.New(aliyun.Config{
		Model:             aliyun.ModelWanReferenceVideo,
		WaitForCompletion: true,
		PollInterval:      15 * time.Second,
	}))
	if err != nil {
		log.Fatal(err)
	}

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "展示最新款智能手表的多功能性和时尚设计。第1个镜头[0-1秒] character1 在办公室里抬腕查看提醒。第2个镜头[1-2秒] 特写手表屏幕，展现健康数据。"}},
		"2": {ClassType: "LoadVideo", Inputs: map[string]any{"url": "https://cdn.wanx.aliyuncs.com/static/demo-wan26/vace.mp4"}},
		"3": {ClassType: "VideoOptions", Inputs: map[string]any{"size": "1280*720", "duration": 2, "shot_type": "multi", "watermark": false}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	url, err := client.Execute(ctx, "aliyun-wan-r2v", graph)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(url)
}
