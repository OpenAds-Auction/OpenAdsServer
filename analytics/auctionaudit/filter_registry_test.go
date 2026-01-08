package auctionaudit

import (
	"testing"
	"time"

	metricsConfig "github.com/prebid/prebid-server/v3/metrics/config"
	"github.com/stretchr/testify/assert"
)

func newTestRegistry(maxFilters int) *FilterRegistry {
	return NewFilterRegistry(maxFilters, 1*time.Hour, &metricsConfig.NilMetricsEngine{})
}

func TestMediaTypeSet(t *testing.T) {
	tests := []struct {
		name     string
		types    []MediaType
		expected MediaTypeSet
	}{
		{"empty", nil, 0},
		{"banner only", []MediaType{MediaType_MEDIA_TYPE_BANNER}, MediaTypeBannerBit},
		{"video only", []MediaType{MediaType_MEDIA_TYPE_VIDEO}, MediaTypeVideoBit},
		{"audio only", []MediaType{MediaType_MEDIA_TYPE_AUDIO}, MediaTypeAudioBit},
		{"native only", []MediaType{MediaType_MEDIA_TYPE_NATIVE}, MediaTypeNativeBit},
		{"banner and video", []MediaType{MediaType_MEDIA_TYPE_BANNER, MediaType_MEDIA_TYPE_VIDEO}, MediaTypeBannerBit | MediaTypeVideoBit},
		{"all types", []MediaType{MediaType_MEDIA_TYPE_BANNER, MediaType_MEDIA_TYPE_VIDEO, MediaType_MEDIA_TYPE_AUDIO, MediaType_MEDIA_TYPE_NATIVE}, MediaTypeBannerBit | MediaTypeVideoBit | MediaTypeAudioBit | MediaTypeNativeBit},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToMediaTypeSet(tt.types)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMediaTypeSet_Intersects(t *testing.T) {
	tests := []struct {
		name     string
		set1     MediaTypeSet
		set2     MediaTypeSet
		expected bool
	}{
		{"both empty", 0, 0, false},
		{"one empty", MediaTypeBannerBit, 0, false},
		{"same type", MediaTypeBannerBit, MediaTypeBannerBit, true},
		{"different types", MediaTypeBannerBit, MediaTypeVideoBit, false},
		{"overlap one", MediaTypeBannerBit | MediaTypeVideoBit, MediaTypeVideoBit | MediaTypeAudioBit, true},
		{"no overlap", MediaTypeBannerBit | MediaTypeVideoBit, MediaTypeAudioBit | MediaTypeNativeBit, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.set1.Intersects(tt.set2)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFilterRegistry_Register(t *testing.T) {
	registry := newTestRegistry(10)

	filter := &AuctionFilterRequest{
		SessionId:   1,
		PartitionId: 0,
		AccountId:   "account-123",
		Domain:      "example.com",
		ExpiresAtMs: time.Now().Add(10 * time.Minute).UnixMilli(),
	}

	assert.NoError(t, registry.Register(filter))
	assert.Equal(t, 1, registry.Count())

	filter2 := &AuctionFilterRequest{
		SessionId:   1,
		PartitionId: 1,
		AccountId:   "account-123",
		Domain:      "updated.com",
		ExpiresAtMs: time.Now().Add(10 * time.Minute).UnixMilli(),
	}
	assert.NoError(t, registry.Register(filter2))
	assert.Equal(t, 1, registry.Count())

	matches := registry.GetMatches("account-123", "updated.com", "", 0)
	assert.Len(t, matches, 1)
}

func TestFilterRegistry_Register_MaxLimit(t *testing.T) {
	registry := newTestRegistry(2)

	assert.NoError(t, registry.Register(&AuctionFilterRequest{SessionId: 1, AccountId: "a1"}))
	assert.NoError(t, registry.Register(&AuctionFilterRequest{SessionId: 2, AccountId: "a2"}))
	assert.Equal(t, 2, registry.Count())

	assert.ErrorIs(t, registry.Register(&AuctionFilterRequest{SessionId: 3, AccountId: "a3"}), ErrRegistryAtCapacity)
	assert.Equal(t, 2, registry.Count())

	assert.NoError(t, registry.Register(&AuctionFilterRequest{SessionId: 1, AccountId: "a1", Domain: "updated.com"}))
	assert.Equal(t, 2, registry.Count())
}

func TestFilterRegistry_Register_InvalidFilter(t *testing.T) {
	registry := newTestRegistry(10)

	assert.ErrorIs(t, registry.Register(nil), ErrInvalidFilterRequest)

	assert.ErrorIs(t, registry.Register(&AuctionFilterRequest{AccountId: "a1"}), ErrInvalidFilterRequest)

	assert.ErrorIs(t, registry.Register(&AuctionFilterRequest{SessionId: 1}), ErrInvalidFilterRequest)

	assert.Equal(t, 0, registry.Count())
}

func TestFilterRegistry_Unregister(t *testing.T) {
	registry := newTestRegistry(10)

	registry.Register(&AuctionFilterRequest{SessionId: 1, AccountId: "a1"})
	registry.Register(&AuctionFilterRequest{SessionId: 2, AccountId: "a2"})
	assert.Equal(t, 2, registry.Count())

	registry.Unregister(1, "a1")
	assert.Equal(t, 1, registry.Count())

	// Unregister non-existent is a no-op
	registry.Unregister(999, "a999")
	assert.Equal(t, 1, registry.Count())

	// Unregister with wrong accountId is a no-op
	registry.Unregister(2, "wrong-account")
	assert.Equal(t, 1, registry.Count())
}

func TestFilterRegistry_GetMatches_AccountIdRequired(t *testing.T) {
	registry := newTestRegistry(10)

	registry.Register(&AuctionFilterRequest{
		SessionId: 1,
		AccountId: "account-123",
	})

	matches := registry.GetMatches("account-123", "", "", 0)
	assert.Len(t, matches, 1)

	matches = registry.GetMatches("account-456", "", "", 0)
	assert.Len(t, matches, 0)
}

func TestFilterRegistry_GetMatches_DomainFilter(t *testing.T) {
	registry := newTestRegistry(10)

	registry.Register(&AuctionFilterRequest{
		SessionId: 1,
		AccountId: "account-123",
		Domain:    "example.com",
	})

	matches := registry.GetMatches("account-123", "example.com", "", 0)
	assert.Len(t, matches, 1)

	matches = registry.GetMatches("account-123", "other.com", "", 0)
	assert.Len(t, matches, 0)

	matches = registry.GetMatches("account-123", "", "", 0)
	assert.Len(t, matches, 0)
}

func TestFilterRegistry_GetMatches_AppBundleFilter(t *testing.T) {
	registry := newTestRegistry(10)

	registry.Register(&AuctionFilterRequest{
		SessionId: 1,
		AccountId: "account-123",
		AppBundle: "com.example.app",
	})

	matches := registry.GetMatches("account-123", "", "com.example.app", 0)
	assert.Len(t, matches, 1)

	matches = registry.GetMatches("account-123", "", "com.other.app", 0)
	assert.Len(t, matches, 0)
}

func TestFilterRegistry_GetMatches_MediaTypeFilter(t *testing.T) {
	registry := newTestRegistry(10)

	registry.Register(&AuctionFilterRequest{
		SessionId:  1,
		AccountId:  "account-123",
		MediaTypes: []MediaType{MediaType_MEDIA_TYPE_VIDEO, MediaType_MEDIA_TYPE_BANNER},
	})

	matches := registry.GetMatches("account-123", "", "", MediaTypeVideoBit)
	assert.Len(t, matches, 1)

	matches = registry.GetMatches("account-123", "", "", MediaTypeBannerBit)
	assert.Len(t, matches, 1)

	matches = registry.GetMatches("account-123", "", "", MediaTypeAudioBit)
	assert.Len(t, matches, 0)

	matches = registry.GetMatches("account-123", "", "", MediaTypeAudioBit|MediaTypeVideoBit)
	assert.Len(t, matches, 1)

	matches = registry.GetMatches("account-123", "", "", MediaTypeBannerBit|MediaTypeVideoBit)
	assert.Len(t, matches, 1)
}

func TestFilterRegistry_GetMatches_NoMediaTypeFilter(t *testing.T) {
	registry := newTestRegistry(10)

	registry.Register(&AuctionFilterRequest{
		SessionId: 1,
		AccountId: "account-123",
	})

	matches := registry.GetMatches("account-123", "", "", MediaTypeVideoBit)
	assert.Len(t, matches, 1)

	matches = registry.GetMatches("account-123", "", "", MediaTypeNativeBit)
	assert.Len(t, matches, 1)

	matches = registry.GetMatches("account-123", "", "", 0)
	assert.Len(t, matches, 1)
}

func TestFilterRegistry_GetMatches_MultipleFiltersPerAccount(t *testing.T) {
	registry := newTestRegistry(10)

	registry.Register(&AuctionFilterRequest{
		SessionId: 1,
		AccountId: "account-123",
		Domain:    "example.com",
	})
	registry.Register(&AuctionFilterRequest{
		SessionId: 2,
		AccountId: "account-123",
		AppBundle: "com.example.app",
	})

	assert.Equal(t, 2, registry.Count())

	// Event matches first filter only
	matches := registry.GetMatches("account-123", "example.com", "", 0)
	assert.Len(t, matches, 1)
	assert.Equal(t, int32(1), matches[0].SessionId)

	// Event matches second filter only
	matches = registry.GetMatches("account-123", "", "com.example.app", 0)
	assert.Len(t, matches, 1)
	assert.Equal(t, int32(2), matches[0].SessionId)
}

func TestFilterRegistry_GetMatches_DifferentAccounts(t *testing.T) {
	registry := newTestRegistry(10)

	registry.Register(&AuctionFilterRequest{
		SessionId: 1,
		AccountId: "account-123",
	})
	registry.Register(&AuctionFilterRequest{
		SessionId: 2,
		AccountId: "account-456",
	})

	// Only returns filters for the requested account
	matches := registry.GetMatches("account-123", "", "", 0)
	assert.Len(t, matches, 1)
	assert.Equal(t, int32(1), matches[0].SessionId)

	matches = registry.GetMatches("account-456", "", "", 0)
	assert.Len(t, matches, 1)
	assert.Equal(t, int32(2), matches[0].SessionId)

	// No filters for this account
	matches = registry.GetMatches("account-789", "", "", 0)
	assert.Len(t, matches, 0)
}

func TestFilterRegistry_GetMatches_ExpiredFilter(t *testing.T) {
	registry := newTestRegistry(10)

	// Expired filter
	registry.Register(&AuctionFilterRequest{
		SessionId:   1,
		AccountId:   "account-123",
		ExpiresAtMs: time.Now().Add(-1 * time.Minute).UnixMilli(), // Expired 1 minute ago
	})

	// Valid filter
	registry.Register(&AuctionFilterRequest{
		SessionId:   2,
		AccountId:   "account-123",
		ExpiresAtMs: time.Now().Add(10 * time.Minute).UnixMilli(),
	})

	matches := registry.GetMatches("account-123", "", "", 0)
	assert.Len(t, matches, 1)
	assert.Equal(t, int32(2), matches[0].SessionId)
}

func TestFilterRegistry_GetMatches_NoExpiry(t *testing.T) {
	registry := newTestRegistry(10)

	registry.Register(&AuctionFilterRequest{
		SessionId:   1,
		AccountId:   "account-123",
		ExpiresAtMs: 0,
	})

	matches := registry.GetMatches("account-123", "", "", 0)
	assert.Len(t, matches, 1)
}

func TestFilterRegistry_CleanupExpired(t *testing.T) {
	registry := newTestRegistry(10)

	// expired
	registry.Register(&AuctionFilterRequest{
		SessionId:   1,
		AccountId:   "account-123",
		ExpiresAtMs: time.Now().Add(-1 * time.Minute).UnixMilli(),
	})

	// valid
	registry.Register(&AuctionFilterRequest{
		SessionId:   2,
		AccountId:   "account-123",
		ExpiresAtMs: time.Now().Add(10 * time.Minute).UnixMilli(),
	})

	assert.Equal(t, 2, registry.Count())

	registry.cleanupExpired()

	assert.Equal(t, 1, registry.Count())
	matches := registry.GetMatches("account-123", "", "", 0)
	assert.Len(t, matches, 1)
	assert.Equal(t, int32(2), matches[0].SessionId)
}

func TestFilterRegistry_CleanupExpired_RemovesEmptyAccountMap(t *testing.T) {
	registry := newTestRegistry(10)

	// expired
	registry.Register(&AuctionFilterRequest{
		SessionId:   1,
		AccountId:   "account-123",
		ExpiresAtMs: time.Now().Add(-1 * time.Minute).UnixMilli(),
	})

	assert.Equal(t, 1, registry.Count())

	registry.cleanupExpired()

	assert.Equal(t, 0, registry.Count())

	matches := registry.GetMatches("account-123", "", "", 0)
	assert.Len(t, matches, 0)
}

func TestFilterRegistry_CombinedFilters(t *testing.T) {
	registry := newTestRegistry(10)

	registry.Register(&AuctionFilterRequest{
		SessionId:   1,
		AccountId:   "account-123",
		Domain:      "example.com",
		AppBundle:   "com.example.app",
		MediaTypes:  []MediaType{MediaType_MEDIA_TYPE_VIDEO},
		ExpiresAtMs: time.Now().Add(10 * time.Minute).UnixMilli(),
	})

	// All criteria match
	matches := registry.GetMatches("account-123", "example.com", "com.example.app", MediaTypeVideoBit)
	assert.Len(t, matches, 1)

	// Wrong account
	matches = registry.GetMatches("account-456", "example.com", "com.example.app", MediaTypeVideoBit)
	assert.Len(t, matches, 0)

	// Wrong domain
	matches = registry.GetMatches("account-123", "other.com", "com.example.app", MediaTypeVideoBit)
	assert.Len(t, matches, 0)

	// Wrong app bundle
	matches = registry.GetMatches("account-123", "example.com", "com.other.app", MediaTypeVideoBit)
	assert.Len(t, matches, 0)

	// Wrong media type
	matches = registry.GetMatches("account-123", "example.com", "com.example.app", MediaTypeBannerBit)
	assert.Len(t, matches, 0)
}

func TestFilterRegistry_MaxTTL_CapsExpiration(t *testing.T) {
	maxTTL := 1 * time.Hour
	registry := NewFilterRegistry(10, maxTTL, &metricsConfig.NilMetricsEngine{})

	filter := &AuctionFilterRequest{
		SessionId:   1,
		AccountId:   "account-123",
		ExpiresAtMs: time.Now().Add(5 * 24 * time.Hour).UnixMilli(),
	}

	registry.Register(filter)

	maxAllowed := time.Now().Add(maxTTL).UnixMilli()
	assert.LessOrEqual(t, filter.ExpiresAtMs, maxAllowed+1000)
	assert.Greater(t, filter.ExpiresAtMs, time.Now().UnixMilli())
}

func TestFilterRegistry_MaxTTL_ZeroExpiration(t *testing.T) {
	maxTTL := 1 * time.Hour
	registry := NewFilterRegistry(10, maxTTL, &metricsConfig.NilMetricsEngine{})

	filter := &AuctionFilterRequest{
		SessionId:   1,
		AccountId:   "account-123",
		ExpiresAtMs: 0,
	}

	registry.Register(filter)

	maxAllowed := time.Now().Add(maxTTL).UnixMilli()
	assert.LessOrEqual(t, filter.ExpiresAtMs, maxAllowed+1000)
	assert.Greater(t, filter.ExpiresAtMs, time.Now().UnixMilli())
}

func TestFilterRegistry_MaxTTL_ValidExpiration(t *testing.T) {
	maxTTL := 1 * time.Hour
	registry := NewFilterRegistry(10, maxTTL, &metricsConfig.NilMetricsEngine{})

	expectedExpiration := time.Now().Add(30 * time.Minute).UnixMilli()
	filter := &AuctionFilterRequest{
		SessionId:   1,
		AccountId:   "account-123",
		ExpiresAtMs: expectedExpiration,
	}

	registry.Register(filter)

	assert.InDelta(t, expectedExpiration, filter.ExpiresAtMs, 1000)
}
