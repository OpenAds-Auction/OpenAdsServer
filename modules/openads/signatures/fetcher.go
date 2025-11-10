package signatures

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
)

type SignatureFetcher interface {
	Fetch(ctx context.Context, body []byte) ([]interface{}, error)
}

type httpFetcher struct {
	client *http.Client
	url    string
}

func newFetcher(cfg *Config) (SignatureFetcher, error) {
	var client *http.Client
	var fetchURL string

	switch cfg.Transport {
	case TransportUDS:
		client = &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					return (&net.Dialer{}).DialContext(ctx, "unix", cfg.BasePath)
				},
			},
		}
		fetchURL = "http://unix/" + cfg.RequestPath

	case TransportTCP:
		client = &http.Client{}
		fetchURL = cfg.BasePath + "/" + cfg.RequestPath

	default:
		return nil, fmt.Errorf("unsupported transport type: %s", cfg.Transport)
	}

	return &httpFetcher{
		client: client,
		url:    fetchURL,
	}, nil
}

func (f *httpFetcher) Fetch(ctx context.Context, body []byte) ([]interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", f.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if len(respBody) == 0 {
		return nil, fmt.Errorf("empty response body")
	}

	// currently letting any valid json through
	var signatures []interface{}
	if err := json.Unmarshal(respBody, &signatures); err != nil {
		return nil, fmt.Errorf("invalid JSON from signature service: %w", err)
	}

	return signatures, nil
}
