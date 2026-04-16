package newapi

import (
	"context"
	"fmt"

	"github.com/godeps/aigo/engine"
)

// Resume implements engine.Resumer — resumes polling a previously submitted task.
// Jimeng route is not supported because polling requires a req_key that is only
// available at submission time.
func (e *Engine) Resume(ctx context.Context, remoteID string) (engine.Result, error) {
	if e.origin == "" {
		return engine.Result{}, ErrMissingBaseURL
	}

	apiKey, err := engine.ResolveKey(e.apiKey, "NEWAPI_API_KEY")
	if err != nil {
		return engine.Result{}, err
	}

	r := e.effectiveRoute()
	var url string

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
