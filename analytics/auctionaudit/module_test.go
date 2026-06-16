package auctionaudit

import (
	"context"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/IBM/sarama/mocks"
	"github.com/prebid/openrtb/v20/openrtb2"
	"github.com/prebid/prebid-server/v3/analytics"
	"github.com/prebid/prebid-server/v3/config"
	"github.com/prebid/prebid-server/v3/metrics"
	"github.com/prebid/prebid-server/v3/openrtb_ext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name   string
		cfg    config.AuctionAuditKafkaConfig
		errMsg string
	}{
		{
			name: "valid config",
			cfg: config.AuctionAuditKafkaConfig{
				Brokers:      []string{"mybroker"},
				FilterTopic:  "filter-topic",
				MatchedTopic: "matched-topic",
			},
			errMsg: "",
		},
		{
			name: "missing brokers",
			cfg: config.AuctionAuditKafkaConfig{
				Brokers:      []string{},
				FilterTopic:  "filter-topic",
				MatchedTopic: "matched-topic",
			},
			errMsg: "kafka.brokers is required",
		},
		{
			name: "nil brokers",
			cfg: config.AuctionAuditKafkaConfig{
				Brokers:      nil,
				FilterTopic:  "filter-topic",
				MatchedTopic: "matched-topic",
			},
			errMsg: "kafka.brokers is required",
		},
		{
			name: "missing filter_topic",
			cfg: config.AuctionAuditKafkaConfig{
				Brokers:      []string{"mybroker"},
				FilterTopic:  "",
				MatchedTopic: "matched-topic",
			},
			errMsg: "kafka.filter_topic is required",
		},
		{
			name: "missing matched_topic",
			cfg: config.AuctionAuditKafkaConfig{
				Brokers:      []string{"mybroker"},
				FilterTopic:  "filter-topic",
				MatchedTopic: "",
			},
			errMsg: "kafka.matched_topic is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.cfg)
			if tt.errMsg == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			}
		})
	}
}

func newTestModule(t *testing.T, me *metrics.MetricsEngineMock, maxEventsPerSec float64) *AuctionAuditModule {
	t.Helper()

	saramaConfig := sarama.NewConfig()
	saramaConfig.Producer.Return.Successes = false
	mockProducer := mocks.NewAsyncProducer(t, saramaConfig)

	registry := NewFilterRegistry(100, time.Hour, maxEventsPerSec, me)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	return &AuctionAuditModule{
		ctx:    ctx,
		cancel: cancel,
		producer: &Producer{
			producer:      mockProducer,
			topic:         "test-topic",
			metricsEngine: me,
		},
		environment:    "test",
		filterRegistry: registry,
		metricsEngine:  me,
	}
}

func newTestAuctionObject() *analytics.AuctionObject {
	return &analytics.AuctionObject{
		Account: &config.Account{ID: "testaccount"},
		RequestWrapper: &openrtb_ext.RequestWrapper{
			BidRequest: &openrtb2.BidRequest{
				Site: &openrtb2.Site{Domain: "example.com"},
				Imp: []openrtb2.Imp{
					{Banner: &openrtb2.Banner{Format: []openrtb2.Format{{W: 300, H: 250}}}},
				},
			},
		},
	}
}

func registerTestFilter(t *testing.T, registry *FilterRegistry) {
	t.Helper()
	err := registry.Register(&AuctionFilterRequest{
		SessionId:   1,
		PartitionId: 0,
		AccountId:   "testaccount",
		ExpiresAtMs: time.Now().Add(time.Hour).UnixMilli(),
	})
	assert.NoError(t, err)
}

func TestLogAuctionObject_RateLimiterDisabled_AllEventsPass(t *testing.T) {
	me := &metrics.MetricsEngineMock{}
	me.On("RecordAuctionAudit", mock.Anything, mock.Anything, mock.Anything).Return()
	me.On("RecordAuctionAuditActiveFilters", mock.Anything).Return()

	module := newTestModule(t, me, 0)
	module.producer.producer.(*mocks.AsyncProducer).ExpectInputAndSucceed()

	registerTestFilter(t, module.filterRegistry)

	module.LogAuctionObject(newTestAuctionObject())

	me.AssertCalled(t, "RecordAuctionAudit", metrics.AuctionAuditEventMatched, "testaccount", 1)
	me.AssertNotCalled(t, "RecordAuctionAudit", metrics.AuctionAuditEventDropped, mock.Anything, mock.Anything)
}

