package utils

import (
	"net/http"
	"sync/atomic"
)

var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:125.0) Gecko/20100101 Firefox/125.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:124.0) Gecko/20100101 Firefox/124.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4.1 Safari/605.1.15",
}

var userAgentIndex atomic.Uint32

// GetRandomUserAgent returns a rotating User-Agent string.
func GetRandomUserAgent() string {
	idx := userAgentIndex.Add(1)
	return userAgents[int(idx)%len(userAgents)]
}

// SetHeaders adds necessary headers to an HTTP request, including a rotating User-Agent.
func SetHeaders(req *http.Request) {
	req.Header.Set("User-Agent", GetRandomUserAgent())
}
