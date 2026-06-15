package auctionaudit

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/prebid/openrtb/v20/openrtb2"
	"github.com/prebid/prebid-server/v3/logger"
	"github.com/prebid/prebid-server/v3/metrics"
	"golang.org/x/time/rate"
)

var (
	ErrInvalidFilterRequest = errors.New("filter is nil or missing required fields (session_id, account_id)")
	ErrRegistryAtCapacity   = errors.New("filter registry at max capacity")
)

type MediaTypeSet uint8

const (
	MediaTypeBannerBit MediaTypeSet = 1 << iota // 1
	MediaTypeVideoBit                           // 2
	MediaTypeAudioBit                           // 4
	MediaTypeNativeBit                          // 8
)

func ToMediaTypeSet(types []MediaType) MediaTypeSet {
	var set MediaTypeSet
	for _, t := range types {
		switch t {
		case MediaType_MEDIA_TYPE_BANNER:
			set |= MediaTypeBannerBit
		case MediaType_MEDIA_TYPE_VIDEO:
			set |= MediaTypeVideoBit
		case MediaType_MEDIA_TYPE_AUDIO:
			set |= MediaTypeAudioBit
		case MediaType_MEDIA_TYPE_NATIVE:
			set |= MediaTypeNativeBit
		}
	}
	return set
}

func MediaTypeSetFromImps(imps []openrtb2.Imp) MediaTypeSet {
	var set MediaTypeSet
	for i := range imps {
		if imps[i].Banner != nil {
			set |= MediaTypeBannerBit
		}
		if imps[i].Video != nil {
			set |= MediaTypeVideoBit
		}
		if imps[i].Audio != nil {
			set |= MediaTypeAudioBit
		}
		if imps[i].Native != nil {
			set |= MediaTypeNativeBit
		}
	}
	return set
}

// Intersects returns true if any media type is present in both sets
func (s MediaTypeSet) Intersects(other MediaTypeSet) bool {
	return (s & other) != 0
}

// ToSlice converts the bitmask to a slice of MediaType enums
func (s MediaTypeSet) ToSlice() []MediaType {
	var result []MediaType
	if s&MediaTypeBannerBit != 0 {
		result = append(result, MediaType_MEDIA_TYPE_BANNER)
	}
	if s&MediaTypeVideoBit != 0 {
		result = append(result, MediaType_MEDIA_TYPE_VIDEO)
	}
	if s&MediaTypeAudioBit != 0 {
		result = append(result, MediaType_MEDIA_TYPE_AUDIO)
	}
	if s&MediaTypeNativeBit != 0 {
		result = append(result, MediaType_MEDIA_TYPE_NATIVE)
	}
	return result
}

type storedFilter struct {
	*AuctionFilterRequest
	mediaTypeSet MediaTypeSet
	rateLimiter  *rate.Limiter
}

func (f *storedFilter) matches(domain, appBundle string, eventMediaTypes MediaTypeSet) bool {
	if f.Domain != "" && !strings.EqualFold(f.Domain, domain) {
		return false
	}

	if f.AppBundle != "" && !strings.EqualFold(f.AppBundle, appBundle) {
		return false
	}

	// at least 1 media type must be present
	if f.mediaTypeSet != 0 && !f.mediaTypeSet.Intersects(eventMediaTypes) {
		return false
	}

	return true
}

type FilterRegistry struct {
	mu              sync.RWMutex
	byAccount       map[string]map[int32]*storedFilter // accountId -> sessionId -> filter
	count           int
	maxFilters      int
	maxTTL          time.Duration
	maxEventsPerSec float64
	metricsEngine   metrics.MetricsEngine
}

func NewFilterRegistry(maxFilters int, maxTTL time.Duration, maxEventsPerSec float64, metricsEngine metrics.MetricsEngine) *FilterRegistry {
	return &FilterRegistry{
		byAccount:       make(map[string]map[int32]*storedFilter),
		maxFilters:      maxFilters,
		maxTTL:          maxTTL,
		maxEventsPerSec: maxEventsPerSec,
		metricsEngine:   metricsEngine,
	}
}

