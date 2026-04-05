package runtime

import (
	"net/http"
	"time"
)

// RT 为各能力子包共用的运行时参数（HTTP、轮询等）。
type RT struct {
	BaseURL           string
	HTTPClient        *http.Client
	WaitForCompletion bool
	PollInterval      time.Duration
}

// DefaultPollInterval 与根包默认轮询间隔一致。
const DefaultPollInterval = 15 * time.Second
