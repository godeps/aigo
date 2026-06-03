// Package ossmeta implements the material.Searcher interface using Alibaba Cloud
// OSS DoMetaQuery API for both scalar (basic) and semantic (vector) search.
package ossmeta

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/godeps/aigo/material"

	oss "github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
)

func init() {
	material.Register("oss", func(p material.ParsedURI) (material.Searcher, error) {
		mode := Mode(p.Mode)
		if mode == "" {
			mode = ModeSemantic
		}
		return New(Config{
			AccessKeyID:     p.APIKey,
			AccessKeySecret: p.Secret,
			SecurityToken:   p.SecurityToken,
			Region:          p.Region,
			Bucket:          p.Bucket,
			Mode:            mode,
		})
	})
}

// Mode specifies the DoMetaQuery search mode.
type Mode string

const (
	ModeBasic    Mode = "basic"
	ModeSemantic Mode = "semantic"
)

// Config configures the OSS MetaQuery searcher.
type Config struct {
	AccessKeyID     string
	AccessKeySecret string
	SecurityToken   string // optional STS token
	Region          string // e.g. "cn-hangzhou"
	Bucket          string
	Endpoint        string // optional custom endpoint
	Mode            Mode   // default: ModeSemantic
	HTTPClient      *http.Client
}

// Searcher queries OSS using DoMetaQuery for material retrieval.
type Searcher struct {
	client *oss.Client
	bucket string
	mode   Mode
	region string
}

// New creates an OSS MetaQuery searcher using the official SDK v2.
// Empty config fields fall back to environment variables:
// AccessKeyID → OSS_ACCESS_KEY_ID, AccessKeySecret → OSS_ACCESS_KEY_SECRET,
// Bucket → OSS_BUCKET, Region → OSS_REGION.
func New(cfg Config) (*Searcher, error) {
	if cfg.AccessKeyID == "" {
		cfg.AccessKeyID = os.Getenv("OSS_ACCESS_KEY_ID")
	}
	if cfg.AccessKeySecret == "" {
		cfg.AccessKeySecret = os.Getenv("OSS_ACCESS_KEY_SECRET")
	}
	if cfg.Bucket == "" {
		cfg.Bucket = os.Getenv("OSS_BUCKET")
	}
	if cfg.Region == "" {
		cfg.Region = os.Getenv("OSS_REGION")
	}

	if cfg.Bucket == "" {
		return nil, fmt.Errorf("ossmeta: bucket is required (set config or env OSS_BUCKET)")
	}
	if cfg.Region == "" {
		return nil, fmt.Errorf("ossmeta: region is required (set config or env OSS_REGION)")
	}

	mode := cfg.Mode
	if mode == "" {
		mode = ModeSemantic
	}

	var credProvider credentials.CredentialsProvider
	if cfg.SecurityToken != "" {
		credProvider = credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID, cfg.AccessKeySecret, cfg.SecurityToken)
	} else {
		credProvider = credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID, cfg.AccessKeySecret)
	}

	ossCfg := oss.LoadDefaultConfig().
		WithCredentialsProvider(credProvider).
		WithRegion(cfg.Region)

	if cfg.Endpoint != "" {
		ossCfg = ossCfg.WithEndpoint(cfg.Endpoint)
	}
	if cfg.HTTPClient != nil {
		ossCfg = ossCfg.WithHttpClient(cfg.HTTPClient)
	}

	client := oss.NewClient(ossCfg)

	return &Searcher{
		client: client,
		bucket: cfg.Bucket,
		mode:   mode,
		region: cfg.Region,
	}, nil
}

func (s *Searcher) Source() string { return "oss" }
func (s *Searcher) SupportedMediaTypes() []string {
	return []string{"image", "video", "audio", "document"}
}

// Search executes a DoMetaQuery request against the configured OSS bucket.
func (s *Searcher) Search(ctx context.Context, req material.Request) (material.Result, error) {
	mode := s.mode
	if req.FieldQuery != "" && mode == ModeSemantic {
		mode = ModeBasic
	}

	maxResults := req.MaxResults
	if maxResults <= 0 {
		maxResults = 20
	}
	if maxResults > 100 {
		maxResults = 100
	}

	var xmlBody string
	switch mode {
	case ModeBasic:
		xmlBody = buildBasicXML(req, maxResults)
	case ModeSemantic:
		xmlBody = buildSemanticXML(req, maxResults)
	default:
		return material.Result{}, fmt.Errorf("ossmeta: unsupported mode %q", mode)
	}

	respBody, err := s.doMetaQuery(ctx, string(mode), xmlBody)
	if err != nil {
		return material.Result{}, err
	}

	switch mode {
	case ModeBasic:
		return parseBasicResponse(respBody)
	case ModeSemantic:
		return parseSemanticResponse(respBody)
	}
	return material.Result{}, fmt.Errorf("ossmeta: unexpected mode %q", mode)
}

