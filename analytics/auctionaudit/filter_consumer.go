package auctionaudit

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/golang/glog"
	"github.com/prebid/prebid-server/v3/config"
	"github.com/prebid/prebid-server/v3/metrics"
	"github.com/prebid/prebid-server/v3/util/uuidutil"
	"google.golang.org/protobuf/proto"
)

const (
	maxConsumeRetries = 5
	consumeRetryDelay = 5 * time.Second
)

const (
	FilterActionCreate byte = 0
	FilterActionRemove byte = 1
)

type FilterConsumer struct {
	ctx           context.Context
	consumer      sarama.ConsumerGroup
	topic         string
	handler       *filterConsumerHandler
	metricsEngine metrics.MetricsEngine
}

type filterConsumerHandler struct {
	registry      *FilterRegistry
	metricsEngine metrics.MetricsEngine
}

func NewFilterConsumer(ctx context.Context, cfg config.AuctionAuditAnalytics, registry *FilterRegistry, metricsEngine metrics.MetricsEngine) (*FilterConsumer, error) {
	saramaConfig := sarama.NewConfig()
	saramaConfig.Consumer.Offsets.Initial = sarama.OffsetNewest

	saramaConfig.Metadata.Retry.Max = 3
	saramaConfig.Metadata.Retry.Backoff = 500 * time.Millisecond
	saramaConfig.Net.DialTimeout = 5 * time.Second

	if cfg.SASL.Enabled {
		configureSASL(saramaConfig, cfg.SASL)
	}

	// fan out, so each instance is a consumer group
	uuidGen := uuidutil.UUIDRandomGenerator{}
	id, err := uuidGen.Generate()
	if err != nil {
		return nil, fmt.Errorf("failed to generate consumer group ID: %w", err)
	}
	groupID := fmt.Sprintf("auction-audit-filters-%s", id)

	consumer, err := sarama.NewConsumerGroup(cfg.Brokers, groupID, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer group: %w", err)
	}

	fc := &FilterConsumer{
		ctx:           ctx,
		consumer:      consumer,
		topic:         cfg.FilterTopic,
		handler:       &filterConsumerHandler{registry: registry, metricsEngine: metricsEngine},
		metricsEngine: metricsEngine,
	}

	go fc.consumeLoop()

	return fc, nil
}

func (fc *FilterConsumer) Close() error {
	return fc.consumer.Close()
}

func (fc *FilterConsumer) consumeLoop() {
	consecutiveFailures := 0

	for {
		err := fc.consumer.Consume(fc.ctx, []string{fc.topic}, fc.handler)
		if err != nil {
			if errors.Is(err, sarama.ErrClosedConsumerGroup) {
				return
			}

			// This may be overkill and we may want to just loop indefinitely. We'll see how common it is.
			consecutiveFailures++
			glog.Errorf("[auctionaudit] Filter consumer error (%d/%d): %v", consecutiveFailures, maxConsumeRetries, err)
			fc.metricsEngine.RecordAuctionAuditError(metrics.AuctionAuditErrorConnection)

			if consecutiveFailures >= maxConsumeRetries {
				glog.Errorf("[auctionaudit] Filter consumer giving up after %d consecutive failures", maxConsumeRetries)
				return
			}

			time.Sleep(consumeRetryDelay)
			continue
		}

		if fc.ctx.Err() != nil {
			return
		}

		consecutiveFailures = 0
	}
}

func (h *filterConsumerHandler) Setup(sarama.ConsumerGroupSession) error {
	glog.Info("[auctionaudit] Filter consumer session started")
	return nil
}

func (h *filterConsumerHandler) Cleanup(sarama.ConsumerGroupSession) error {
	glog.Info("[auctionaudit] Filter consumer session ended")
	return nil
}

func (h *filterConsumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case msg, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			h.processMessage(msg)
			session.MarkMessage(msg, "")
		case <-session.Context().Done():
			return nil
		}
	}
}

func (h *filterConsumerHandler) processMessage(msg *sarama.ConsumerMessage) {
	filter := &AuctionFilterRequest{}
	if err := proto.Unmarshal(msg.Value, filter); err != nil {
		glog.Errorf("[auctionaudit] Failed to unmarshal filter message: %v", err)
		h.metricsEngine.RecordAuctionAuditError(metrics.AuctionAuditErrorConsume)
		return
	}

	action := FilterActionCreate
	if len(msg.Key) > 0 {
		action = msg.Key[0]
	}

	switch action {
	case FilterActionRemove:
		h.registry.Unregister(filter.SessionId, filter.AccountId)
		glog.Infof("[auctionaudit] Unregistered filter: session=%d account=%s", filter.SessionId, filter.AccountId)
	default:
		// Default to create
		if h.registry.Register(filter) {
			glog.Infof("[auctionaudit] Registered filter: session=%d account=%s", filter.SessionId, filter.AccountId)
		} else {
			glog.Warningf("[auctionaudit] Failed to register filter: session=%d account=%s", filter.SessionId, filter.AccountId)
		}
	}
}
