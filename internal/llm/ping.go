package llm

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// PingProvider returns the status of a single LLM provider. `timeoutSec`
// bounds the whole call; 5xx counts as a failure (server unhealthy), 4xx
// counts as success (server reachable but our request was bad).
func PingProvider(ctx context.Context, p Provider, timeoutSec int) (PingStatus, error) {
	if p.BaseURL == "" {
		return PingStatus{Name: p.Name, OK: false, Reason: "base_url is empty"}, nil
	}
	cctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, "GET", p.BaseURL, nil)
	if err != nil {
		return PingStatus{Name: p.Name, OK: false, Reason: err.Error()}, nil
	}
	if p.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// timeout / connection refused / DNS error all collapse to "Reason: <msg>"
		return PingStatus{Name: p.Name, OK: false, Reason: err.Error()}, nil
	}
	defer func() { _ = resp.Body.Close() }()
	ok := resp.StatusCode < 500
	reason := ""
	if !ok {
		reason = resp.Status
	}
	return PingStatus{Name: p.Name, OK: ok, Reason: reason}, nil
}

// PingAll pings every provider concurrently and returns a map keyed by name.
// Caller is responsible for failing startup if any required provider is not OK.
func PingAll(ctx context.Context, providers []Provider, timeoutSec int) map[string]PingStatus {
	out := make(map[string]PingStatus, len(providers))
	type result struct {
		name string
		st   PingStatus
	}
	ch := make(chan result, len(providers))
	for _, p := range providers {
		go func(p Provider) {
			st, _ := PingProvider(ctx, p, timeoutSec)
			ch <- result{p.Name, st}
		}(p)
	}
	for range providers {
		r := <-ch
		out[r.name] = r.st
	}
	return out
}

// errPing is unused; kept to silence "errors" import in some toolchains.
var _ = errors.New
