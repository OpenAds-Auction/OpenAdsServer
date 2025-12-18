package auctionaudit

import (
	"context"
	"time"

	"github.com/golang/glog"
	"github.com/prebid/prebid-server/v3/analytics"
	"github.com/prebid/prebid-server/v3/config"
	"github.com/prebid/prebid-server/v3/metrics"
)

type AuctionAuditModule struct {
	ctx            context.Context
	cancel         context.CancelFunc
	producer       *Producer
	environment    string
	filterRegistry *FilterRegistry
	filterConsumer *FilterConsumer
	metricsEngine  metrics.MetricsEngine
}

func NewModule(cfg config.AuctionAuditAnalytics, metricsEngine metrics.MetricsEngine) (analytics.Module, error) {
	cleanupInterval, err := time.ParseDuration(cfg.CleanupInterval)
	if err != nil {
		return nil, err
	}

	maxFilterTTL, err := time.ParseDuration(cfg.MaxFilterTTL)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	filterRegistry := NewFilterRegistry(cfg.MaxFilters, maxFilterTTL, metricsEngine)

	producer, err := NewProducer(cfg, metricsEngine)
	if err != nil {
		cancel()
		return nil, err
	}

	var filterConsumer *FilterConsumer
	if cfg.FilterTopic != "" {
		filterConsumer, err = NewFilterConsumer(ctx, cfg, filterRegistry, metricsEngine)
		if err != nil {
			cancel()
			producer.Close()
			return nil, err
		}
	}

	module := &AuctionAuditModule{
		ctx:            ctx,
		cancel:         cancel,
		producer:       producer,
		environment:    cfg.Environment,
		filterRegistry: filterRegistry,
		filterConsumer: filterConsumer,
		metricsEngine:  metricsEngine,
	}

	filterRegistry.Start(ctx, cleanupInterval)

	glog.Infof("[auctionaudit] Auction audit module initialized: matched_topic=%s filter_topic=%s brokers=%v",
		cfg.MatchedTopic, cfg.FilterTopic, cfg.Brokers)

	return module, nil
}

func (m *AuctionAuditModule) LogAuctionObject(ao *analytics.AuctionObject) {
	if ao == nil || ao.Account == nil || ao.RequestWrapper == nil || ao.RequestWrapper.BidRequest == nil {
		return
	}

	accountID := ao.Account.ID
	req := ao.RequestWrapper.BidRequest

	domain := ""
	appBundle := ""
	if req.Site != nil {
		domain = req.Site.Domain
	}
	if req.App != nil {
		appBundle = req.App.Bundle
	}

	mediaTypeSet := MediaTypeSetFromImps(req.Imp)
	filters := m.filterRegistry.GetMatches(accountID, domain, appBundle, mediaTypeSet)
	if len(filters) == 0 {
		return
	}

	event := buildAuctionEvent(ao, m.environment, accountID, domain, appBundle, mediaTypeSet.ToSlice())

	if err := m.producer.SendMatchedEvent(event, filters); err != nil {
		glog.Errorf("[auctionaudit] %v", err)
		m.metricsEngine.RecordAuctionAuditError(metrics.AuctionAuditErrorSend)
		return
	}

	for range filters {
		m.metricsEngine.RecordAuctionAudit(metrics.AuctionAuditEventMatched, accountID)
	}
}

func (m *AuctionAuditModule) Shutdown() {
	glog.Info("[auctionaudit] Shutdown initiated")

	m.cancel()

	if m.filterConsumer != nil {
		if err := m.filterConsumer.Close(); err != nil {
			glog.Errorf("[auctionaudit] Failed to close filter consumer: %v", err)
		}
	}

	if err := m.producer.Close(); err != nil {
		glog.Errorf("[auctionaudit] Failed to close producer: %v", err)
	}

	glog.Info("[auctionaudit] Shutdown complete")
}

func (m *AuctionAuditModule) LogAmpObject(ao *analytics.AmpObject) {}

func (m *AuctionAuditModule) LogVideoObject(vo *analytics.VideoObject) {}

func (m *AuctionAuditModule) LogSetUIDObject(so *analytics.SetUIDObject) {}

func (m *AuctionAuditModule) LogCookieSyncObject(cso *analytics.CookieSyncObject) {}

func (m *AuctionAuditModule) LogNotificationEventObject(ne *analytics.NotificationEvent) {}
