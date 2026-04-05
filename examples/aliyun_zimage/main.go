package main

import (
	"context"
	"fmt"
	"log"
	"time"

	aigo "github.com/godeps/aigo"
	"github.com/godeps/aigo/pkg/engine/aliyun"
)

func main() {
	client := aigo.NewClient()
	err := client.RegisterEngine("aliyun-zimage", aliyun.New(aliyun.Config{
		Model: aliyun.ModelZImageTurbo,
	}))
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	url, err := client.ExecuteTask(ctx, "aliyun-zimage", aigo.AgentTask{
		Prompt: "film grain, cinematic lighting, a neon-soaked alley after rain, detailed storefront reflections",
		Size:   "1120*1440",
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(url)
}
