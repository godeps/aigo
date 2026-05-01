// 阿里云百炼 Tripo 3D 资产生成示例：演示文生 3D / 单图生 3D / 多图生 3D 三种入口。
//
// 运行前需设置环境变量：
//   export DASHSCOPE_API_KEY=sk-xxxxxxx
//
// 通过 ENGINE 选择执行哪一种模式（默认 text）：
//   ENGINE=text  go run ./examples/alibabacloud_tripo_3d
//   ENGINE=image go run ./examples/alibabacloud_tripo_3d
//   ENGINE=multi IMAGE_URLS=https://your.cdn/a.png,https://your.cdn/b.png \
//                go run ./examples/alibabacloud_tripo_3d
//
// multi 模式必须通过 IMAGE_URLS 提供 2-4 张可公网访问的参考图 URL，
// 否则示例会立即报错退出（避免使用占位 URL 触发上游 422/404 浪费配额）。
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	aigo "github.com/godeps/aigo"
	"github.com/godeps/aigo/engine/alibabacloud"
	"github.com/godeps/aigo/workflow"
)

func main() {
	mode := os.Getenv("ENGINE")
	if mode == "" {
		mode = "text"
	}

	client := aigo.NewClient()

	// P1.0 适合 text / single-image 场景；多图建议用 H3.1 高精度模型。
	model := alibabacloud.ModelTripoP1
	if mode == "multi" {
		model = alibabacloud.ModelTripoH31
	}

	if err := client.RegisterEngine("tripo-3d", alibabacloud.New(alibabacloud.Config{
		Model:             model,
		WaitForCompletion: true,
		PollInterval:      5 * time.Second,
	})); err != nil {
		log.Fatal(err)
	}

	graph, label := buildGraph(mode)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	result, err := client.Execute(ctx, "tripo-3d", graph)
	if err != nil {
		log.Fatalf("[%s] execute: %v", label, err)
	}

	fmt.Printf("[%s] model=%s\n", label, model)
	fmt.Printf("[%s] glb url: %s\n", label, result.Value)
	fmt.Printf("[%s] elapsed: %s\n", label, result.Elapsed)
}

// buildGraph 根据模式构造对应的 Tripo 输入工作流图。
func buildGraph(mode string) (workflow.Graph, string) {
	switch mode {
	case "image":
		return workflow.Graph{
			"1": {ClassType: "LoadImage", Inputs: map[string]any{
				"url": "https://help-static-aliyun-doc.aliyuncs.com/file-manage-files/zh-CN/20241231/cat.png",
			}},
			// 不传 ImageOptions 即生成「无贴图」纯几何模型。
			"2": {ClassType: "ImageOptions", Inputs: map[string]any{
				"texture_quality": "standard",
			}},
		}, "single-image-to-3d"
	case "multi":
		urls := parseImageURLs(os.Getenv("IMAGE_URLS"))
		if len(urls) < 2 || len(urls) > 4 {
			log.Fatalf("multi mode requires 2-4 IMAGE_URLS (comma-separated), got %d", len(urls))
		}
		g := workflow.Graph{}
		for i, u := range urls {
			id := fmt.Sprintf("%d", i+1)
			g[id] = workflow.Node{
				ClassType: "LoadImage",
				Inputs:    map[string]any{"url": u},
			}
		}
		g[fmt.Sprintf("%d", len(urls)+1)] = workflow.Node{
			ClassType: "ImageOptions",
			Inputs: map[string]any{
				"texture_quality":  "detailed",
				"geometry_quality": "ultra", // 仅 Tripo-H3.1 支持
			},
		}
		return g, "multi-image-to-3d"
	default:
		return workflow.Graph{
			"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{
				"text": "一只可爱的橙色猫咪，写实风格，电影感细节",
			}},
			"2": {ClassType: "ImageOptions", Inputs: map[string]any{
				"texture_quality": "standard",
			}},
		}, "text-to-3d"
	}
}

// parseImageURLs 解析逗号分隔的 URL 列表，过滤空白项。
func parseImageURLs(env string) []string {
	parts := strings.Split(env, ",")
	urls := make([]string, 0, len(parts))
	for _, p := range parts {
		if u := strings.TrimSpace(p); u != "" {
			urls = append(urls, u)
		}
	}
	return urls
}
