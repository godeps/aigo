package newapi

import (
	"context"
	"fmt"
	"os"

	"github.com/godeps/aigo/engine"
)

// Resume implements engine.Resumer — resumes polling a previously submitted task.
// Jimeng route is not supported because polling requires a req_key that is only
// available at submission time.
func (e *Engine) Resume(ctx context.Context, remoteID string) (engine.Result, error) {
	if e.origin == "" {
		return engine.Result{}, ErrMissingBaseURL
	}

	apiKey := e.apiKey
	if apiKey == "" {
		apiKey = os.Getenv("NEWAPI_API_KEY")
	}
	if apiKey == "" {
		return engine.Result{}, ErrMissingAPIKey
	}

	r := e.effectiveRoute()
	var (
		url string
		err error
	)

	switch r {
	case RouteOpenAIVideoGenerations:
		url, err = e.pollVideoGET(ctx, apiKey, func(id string) string {
			return e.apiURL("/v1/video/generations/" + id)
		}, remoteID)

	case RouteKlingText2Video:
		url, err = e.pollVideoGET(ctx, apiKey, func(id string) string {
			return e.apiURL("/kling/v1/videos/text2video/" + id)
		}, remoteID)

	case RouteKlingImage2Video:
		url, err = e.pollVideoGET(ctx, apiKey, func(id string) string {
			return e.apiURL("/kling/v1/videos/image2video/" + id)
		}, remoteID)

	case RouteSoraVideos:
		url, err = e.pollSora(ctx, apiKey, remoteID)

	default:
		return engine.Result{}, fmt.Errorf("%s: route %q does not support resume", "newapi", r)
	}

	if err != nil {
		return engine.Result{}, err
	}
	return engine.Result{Value: url, Kind: engine.ClassifyOutput(url)}, nil
}
