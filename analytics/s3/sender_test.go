package s3

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/prebid/prebid-server/v3/config"
	metricsConfig "github.com/prebid/prebid-server/v3/metrics/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testS3Client struct {
	calls       []string
	shouldError error
}

func (c *testS3Client) PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	c.calls = append(c.calls, *params.Key)
	if c.shouldError != nil {
		return nil, c.shouldError
	}
	return &s3.PutObjectOutput{}, nil
}

func TestCreateS3Sender_Success(t *testing.T) {
	client := &testS3Client{}
	cfg := config.S3Analytics{
		Bucket:        "test-bucket",
		UploadTimeout: "1s",
	}
	metricsEngine := &metricsConfig.NilMetricsEngine{}

	sender, err := createS3Sender(client, cfg, metricsEngine)
	require.NoError(t, err)

	err = sender([]byte("test payload"), "test-key.gz")
	assert.NoError(t, err)
	assert.Len(t, client.calls, 1)
	assert.Equal(t, "test-key.gz", client.calls[0])
}

func TestCreateS3Sender_UploadFails(t *testing.T) {
	client := &testS3Client{
		shouldError: errors.New("upload error"),
	}
	cfg := config.S3Analytics{
		Bucket:        "test-bucket",
		UploadTimeout: "1s",
	}
	metricsEngine := &metricsConfig.NilMetricsEngine{}

	sender, err := createS3Sender(client, cfg, metricsEngine)
	require.NoError(t, err)

	err = sender([]byte("test payload"), "test-key.gz")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "s3 upload failed")
}

func TestCreateS3Sender_TimeoutDetected(t *testing.T) {
	client := &testS3Client{
		shouldError: context.DeadlineExceeded,
	}
	cfg := config.S3Analytics{
		Bucket:        "test-bucket",
		UploadTimeout: "1s",
	}
	metricsEngine := &metricsConfig.NilMetricsEngine{}

	sender, err := createS3Sender(client, cfg, metricsEngine)
	require.NoError(t, err)

	err = sender([]byte("test payload"), "test-key.gz")
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestCreateS3Sender_FallbackSuccess(t *testing.T) {
	client := &testS3Client{
		shouldError: errors.New("s3 error"),
	}

	tmpDir := t.TempDir()
	cfg := config.S3Analytics{
		Bucket:        "test-bucket",
		UploadTimeout: "1s",
		FallbackDir:   tmpDir,
	}
	metricsEngine := &metricsConfig.NilMetricsEngine{}

	sender, err := createS3Sender(client, cfg, metricsEngine)
	require.NoError(t, err)

	testPayload := []byte("test payload data")
	err = sender(testPayload, "prefix/test-key.gz")
	assert.Error(t, err, "S3 upload should fail")

	// Check fallback file was written
	fallbackFile := filepath.Join(tmpDir, "prefix_test-key.gz")
	data, err := os.ReadFile(fallbackFile)
	require.NoError(t, err)
	assert.Equal(t, testPayload, data)
}

func TestCreateS3Sender_InvalidTimeout(t *testing.T) {
	client := &testS3Client{}
	cfg := config.S3Analytics{
		Bucket:        "test-bucket",
		UploadTimeout: "invalid",
	}
	metricsEngine := &metricsConfig.NilMetricsEngine{}

	_, err := createS3Sender(client, cfg, metricsEngine)
	assert.Error(t, err, "should fail to parse invalid timeout")
}
