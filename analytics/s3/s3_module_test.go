package s3

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/benbjohnson/clock"
	"github.com/prebid/prebid-server/v3/analytics"
	"github.com/prebid/prebid-server/v3/config"
	metricsConfig "github.com/prebid/prebid-server/v3/metrics/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockS3Client struct {
	mu       sync.Mutex
	calls    []mockS3Call
	errCount int
	err      error
}

type mockS3Call struct {
	bucket string
	key    string
	body   []byte
}

func (m *mockS3Client) PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.errCount > 0 {
		m.errCount--
		return nil, m.err
	}

	body := make([]byte, 0)
	if params.Body != nil {
		buf := make([]byte, 1024)
		for {
			n, err := params.Body.Read(buf)
			if n > 0 {
				body = append(body, buf[:n]...)
			}
			if err != nil {
				break
			}
		}
	}

	m.calls = append(m.calls, mockS3Call{
		bucket: *params.Bucket,
		key:    *params.Key,
		body:   body,
	})

	return &s3.PutObjectOutput{}, nil
}

func (m *mockS3Client) getCalls() []mockS3Call {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]mockS3Call{}, m.calls...)
}

func TestNewModule_ValidConfig(t *testing.T) {
	cfg := config.S3Analytics{
		Enabled:       true,
		Bucket:        "test-bucket",
		Prefix:        "test-prefix",
		UploadTimeout: "30s",
		Buffers: config.S3AnalyticsBuffer{
			BufferSize: "10MB",
			Timeout:    "1m",
		},
	}

	client := &mockS3Client{}
	clk := clock.NewMock()

	module, err := NewModule(cfg, client, clk, &metricsConfig.NilMetricsEngine{})

	assert.NoError(t, err)
	assert.NotNil(t, module)

	s3Module := module.(*S3Module)
	assert.NotNil(t, s3Module.auctionLogger)
	assert.NotNil(t, s3Module.ampLogger)
	assert.NotNil(t, s3Module.videoLogger)
}