func TestLogAuctionObject_PerFilterRateLimit_DropsExcessEvents(t *testing.T) {
	me := &metrics.MetricsEngineMock{}
	me.On("RecordAuctionAudit", mock.Anything, mock.Anything, mock.Anything).Return()
	me.On("RecordAuctionAuditActiveFilters", mock.Anything).Return()

	module := newTestModule(t, me, 1)
	module.producer.producer.(*mocks.AsyncProducer).ExpectInputAndSucceed()

	registerTestFilter(t, module.filterRegistry)

	// First call should pass (burst of 1)
	module.LogAuctionObject(newTestAuctionObject())

	matchedCalls := 0
	sampledCalls := 0
	for _, call := range me.Calls {
		if call.Method == "RecordAuctionAudit" {
			action := call.Arguments.Get(0).(metrics.AuctionAuditAction)
			count := call.Arguments.Get(2).(int)
			if action == metrics.AuctionAuditEventMatched {
				matchedCalls += count
			}
			if action == metrics.AuctionAuditEventDropped {
				sampledCalls += count
			}
		}
	}
	assert.Equal(t, 1, matchedCalls, "first event should be matched")
	assert.Equal(t, 0, sampledCalls, "no events should be sampled yet")

	// Second call should be sampled (rate exhausted)
	module.LogAuctionObject(newTestAuctionObject())

	matchedCalls = 0
	sampledCalls = 0
	for _, call := range me.Calls {
		if call.Method == "RecordAuctionAudit" {
			action := call.Arguments.Get(0).(metrics.AuctionAuditAction)
			count := call.Arguments.Get(2).(int)
			if action == metrics.AuctionAuditEventMatched {
				matchedCalls += count
			}
			if action == metrics.AuctionAuditEventDropped {
				sampledCalls += count
			}
		}
	}
	assert.Equal(t, 1, matchedCalls, "still only one matched event")
	assert.Equal(t, 1, sampledCalls, "second event should be sampled")
}

func TestLogAuctionObject_PerFilterRateLimit_IndependentPerFilter(t *testing.T) {
	me := &metrics.MetricsEngineMock{}
	me.On("RecordAuctionAudit", mock.Anything, mock.Anything, mock.Anything).Return()
	me.On("RecordAuctionAuditActiveFilters", mock.Anything).Return()

	module := newTestModule(t, me, 1)

	// Register two filters for the same account
	err := module.filterRegistry.Register(&AuctionFilterRequest{
		SessionId:   1,
		PartitionId: 0,
		AccountId:   "testaccount",
		ExpiresAtMs: time.Now().Add(time.Hour).UnixMilli(),
	})
	assert.NoError(t, err)

	err = module.filterRegistry.Register(&AuctionFilterRequest{
		SessionId:   2,
		PartitionId: 1,
		AccountId:   "testaccount",
		ExpiresAtMs: time.Now().Add(time.Hour).UnixMilli(),
	})
	assert.NoError(t, err)

	// First call: both filters should pass (each has burst of 1)
	module.producer.producer.(*mocks.AsyncProducer).ExpectInputAndSucceed()
	module.producer.producer.(*mocks.AsyncProducer).ExpectInputAndSucceed()
	module.LogAuctionObject(newTestAuctionObject())

	matchedCalls := 0
	sampledCalls := 0
	for _, call := range me.Calls {
		if call.Method == "RecordAuctionAudit" {
			action := call.Arguments.Get(0).(metrics.AuctionAuditAction)
			count := call.Arguments.Get(2).(int)
			if action == metrics.AuctionAuditEventMatched {
				matchedCalls += count
			}
			if action == metrics.AuctionAuditEventDropped {
				sampledCalls += count
			}
		}
	}
	assert.Equal(t, 2, matchedCalls, "both filters should match on first call")
	assert.Equal(t, 0, sampledCalls, "no events should be sampled yet")

	// Second call: both filters should be rate-limited
	module.LogAuctionObject(newTestAuctionObject())

	matchedCalls = 0
	sampledCalls = 0
	for _, call := range me.Calls {
		if call.Method == "RecordAuctionAudit" {
			action := call.Arguments.Get(0).(metrics.AuctionAuditAction)
			count := call.Arguments.Get(2).(int)
			if action == metrics.AuctionAuditEventMatched {
				matchedCalls += count
			}
			if action == metrics.AuctionAuditEventDropped {
				sampledCalls += count
			}
		}
	}
	assert.Equal(t, 2, matchedCalls, "still only two matched from first call")
	assert.Equal(t, 2, sampledCalls, "both filters should be sampled on second call")
}

func TestLogAuctionObject_ZeroConfig_NoRateLimiting(t *testing.T) {
	registry := NewFilterRegistry(10, time.Hour, 0, &metrics.MetricsEngineMock{})
	registry.mu.RLock()
	// No way to directly inspect, but registering a filter and checking it passes unlimited
	registry.mu.RUnlock()

	me := &metrics.MetricsEngineMock{}
	me.On("RecordAuctionAudit", mock.Anything, mock.Anything, mock.Anything).Return()
	me.On("RecordAuctionAuditActiveFilters", mock.Anything).Return()

	module := newTestModule(t, me, 0)

	registerTestFilter(t, module.filterRegistry)

	// Send many events — none should be sampled
	for i := 0; i < 10; i++ {
		module.producer.producer.(*mocks.AsyncProducer).ExpectInputAndSucceed()
		module.LogAuctionObject(newTestAuctionObject())
	}

	sampledCalls := 0
	matchedCalls := 0
	for _, call := range me.Calls {
		if call.Method == "RecordAuctionAudit" {
			action := call.Arguments.Get(0).(metrics.AuctionAuditAction)
			if action == metrics.AuctionAuditEventDropped {
				sampledCalls++
			}
			if action == metrics.AuctionAuditEventMatched {
				matchedCalls++
			}
		}
	}
	assert.Equal(t, 0, sampledCalls, "no events should be sampled with rate=0")
	assert.Equal(t, 10, matchedCalls, "all events should be matched")
}
