package main

import (
	"context"
	"fmt"
	"log"
	"time"

	aigo "github.com/godeps/aigo"
	"github.com/godeps/aigo/engine/alibabacloud"
)

func main() {
	client := aigo.NewClient()
	err := client.RegisterEngine("alibabacloud-zimage", alibabacloud.New(alibabacloud.Config{
		Model: alibabacloud.ModelZImageTurbo,
	}))
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	url, err := client.ExecuteTask(ctx, "alibabacloud-zimage", aigo.AgentTask{
		Prompt: "film grain, cinematic lighting, a neon-soaked alley after rain, detailed storefront reflections",
		Size:   "1120*1440",
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(url)
}