func (s *Searcher) doMetaQuery(ctx context.Context, mode, xmlBody string) ([]byte, error) {
	result, err := s.client.InvokeOperation(ctx, &oss.OperationInput{
		OpName: "DoMetaQuery",
		Method: "POST",
		Bucket: oss.Ptr(s.bucket),
		Parameters: map[string]string{
			"metaQuery": "",
			"comp":      "query",
			"mode":      mode,
		},
		Headers: map[string]string{
			"Content-Type": "application/xml",
		},
		Body: strings.NewReader(xmlBody),
	})
	if err != nil {
		return nil, fmt.Errorf("ossmeta: DoMetaQuery request failed: %w", err)
	}
	defer result.Body.Close()

	body, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("ossmeta: read response: %w", err)
	}
	if result.StatusCode < 200 || result.StatusCode >= 300 {
		return nil, fmt.Errorf("ossmeta: API error (HTTP %d): %s", result.StatusCode, truncate(string(body), 300))
	}
	return body, nil
}

func buildBasicXML(req material.Request, maxResults int) string {
	query := req.FieldQuery
	if query == "" {
		query = fmt.Sprintf(`{"Field":"Filename","Value":"%s","Operation":"match"}`, escapeXMLValue(req.Query))
	}

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	sb.WriteString(`<MetaQuery>`)
	if req.NextToken != "" {
		sb.WriteString(`<NextToken>` + escapeXMLValue(req.NextToken) + `</NextToken>`)
	}
	sb.WriteString(`<MaxResults>` + strconv.Itoa(maxResults) + `</MaxResults>`)
	sb.WriteString(`<Query>` + escapeXMLValue(query) + `</Query>`)
	if req.Sort != "" {
		sb.WriteString(`<Sort>` + escapeXMLValue(req.Sort) + `</Sort>`)
	}
	if req.Order != "" {
		sb.WriteString(`<Order>` + escapeXMLValue(req.Order) + `</Order>`)
	}
	sb.WriteString(`</MetaQuery>`)
	return sb.String()
}

func buildSemanticXML(req material.Request, maxResults int) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	sb.WriteString(`<MetaQuery>`)
	sb.WriteString(`<MaxResults>` + strconv.Itoa(maxResults) + `</MaxResults>`)
	sb.WriteString(`<Query>` + escapeXMLValue(req.Query) + `</Query>`)

	mediaTypes := req.MediaTypes
	if len(mediaTypes) == 0 {
		mediaTypes = []string{"image"}
	}
	sb.WriteString(`<MediaTypes>`)
	for _, mt := range mediaTypes {
		sb.WriteString(`<MediaType>` + escapeXMLValue(mt) + `</MediaType>`)
	}
	sb.WriteString(`</MediaTypes>`)

	if req.SimpleQuery != "" {
		sb.WriteString(`<SimpleQuery>` + escapeXMLValue(req.SimpleQuery) + `</SimpleQuery>`)
	}
	sb.WriteString(`</MetaQuery>`)
	return sb.String()
}

// --- XML response parsing ---

type basicMetaQueryResp struct {
	XMLName   xml.Name       `xml:"MetaQuery"`
	NextToken string         `xml:"NextToken"`
	Files     basicFilesList `xml:"Files"`
}

type basicFilesList struct {
	File []basicFile `xml:"File"`
}

type basicFile struct {
	Filename         string `xml:"Filename"`
	Size             int64  `xml:"Size"`
	FileModifiedTime string `xml:"FileModifiedTime"`
	OSSObjectType    string `xml:"OSSObjectType"`
	OSSStorageClass  string `xml:"OSSStorageClass"`
	ObjectACL        string `xml:"ObjectACL"`
	ETag             string `xml:"ETag"`
	OSSCRC64         string `xml:"OSSCRC64"`
}

