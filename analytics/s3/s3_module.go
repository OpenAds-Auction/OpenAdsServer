package s3

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/docker/go-units"
	"github.com/golang/glog"
	"github.com/prebid/prebid-server/v3/analytics"
	"github.com/prebid/prebid-server/v3/config"
	"github.com/prebid/prebid-server/v3/metrics"
	"github.com/prebid/prebid-server/v3/util/uuidutil"
)

type S3Logger struct {
	sender            logSender
	eventType         string
	clock             clock.Clock
	bucket            string
	prefix            string
	environment       string
	bufferSize        int64 // tracks uncompressed bytes written
	maxBufferByteSize int64
	maxDuration       time.Duration
	mux               sync.RWMutex
	sigTermCh         chan os.Signal
	buffer            bytes.Buffer
	gzw               *gzip.Writer
	bufferCh          chan []byte
}

type S3Module struct {
	auctionLogger *S3Logger
	ampLogger     *S3Logger
	videoLogger   *S3Logger
}

func newS3Logger(cfg config.S3Analytics, sender logSender, clock clock.Clock, eventType string) (*S3Logger, error) {
	bufferSize, err := units.FromHumanSize(cfg.Buffers.BufferSize)
	if err != nil {
		return nil, fmt.Errorf("invalid buffer size: %w", err)
	}

	flushInterval, err := time.ParseDuration(cfg.Buffers.Timeout)
	if err != nil {
		return nil, fmt.Errorf("invalid flush interval: %w", err)
	}

	logger := &S3Logger{
		sender:            sender,
		eventType:         eventType,
		clock:             clock,
		bucket:            cfg.Bucket,
		prefix:            cfg.Prefix,
		environment:       cfg.Environment,
		maxBufferByteSize: bufferSize,
		maxDuration:       flushInterval,
		bufferCh:          make(chan []byte),
		sigTermCh:         make(chan os.Signal, 1),
	}

	logger.gzw = gzip.NewWriter(&logger.buffer)

	signal.Notify(logger.sigTermCh, os.Interrupt, syscall.SIGTERM)

	return logger, nil
}

func NewModule(cfg config.S3Analytics, client S3Client, clock clock.Clock, metricsEngine metrics.MetricsEngine) (analytics.Module, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("invalid S3 analytics config: %w", err)
	}

	sender, err := createS3Sender(client, cfg, metricsEngine)
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 sender: %w", err)
	}

	// Create loggers for each event type
	loggers := make([]*S3Logger, 0, 3)
	for _, eventType := range []string{"auction", "amp", "video"} {
		logger, err := newS3Logger(cfg, sender, clock, eventType)
		if err != nil {
			return nil, fmt.Errorf("failed to create %s logger: %w", eventType, err)
		}
		loggers = append(loggers, logger)
		go logger.start()
	}

	glog.Infof("[s3] S3 analytics module initialized: bucket=%s prefix=%s env=%s region=%s",
		cfg.Bucket, cfg.Prefix, cfg.Environment, cfg.Region)

	return &S3Module{
		auctionLogger: loggers[0],
		ampLogger:     loggers[1],
		videoLogger:   loggers[2],
	}, nil
}

func (l *S3Logger) start() {
	ticker := l.clock.Ticker(l.maxDuration)
	defer ticker.Stop()

	for {
		select {
		case <-l.sigTermCh:
			glog.Infof("[s3] %s logger received shutdown signal, flushing buffer", l.eventType)
			l.flush()
			return
		case event := <-l.bufferCh:
			l.bufferEvent(event)
			if l.isFull() {
				l.flush()
			}
		case <-ticker.C:
			l.flush()
		}
	}
}

func (l *S3Logger) bufferEvent(data []byte) {
	l.mux.Lock()
	defer l.mux.Unlock()

	// Write event + newline for NDJSON format
	if _, err := l.gzw.Write(data); err != nil {
		glog.Errorf("[s3] Failed to write event to %s buffer: %v", l.eventType, err)
		return
	}

	if _, err := l.gzw.Write([]byte("\n")); err != nil {
		glog.Errorf("[s3] Failed to write newline to %s buffer: %v", l.eventType, err)
		return
	}

	l.bufferSize += int64(len(data))
}