func TestNewModule_InvalidConfig(t *testing.T) {
	tests := []struct {
		name   string
		cfg    config.S3Analytics
		errMsg string
	}{
		{
			name: "missing bucket",
			cfg: config.S3Analytics{
				Enabled: true,
				Prefix:  "test-prefix",
			},
			errMsg: "bucket is required",
		},
		{
			name: "missing prefix",
			cfg: config.S3Analytics{
				Enabled: true,
				Bucket:  "test-bucket",
			},
			errMsg: "prefix is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &mockS3Client{}
			clk := clock.NewMock()
			metricsEngine := &metricsConfig.NilMetricsEngine{}

			module, err := NewModule(tt.cfg, client, clk, metricsEngine)

			assert.Error(t, err)
			assert.Nil(t, module)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func TestLogAuctionObject_FlushOnSizeThreshold(t *testing.T) {
	cfg := config.S3Analytics{
		Enabled:       true,
		Bucket:        "test-bucket",
		Prefix:        "test-prefix",
		UploadTimeout: "30s",
		Buffers: config.S3AnalyticsBuffer{
			BufferSize: "100",
			Timeout:    "1h",
		},
	}

	client := &mockS3Client{}
	clk := clock.NewMock()

	module, err := NewModule(cfg, client, clk, &metricsConfig.NilMetricsEngine{})
	require.NoError(t, err)

	s3Module := module.(*S3Module)

	// Log multiple objects to exceed size threshold
	for i := 0; i < 2; i++ {
		ao := &analytics.AuctionObject{
			Status: 200,
		}
		s3Module.LogAuctionObject(ao)
	}

	// Wait for async processing to complete
	time.Sleep(100 * time.Millisecond)

	// Should have flushed due to size threshold
	calls := client.getCalls()
	assert.Greater(t, len(calls), 0, "should have flushed due to size threshold")
}

func TestLogAmpObject(t *testing.T) {
	cfg := config.S3Analytics{
		Enabled:       true,
		Bucket:        "test-bucket",
		Prefix:        "test-prefix",
		UploadTimeout: "30s",
		Buffers: config.S3AnalyticsBuffer{
			BufferSize: "100",
			Timeout:    "1m",
		},
	}

	client := &mockS3Client{}
	clk := clock.NewMock()

	module, err := NewModule(cfg, client, clk, &metricsConfig.NilMetricsEngine{})
	require.NoError(t, err)

	s3Module := module.(*S3Module)

	ao := &analytics.AmpObject{
		Status: 200,
	}

	s3Module.LogAmpObject(ao)

	clk.Add(2 * time.Minute)

	calls := client.getCalls()
	assert.Len(t, calls, 1)
	assert.Contains(t, calls[0].key, "type=amp")
}

func TestLogVideoObject(t *testing.T) {
	cfg := config.S3Analytics{
		Enabled:       true,
		Bucket:        "test-bucket",
		Prefix:        "test-prefix",
		UploadTimeout: "30s",
		Buffers: config.S3AnalyticsBuffer{
			BufferSize: "100",
			Timeout:    "1m",
		},
	}

	client := &mockS3Client{}
	clk := clock.NewMock()

	module, err := NewModule(cfg, client, clk, &metricsConfig.NilMetricsEngine{})
	require.NoError(t, err)

	s3Module := module.(*S3Module)

	vo := &analytics.VideoObject{
		Status: 200,
	}

	s3Module.LogVideoObject(vo)

	clk.Add(2 * time.Minute)

	calls := client.getCalls()
	assert.Len(t, calls, 1)
	assert.Contains(t, calls[0].key, "type=video")
}

func TestLogAuctionObject(t *testing.T) {
	cfg := config.S3Analytics{
		Enabled:       true,
		Bucket:        "test-bucket",
		Prefix:        "test-prefix",
		UploadTimeout: "30s",
		Buffers: config.S3AnalyticsBuffer{
			BufferSize: "100",
			Timeout:    "1m",
		},
	}

	client := &mockS3Client{}
	clk := clock.NewMock()

	module, err := NewModule(cfg, client, clk, &metricsConfig.NilMetricsEngine{})
	require.NoError(t, err)

	s3Module := module.(*S3Module)

	ao := &analytics.AuctionObject{
		Status: 200,
	}

	s3Module.LogAuctionObject(ao)

	clk.Add(2 * time.Minute)

	calls := client.getCalls()
	assert.Len(t, calls, 1)
	assert.Contains(t, calls[0].key, "type=auction")
}

func TestShutdownFlushing(t *testing.T) {
	cfg := config.S3Analytics{
		Enabled:       true,
		Bucket:        "test-bucket",
		Prefix:        "test-prefix",
		UploadTimeout: "30s",
		Buffers: config.S3AnalyticsBuffer{
			BufferSize: "10MB",
			Timeout:    "1m",
		},
	}

	client := &mockS3Client{}
	clk := clock.NewMock()

	module, err := NewModule(cfg, client, clk, &metricsConfig.NilMetricsEngine{})
	require.NoError(t, err)

	s3Module := module.(*S3Module)

	ao := &analytics.AuctionObject{
		Status: 200,
	}
	s3Module.LogAuctionObject(ao)

	ampObj := &analytics.AmpObject{
		Status: 200,
	}
	s3Module.LogAmpObject(ampObj)

	vo := &analytics.VideoObject{
		Status: 200,
	}
	s3Module.LogVideoObject(vo)

	s3Module.Shutdown()

	// Wait for async uploads to complete (flush spawns goroutines for uploads)
	time.Sleep(200 * time.Millisecond)

	// Should have 3 uploads (one for each event type)
	calls := client.getCalls()
	assert.Len(t, calls, 3, "shutdown should flush all 3 event types")

	eventTypes := make(map[string]bool)
	for _, call := range calls {
		if strings.Contains(call.key, "type=auction") {
			eventTypes["auction"] = true
		}
		if strings.Contains(call.key, "type=amp") {
			eventTypes["amp"] = true
		}
		if strings.Contains(call.key, "type=video") {
			eventTypes["video"] = true
		}
	}

	assert.True(t, eventTypes["auction"], "auction events should be flushed")
	assert.True(t, eventTypes["amp"], "amp events should be flushed")
	assert.True(t, eventTypes["video"], "video events should be flushed")
}

func TestEmptyBufferNoUpload(t *testing.T) {
	cfg := config.S3Analytics{
		Enabled:       true,
		Bucket:        "test-bucket",
		Prefix:        "test-prefix",
		UploadTimeout: "30s",
		Buffers: config.S3AnalyticsBuffer{
			BufferSize: "10MB",
			Timeout:    "1m",
		},
	}

	client := &mockS3Client{}
	clk := clock.NewMock()

	_, err := NewModule(cfg, client, clk, &metricsConfig.NilMetricsEngine{})
	require.NoError(t, err)

	// Don't log anything, just trigger flush with time
	clk.Add(2 * time.Minute)

	calls := client.getCalls()
	assert.Len(t, calls, 0)
}