func (r *FilterRegistry) Start(ctx context.Context, cleanupInterval time.Duration) {
	go r.cleanupLoop(ctx, cleanupInterval)
}

func (r *FilterRegistry) Register(filter *AuctionFilterRequest) error {
	if filter == nil || filter.SessionId == 0 || filter.AccountId == "" {
		return ErrInvalidFilterRequest
	}

	// Cap expiration to max TTL
	maxExpiration := time.Now().Add(r.maxTTL).UnixMilli()
	if filter.ExpiresAtMs == 0 || filter.ExpiresAtMs > maxExpiration {
		filter.ExpiresAtMs = maxExpiration
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	accountFilters := r.byAccount[filter.AccountId]
	var exists bool
	if accountFilters != nil {
		_, exists = accountFilters[filter.SessionId]
	}

	// reject if at capacity
	if !exists && r.count >= r.maxFilters {
		logger.Warnf("[auctionaudit] Filter rejected: max filters (%d) reached", r.maxFilters)
		return ErrRegistryAtCapacity
	}

	if accountFilters == nil {
		accountFilters = make(map[int32]*storedFilter)
		r.byAccount[filter.AccountId] = accountFilters
	}

	sf := &storedFilter{
		AuctionFilterRequest: filter,
		mediaTypeSet:         ToMediaTypeSet(filter.MediaTypes),
	}
	if r.maxEventsPerSec > 0 {
		sf.rateLimiter = rate.NewLimiter(rate.Limit(r.maxEventsPerSec), int(r.maxEventsPerSec))
	}
	accountFilters[filter.SessionId] = sf

	if !exists {
		r.count++
		r.metricsEngine.RecordAuctionAudit(metrics.AuctionAuditFilterRegistered, filter.AccountId)
	}
	r.metricsEngine.RecordAuctionAuditActiveFilters(r.count)
	return nil
}

func (r *FilterRegistry) Unregister(sessionId int32, accountId string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	accountFilters := r.byAccount[accountId]
	if accountFilters == nil {
		return
	}

	if _, exists := accountFilters[sessionId]; exists {
		delete(accountFilters, sessionId)
		r.count--

		if len(accountFilters) == 0 {
			delete(r.byAccount, accountId)
		}
		r.metricsEngine.RecordAuctionAuditActiveFilters(r.count)
	}
}

// GetMatches returns filters that match AND pass their per-filter rate limiter.
// dropped is the count of filters that matched but were dropped by rate limiting.
func (r *FilterRegistry) GetMatches(accountID, domain, appBundle string, eventMediaTypes MediaTypeSet) (allowed []*AuctionFilterRequest, dropped int) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	accountFilters := r.byAccount[accountID]
	if len(accountFilters) == 0 {
		return nil, 0
	}

	now := time.Now().UnixMilli()

	for _, filter := range accountFilters {
		if filter.ExpiresAtMs > 0 && filter.ExpiresAtMs < now {
			continue
		}

		if !filter.matches(domain, appBundle, eventMediaTypes) {
			continue
		}

		if filter.rateLimiter != nil && !filter.rateLimiter.Allow() {
			dropped++
			continue
		}

		allowed = append(allowed, filter.AuctionFilterRequest)
	}

	return allowed, dropped
}

func (r *FilterRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.count
}

func (r *FilterRegistry) cleanupLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.cleanupExpired()
		}
	}
}

func (r *FilterRegistry) cleanupExpired() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UnixMilli()
	expiredCount := 0

	for accountId, accountFilters := range r.byAccount {
		for sessionId, filter := range accountFilters {
			if filter.ExpiresAtMs > 0 && filter.ExpiresAtMs < now {
				delete(accountFilters, sessionId)
				expiredCount++
				r.metricsEngine.RecordAuctionAudit(metrics.AuctionAuditFilterExpired, filter.AccountId)
				logger.Infof("[auctionaudit] Filter expired: account=%s session=%d", filter.AccountId, filter.SessionId)
			}
		}

		if len(accountFilters) == 0 {
			delete(r.byAccount, accountId)
		}
	}

	if expiredCount > 0 {
		r.count -= expiredCount
	}

	r.metricsEngine.RecordAuctionAuditActiveFilters(r.count)
}