func (l *S3Logger) isFull() bool {
	l.mux.RLock()
	defer l.mux.RUnlock()
	return l.bufferSize >= l.maxBufferByteSize
}

func (l *S3Logger) flush() {
	l.mux.Lock()
	defer l.mux.Unlock()

	if l.bufferSize == 0 {
		return
	}

	// Close gzip writer to finalize compression
	if err := l.gzw.Close(); err != nil {
		glog.Errorf("[s3] Failed to close gzip writer for %s: %v", l.eventType, err)
		l.reset()
		return
	}

	// Copy buffer for async upload
	payload := make([]byte, l.buffer.Len())
	if _, err := l.buffer.Read(payload); err != nil {
		glog.Errorf("[s3] Failed to read buffer for %s: %v", l.eventType, err)
		l.reset()
		return
	}

	key := l.generateS3Key()

	// Reset buffer for next batch
	l.reset()

	// Upload asynchronously
	go func() {
		if err := l.sender(payload, key); err != nil {
			glog.Errorf("[s3] Upload failed for %s: %s: %v", l.eventType, key, err)
		} else {
			glog.Infof("[s3] Successfully uploaded %s batch: %s (%d bytes)",
				l.eventType, key, len(payload))
		}
	}()
}

func (l *S3Logger) reset() {
	l.gzw.Reset(&l.buffer)
	l.buffer.Reset()
	l.bufferSize = 0
}

func (l *S3Logger) generateS3Key() string {
	now := l.clock.Now().UTC()

	// Generate a unique UUID for this log file
	uuidGen := uuidutil.UUIDRandomGenerator{}
	uuid, err := uuidGen.Generate()
	if err != nil {
		// Fallback to timestamp-only if UUID generation fails (shouldn't happen)
		glog.Errorf("[s3] Failed to generate UUID for key, using timestamp only: %v", err)
		uuid = "00000000-0000-0000-0000-000000000000"
	}

	// Format: {prefix}/env={environment}/type={type}/date=YYYY-MM-DD/hour=HH/{timestamp}_{uuid}.jsonl.gz
	return fmt.Sprintf("%s/env=%s/type=%s/date=%s/hour=%s/%s_%s.jsonl.gz",
		l.prefix,
		l.environment,
		l.eventType,
		now.Format("2006-01-02"),
		now.Format("15"),
		strconv.FormatInt(now.Unix(), 10),
		uuid)
}

func (m *S3Module) LogAuctionObject(ao *analytics.AuctionObject) {
	if ao == nil {
		return
	}

	payload, err := serializeAuctionObject(ao)
	if err != nil {
		glog.Errorf("[s3] Failed to serialize auction object: %v", err)
		return
	}

	m.auctionLogger.bufferCh <- payload
}

func (m *S3Module) LogAmpObject(ao *analytics.AmpObject) {
	if ao == nil {
		return
	}

	payload, err := serializeAmpObject(ao)
	if err != nil {
		glog.Errorf("[s3] Failed to serialize amp object: %v", err)
		return
	}

	m.ampLogger.bufferCh <- payload
}

func (m *S3Module) LogVideoObject(vo *analytics.VideoObject) {
	if vo == nil {
		return
	}

	payload, err := serializeVideoObject(vo)
	if err != nil {
		glog.Errorf("[s3] Failed to serialize video object: %v", err)
		return
	}

	m.videoLogger.bufferCh <- payload
}

func (m *S3Module) LogSetUIDObject(so *analytics.SetUIDObject) {
	// Not tracked
}

func (m *S3Module) LogCookieSyncObject(cso *analytics.CookieSyncObject) {
	// Not tracked
}

func (m *S3Module) LogNotificationEventObject(ne *analytics.NotificationEvent) {
	// Not tracked
}

func (m *S3Module) Shutdown() {
	glog.Info("[s3] Shutdown initiated, flushing all buffers")
	m.auctionLogger.flush()
	m.ampLogger.flush()
	m.videoLogger.flush()
}

func validateConfig(cfg config.S3Analytics) error {
	if cfg.Bucket == "" {
		return fmt.Errorf("bucket is required")
	}
	if cfg.Prefix == "" {
		return fmt.Errorf("prefix is required")
	}
	return nil
}