func parseBasicResponse(body []byte) (material.Result, error) {
	var resp basicMetaQueryResp
	if err := xml.Unmarshal(body, &resp); err != nil {
		return material.Result{}, fmt.Errorf("ossmeta: parse basic response: %w", err)
	}

	items := make([]material.Item, 0, len(resp.Files.File))
	for _, f := range resp.Files.File {
		items = append(items, material.Item{
			URI:       f.Filename,
			Filename:  f.Filename,
			Size:      f.Size,
			Source:    "oss",
			MediaType: guessMediaType(f.Filename),
			Metadata: map[string]string{
				"modified_time": f.FileModifiedTime,
				"storage_class": f.OSSStorageClass,
				"object_type":   f.OSSObjectType,
				"etag":          f.ETag,
			},
		})
	}

	return material.Result{
		Items:     items,
		Total:     len(items),
		NextToken: resp.NextToken,
		Source:    "oss",
	}, nil
}

type semanticMetaQueryResp struct {
	XMLName xml.Name          `xml:"MetaQuery"`
	Files   semanticFilesList `xml:"Files"`
}

type semanticFilesList struct {
	File []semanticFile `xml:"File"`
}

type semanticFile struct {
	URI              string          `xml:"URI"`
	Filename         string          `xml:"Filename"`
	Size             int64           `xml:"Size"`
	ObjectACL        string          `xml:"ObjectACL"`
	FileModifiedTime string          `xml:"FileModifiedTime"`
	ETag             string          `xml:"ETag"`
	OSSCRC64         string          `xml:"OSSCRC64"`
	ContentType      string          `xml:"ContentType"`
	MediaType        string          `xml:"MediaType"`
	ImageHeight      int             `xml:"ImageHeight"`
	ImageWidth       int             `xml:"ImageWidth"`
	VideoWidth       int             `xml:"VideoWidth"`
	VideoHeight      int             `xml:"VideoHeight"`
	Duration         float64         `xml:"Duration"`
	Title            string          `xml:"Title"`
	LatLong          string          `xml:"LatLong"`
	ProduceTime      string          `xml:"ProduceTime"`
	Artist           string          `xml:"Artist"`
	Insights         semanticInsight `xml:"Insights"`
}

type semanticInsight struct {
	Image insightImage `xml:"Image"`
	Video insightVideo `xml:"Video"`
}

type insightImage struct {
	Caption     string `xml:"Caption"`
	Description string `xml:"Description"`
}

type insightVideo struct {
	Caption string `xml:"Caption"`
}

func parseSemanticResponse(body []byte) (material.Result, error) {
	var resp semanticMetaQueryResp
	if err := xml.Unmarshal(body, &resp); err != nil {
		return material.Result{}, fmt.Errorf("ossmeta: parse semantic response: %w", err)
	}

	items := make([]material.Item, 0, len(resp.Files.File))
	for _, f := range resp.Files.File {
		width := f.ImageWidth
		height := f.ImageHeight
		if f.VideoWidth > 0 {
			width = f.VideoWidth
			height = f.VideoHeight
		}

		caption := f.Insights.Image.Caption
		if caption == "" {
			caption = f.Insights.Video.Caption
		}

		meta := map[string]string{}
		if f.LatLong != "" {
			meta["lat_long"] = f.LatLong
		}
		if f.ProduceTime != "" {
			meta["produce_time"] = f.ProduceTime
		}
		if f.Insights.Image.Description != "" {
			meta["description"] = f.Insights.Image.Description
		}

		items = append(items, material.Item{
			URI:         f.URI,
			Filename:    f.Filename,
			Size:        f.Size,
			MediaType:   f.MediaType,
			ContentType: f.ContentType,
			Width:       width,
			Height:      height,
			Duration:    f.Duration,
			Source:      "oss",
			Author:      f.Artist,
			Caption:     caption,
			Metadata:    meta,
		})
	}

	return material.Result{
		Items:  items,
		Total:  len(items),
		Source: "oss",
	}, nil
}

func guessMediaType(filename string) string {
	lower := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") ||
		strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".webp") ||
		strings.HasSuffix(lower, ".bmp") || strings.HasSuffix(lower, ".gif"):
		return "image"
	case strings.HasSuffix(lower, ".mp4") || strings.HasSuffix(lower, ".mov") ||
		strings.HasSuffix(lower, ".avi") || strings.HasSuffix(lower, ".mkv"):
		return "video"
	case strings.HasSuffix(lower, ".mp3") || strings.HasSuffix(lower, ".wav") ||
		strings.HasSuffix(lower, ".flac") || strings.HasSuffix(lower, ".aac"):
		return "audio"
	default:
		return "document"
	}
}

func escapeXMLValue(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// Ensure interface compliance.
var (
	_ material.Searcher  = (*Searcher)(nil)
	_ material.Describer = (*Searcher)(nil)
)
