package graph

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/godeps/aigo/workflow"
)

// ImageBytesForEdits 为 OpenAI images/edits 准备 PNG 字节（url / base64 / 本地路径）。
// allowRemoteURL 为 false 时，image_url / edit_image_url 不会发起 HTTP 请求（降低 SSRF 风险）。
func ImageBytesForEdits(g workflow.Graph, client *http.Client, allowRemoteURL bool) ([]byte, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if s, ok := StringOption(g, "image_b64", "image_base64"); ok && strings.TrimSpace(s) != "" {
		return base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	}
	if p, ok := StringOption(g, "image_path", "filename"); ok && p != "" {
		return os.ReadFile(p)
	}
	if u, ok := StringOption(g, "image_url", "edit_image_url"); ok && u != "" {
		if !allowRemoteURL {
			return nil, ErrRemoteMediaDisabled
		}
		return fetchURL(client, u)
	}
	for _, id := range g.SortedNodeIDs() {
		node := g[id]
		if strings.EqualFold(node.ClassType, "LoadImage") {
			if p, ok := node.StringInput("image"); ok && p != "" {
				return os.ReadFile(p)
			}
		}
	}
	return nil, ErrMissingImageSource
}

// AudioBytesForWhisper 为 Whisper multipart 准备音频字节与文件名。
func AudioBytesForWhisper(g workflow.Graph, client *http.Client, allowRemoteURL bool) (filename string, data []byte, err error) {
	if client == nil {
		client = http.DefaultClient
	}
	if s, ok := StringOption(g, "audio_b64"); ok && strings.TrimSpace(s) != "" {
		data, err = base64.StdEncoding.DecodeString(strings.TrimSpace(s))
		if err != nil {
			return "", nil, err
		}
		fn := "audio.bin"
		if f, ok := StringOption(g, "audio_filename"); ok {
			fn = f
		}
		return fn, data, nil
	}
	if p, ok := StringOption(g, "audio_path"); ok && p != "" {
		data, err = os.ReadFile(p)
		if err != nil {
			return "", nil, err
		}
		return baseName(p), data, nil
	}
	if u, ok := StringOption(g, "audio_url"); ok && u != "" {
		if !allowRemoteURL {
			return "", nil, ErrRemoteMediaDisabled
		}
		data, err = fetchURL(client, u)
		if err != nil {
			return "", nil, err
		}
		return "audio.bin", data, nil
	}
	return "", nil, ErrMissingAudioSource
}

func baseName(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

func fetchURL(client *http.Client, u string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("graph: fetch %s: status %s", u, resp.Status)
	}
	return io.ReadAll(resp.Body)
}
