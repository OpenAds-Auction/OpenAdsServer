package auctionaudit

import (
	"testing"

	"github.com/prebid/prebid-server/v3/config"
	"github.com/stretchr/testify/assert"
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
