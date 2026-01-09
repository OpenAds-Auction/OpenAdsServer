package auctionaudit

import (
	"testing"
	"time"

	"github.com/IBM/sarama"
	metricsConfig "github.com/prebid/prebid-server/v3/metrics/config"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

func createTestHandler() (*filterConsumerHandler, *FilterRegistry) {
	registry := NewFilterRegistry(100, 1*time.Hour, &metricsConfig.NilMetricsEngine{})
	handler := &filterConsumerHandler{
		registry:      registry,
		metricsEngine: &metricsConfig.NilMetricsEngine{},
	}
	return handler, registry
}

func createValidFilterMessage(sessionId int32, accountId string, key []byte) *sarama.ConsumerMessage {
	filter := &AuctionFilterRequest{
		SessionId:   sessionId,
		AccountId:   accountId,
		PartitionId: 0,
		ExpiresAtMs: 0,
	}
	data, _ := proto.Marshal(filter)
	return &sarama.ConsumerMessage{
		Key:   key,
		Value: data,
	}
}

func TestProcessMessage_CreateWithEmptyKey(t *testing.T) {
	handler, registry := createTestHandler()

	msg := createValidFilterMessage(123, "test-account", nil)
	handler.processMessage(msg)

	assert.Equal(t, 1, registry.Count(), "Filter should be registered")
}

func TestProcessMessage_CreateWithExplicitKey(t *testing.T) {
	handler, registry := createTestHandler()

	msg := createValidFilterMessage(456, "test-account", []byte{FilterActionCreate})
	handler.processMessage(msg)

	assert.Equal(t, 1, registry.Count(), "Filter should be registered")
}

func TestProcessMessage_RemoveAction(t *testing.T) {
	handler, registry := createTestHandler()

	createMsg := createValidFilterMessage(789, "test-account", nil)
	handler.processMessage(createMsg)
	assert.Equal(t, 1, registry.Count(), "Filter should be registered first")

	removeMsg := createValidFilterMessage(789, "test-account", []byte{FilterActionRemove})
	handler.processMessage(removeMsg)

	assert.Equal(t, 0, registry.Count(), "Filter should be unregistered")
}

func TestProcessMessage_InvalidProtobuf(t *testing.T) {
	handler, registry := createTestHandler()

	msg := &sarama.ConsumerMessage{
		Key:   nil,
		Value: []byte("not valid protobuf data"),
	}

	handler.processMessage(msg)

	assert.Equal(t, 0, registry.Count(), "No filter should be registered")
}

func TestProcessMessage_UnknownAction_DefaultsToCreate(t *testing.T) {
	handler, registry := createTestHandler()

	msg := createValidFilterMessage(999, "test-account", []byte{99})

	handler.processMessage(msg)

	assert.Equal(t, 1, registry.Count(), "Unknown action should default to create")
}

func TestProcessMessage_MultipleFilters(t *testing.T) {
	handler, registry := createTestHandler()

	handler.processMessage(createValidFilterMessage(1, "account-a", nil))
	handler.processMessage(createValidFilterMessage(2, "account-a", nil))
	handler.processMessage(createValidFilterMessage(3, "account-b", nil))

	assert.Equal(t, 3, registry.Count(), "All three filters should be registered")

	handler.processMessage(createValidFilterMessage(2, "account-a", []byte{FilterActionRemove}))

	assert.Equal(t, 2, registry.Count(), "Two filters should remain")
}
