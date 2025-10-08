package s3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/golang/glog"
	"github.com/prebid/prebid-server/v3/config"
	"github.com/prebid/prebid-server/v3/metrics"
)

// S3Client interface for testing
type S3Client interface {
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

type logSender = func(payload []byte, key string) error

func createS3Sender(s3Client S3Client, cfg config.S3Analytics, metricsEngine metrics.MetricsEngine) (logSender, error) {
	uploadTimeout, err := time.ParseDuration(cfg.UploadTimeout)
	if err != nil {
		return nil, err
	}

	return func(payload []byte, key string) error {
		ctx, cancel := context.WithTimeout(context.Background(), uploadTimeout)
		defer cancel()

		err := attemptUpload(ctx, s3Client, cfg, payload, key)
		if err == nil {
			metricsEngine.RecordS3Analytics(metrics.AnalyticsDestinationS3, metrics.S3UploadSuccess)
			return nil // Success
		}

		glog.Errorf("[s3] S3 upload failed: %v", err)

		status := metrics.S3UploadFailure
		if errors.Is(err, context.DeadlineExceeded) {
			status = metrics.S3UploadTimeout
		}
		metricsEngine.RecordS3Analytics(metrics.AnalyticsDestinationS3, status)

		// Write to fallback file if upload failed
		if cfg.FallbackDir != "" {
			if fallbackErr := writeFallbackFile(cfg.FallbackDir, key, payload); fallbackErr != nil {
				glog.Errorf("[s3] Failed to write fallback file for %s: %v", key, fallbackErr)
				metricsEngine.RecordS3Analytics(metrics.AnalyticsDestinationLocal, metrics.S3UploadFailure)
			} else {
				glog.Infof("[s3] Wrote fallback file for %s", key)
				metricsEngine.RecordS3Analytics(metrics.AnalyticsDestinationLocal, metrics.S3UploadSuccess)
			}
		}

		return fmt.Errorf("s3 upload failed: %w", err)
	}, nil
}

func attemptUpload(ctx context.Context, s3Client S3Client, cfg config.S3Analytics, payload []byte, key string) error {
	_, err := s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(cfg.Bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(payload),
		ContentType: aws.String("application/gzip"),
	})

	return err
}

func writeFallbackFile(fallbackDir, s3Key string, payload []byte) error {
	filename := strings.ReplaceAll(s3Key, "/", "_")
	filePath := filepath.Join(fallbackDir, filename)

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create fallback file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, bytes.NewReader(payload)); err != nil {
		return fmt.Errorf("failed to write to fallback file: %w", err)
	}

	return nil
}
