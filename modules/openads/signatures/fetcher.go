package signatures

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/golang/glog"
)

type Signature struct {
	Envelope string `json:"envelope"`
	Source   string `json:"source"`
}

type SignatureWrapper struct {
	Name string    `json:"name"`
	SIS  Signature `json:"sis"`
}

type SignatureFetcher interface {
	Fetch(ctx context.Context, body []byte) ([]SignatureWrapper, error)
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
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     60 * time.Second,
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

func (f *httpFetcher) Fetch(ctx context.Context, body []byte) ([]SignatureWrapper, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", f.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := f.client.Do(req)
	elapsed := time.Since(start)

	if err != nil {
		logFetchError(ctx, err, elapsed)
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Log slow but successful requests (near-misses)
	if elapsed > 40*time.Millisecond {
		glog.Warningf("SIS fetch slow: elapsed=%v, status=%d", elapsed, resp.StatusCode)
	}

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

	var signatures []SignatureWrapper
	if err := json.Unmarshal(respBody, &signatures); err != nil {
		return nil, fmt.Errorf("invalid JSON from signature service: %w", err)
	}

	return signatures, nil
}

func logFetchError(ctx context.Context, err error, elapsed time.Duration) {
	ctxErr := ctx.Err()
	ctxStatus := "active"
	if ctxErr != nil {
		if errors.Is(ctxErr, context.DeadlineExceeded) {
			ctxStatus = "deadline_exceeded"
		} else if errors.Is(ctxErr, context.Canceled) {
			ctxStatus = "canceled"
		}
	}

	errType := fmt.Sprintf("%T", err)

	// Unwrap to get the root cause
	var netErr *net.OpError
	var rootCause string
	if errors.As(err, &netErr) {
		rootCause = fmt.Sprintf("op=%s, net=%s, addr=%v, err=%v", netErr.Op, netErr.Net, netErr.Addr, netErr.Err)
	} else {
		rootCause = err.Error()
	}

	glog.Warningf("SIS fetch failed: type=%s, elapsed=%v, ctx=%s, cause=%s",
		errType, elapsed, ctxStatus, rootCause)
}
