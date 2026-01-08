package auctionaudit

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/golang/glog"
	"github.com/prebid/prebid-server/v3/config"
	"github.com/prebid/prebid-server/v3/metrics"
)

type Producer struct {
	producer      sarama.AsyncProducer
	topic         string
	metricsEngine metrics.MetricsEngine
}

func NewProducer(cfg config.AuctionAuditKafkaConfig, metricsEngine metrics.MetricsEngine) (*Producer, error) {
	saramaConfig := sarama.NewConfig()

	if cfg.FlushInterval != "" {
		flushInterval, err := time.ParseDuration(cfg.FlushInterval)
		if err != nil {
			return nil, fmt.Errorf("invalid flush_interval: %w", err)
		}
		saramaConfig.Producer.Flush.Frequency = flushInterval
	}

	// Set compression
	compression, err := parseCompression(cfg.Compression)
	if err != nil {
		return nil, err
	}
	saramaConfig.Producer.Compression = compression
	saramaConfig.Producer.Partitioner = sarama.NewManualPartitioner
	saramaConfig.Producer.RequiredAcks = sarama.NoResponse
	saramaConfig.Producer.Return.Errors = true

	if cfg.SASL.Enabled {
		configureSASL(saramaConfig, cfg.SASL)
	}

	asyncProducer, err := sarama.NewAsyncProducer(cfg.Brokers, saramaConfig)
	if err != nil {
		return nil, err
	}

	p := &Producer{
		producer:      asyncProducer,
		topic:         cfg.MatchedTopic,
		metricsEngine: metricsEngine,
	}

	go p.consumeErrors()

	return p, nil
}

func (p *Producer) SendMatchedEvent(event *AuctionEvent, filters []*AuctionFilterRequest) error {
	if event == nil || len(filters) == 0 {
		return nil
	}

	data, err := serializeToProtobuf(event)
	if err != nil {
		return fmt.Errorf("failed to serialize event: %w", err)
	}

	// Send to each matching filter's partition with session ID as key
	for _, filter := range filters {
		keyBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(keyBytes, uint32(filter.SessionId))

		msg := &sarama.ProducerMessage{
			Topic:     p.topic,
			Partition: filter.PartitionId,
			Key:       sarama.ByteEncoder(keyBytes),
			Value:     sarama.ByteEncoder(data),
			Timestamp: time.UnixMilli(event.TimestampMs),
		}

		p.producer.Input() <- msg
	}

	return nil
}

func (p *Producer) consumeErrors() {
	for err := range p.producer.Errors() {
		glog.Errorf("[auctionaudit] Producer error: %v", err)
		p.metricsEngine.RecordAuctionAuditError(metrics.AuctionAuditErrorProduce)
	}
}

func (p *Producer) Close() error {
	return p.producer.Close()
}

func parseCompression(compression string) (sarama.CompressionCodec, error) {
	switch compression {
	case "", "none":
		return sarama.CompressionNone, nil
	case "snappy":
		return sarama.CompressionSnappy, nil
	case "gzip":
		return sarama.CompressionGZIP, nil
	case "lz4":
		return sarama.CompressionLZ4, nil
	case "zstd":
		return sarama.CompressionZSTD, nil
	default:
		return sarama.CompressionNone, fmt.Errorf("invalid compression: %s (valid: none, snappy, gzip, lz4, zstd)", compression)
	}
}
