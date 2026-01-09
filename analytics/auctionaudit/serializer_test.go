package auctionaudit

import (
	"net/http"
	"testing"
	"time"

	"github.com/prebid/openrtb/v20/openrtb2"
	"github.com/prebid/prebid-server/v3/analytics"
	"github.com/prebid/prebid-server/v3/config"
	"github.com/prebid/prebid-server/v3/errortypes"
	"github.com/prebid/prebid-server/v3/openrtb_ext"
	"github.com/stretchr/testify/assert"
)

func TestBuildAuctionEvent(t *testing.T) {
	startTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	ao := &analytics.AuctionObject{
		Status:    http.StatusOK,
		StartTime: startTime,
		Account: &config.Account{
			ID: "test-account-123",
		},
		RequestWrapper: &openrtb_ext.RequestWrapper{
			BidRequest: &openrtb2.BidRequest{
				ID: "test-request-id",
				Site: &openrtb2.Site{
					Domain: "example.com",
				},
				Imp: []openrtb2.Imp{
					{Banner: &openrtb2.Banner{}},
					{Video: &openrtb2.Video{}},
				},
			},
		},
		Response: &openrtb2.BidResponse{
			ID: "test-response-id",
		},
	}

	mediaTypes := []MediaType{MediaType_MEDIA_TYPE_BANNER, MediaType_MEDIA_TYPE_VIDEO}
	event := buildAuctionEvent(ao, "production", "test-account-123", "example.com", "", mediaTypes)

	assert.Equal(t, "test-account-123", event.AccountId)
	assert.Equal(t, "example.com", event.Domain)
	assert.Equal(t, "", event.AppBundle)
	assert.Equal(t, "production", event.Environment)
	assert.Equal(t, startTime.UnixMilli(), event.TimestampMs)
	assert.Equal(t, int32(http.StatusOK), event.Status)
	assert.Contains(t, event.BidRequest, "test-request-id")
	assert.Contains(t, event.BidResponse, "test-response-id")
	assert.ElementsMatch(t, []MediaType{MediaType_MEDIA_TYPE_BANNER, MediaType_MEDIA_TYPE_VIDEO}, event.MediaTypes)
}

func TestBuildAuctionEvent_WithApp(t *testing.T) {
	startTime := time.Now()

	ao := &analytics.AuctionObject{
		Status:    http.StatusOK,
		StartTime: startTime,
		Account: &config.Account{
			ID: "app-account",
		},
		RequestWrapper: &openrtb_ext.RequestWrapper{
			BidRequest: &openrtb2.BidRequest{
				ID: "app-request",
				App: &openrtb2.App{
					Bundle: "com.example.app",
				},
				Imp: []openrtb2.Imp{
					{Native: &openrtb2.Native{}},
				},
			},
		},
	}

	mediaTypes := []MediaType{MediaType_MEDIA_TYPE_NATIVE}
	event := buildAuctionEvent(ao, "staging", "app-account", "", "com.example.app", mediaTypes)

	assert.Equal(t, "app-account", event.AccountId)
	assert.Equal(t, "", event.Domain)
	assert.Equal(t, "com.example.app", event.AppBundle)
	assert.Equal(t, startTime.UnixMilli(), event.TimestampMs)
	assert.ElementsMatch(t, []MediaType{MediaType_MEDIA_TYPE_NATIVE}, event.MediaTypes)
}

func TestBuildAuctionEvent_WithErrors(t *testing.T) {
	startTime := time.Now()

	ao := &analytics.AuctionObject{
		Status:    http.StatusBadRequest,
		StartTime: startTime,
		Errors: []error{
			&errortypes.BadInput{Message: "invalid request"},
			&errortypes.Timeout{Message: "bidder timeout"},
		},
		RequestWrapper: &openrtb_ext.RequestWrapper{
			BidRequest: &openrtb2.BidRequest{
				ID: "error-request",
			},
		},
	}

	event := buildAuctionEvent(ao, "test", "", "", "", nil)

	assert.Equal(t, int32(http.StatusBadRequest), event.Status)
	assert.Len(t, event.Errors, 2)
	assert.Equal(t, "invalid request", event.Errors[0].Message)
	assert.Equal(t, int32(errortypes.BadInputErrorCode), event.Errors[0].Code)
	assert.Equal(t, "bidder timeout", event.Errors[1].Message)
	assert.Equal(t, int32(errortypes.TimeoutErrorCode), event.Errors[1].Code)
}

func TestMediaTypeSetToSlice(t *testing.T) {
	tests := []struct {
		name     string
		imps     []openrtb2.Imp
		expected []MediaType
	}{
		{
			name:     "empty impressions",
			imps:     []openrtb2.Imp{},
			expected: nil,
		},
		{
			name: "banner only",
			imps: []openrtb2.Imp{
				{Banner: &openrtb2.Banner{}},
			},
			expected: []MediaType{MediaType_MEDIA_TYPE_BANNER},
		},
		{
			name: "video only",
			imps: []openrtb2.Imp{
				{Video: &openrtb2.Video{}},
			},
			expected: []MediaType{MediaType_MEDIA_TYPE_VIDEO},
		},
		{
			name: "multiple banner imps - deduplicated",
			imps: []openrtb2.Imp{
				{Banner: &openrtb2.Banner{}},
				{Banner: &openrtb2.Banner{}},
				{Banner: &openrtb2.Banner{}},
			},
			expected: []MediaType{MediaType_MEDIA_TYPE_BANNER},
		},
		{
			name: "all media types",
			imps: []openrtb2.Imp{
				{Banner: &openrtb2.Banner{}},
				{Video: &openrtb2.Video{}},
				{Audio: &openrtb2.Audio{}},
				{Native: &openrtb2.Native{}},
			},
			expected: []MediaType{MediaType_MEDIA_TYPE_BANNER, MediaType_MEDIA_TYPE_VIDEO, MediaType_MEDIA_TYPE_AUDIO, MediaType_MEDIA_TYPE_NATIVE},
		},
		{
			name: "multi-format imp",
			imps: []openrtb2.Imp{
				{Banner: &openrtb2.Banner{}, Video: &openrtb2.Video{}},
			},
			expected: []MediaType{MediaType_MEDIA_TYPE_BANNER, MediaType_MEDIA_TYPE_VIDEO},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mediaTypeSet := MediaTypeSetFromImps(tt.imps)
			result := mediaTypeSet.ToSlice()
			assert.ElementsMatch(t, tt.expected, result)
		})
	}
}

func TestSerializeToProtobuf(t *testing.T) {
	event := &AuctionEvent{
		AccountId:   "account-123",
		Domain:      "example.com",
		Environment: "test",
		TimestampMs: 1705315800000,
		Status:      200,
		MediaTypes:  []MediaType{MediaType_MEDIA_TYPE_BANNER, MediaType_MEDIA_TYPE_VIDEO},
		BidRequest:  `{"id":"test"}`,
	}

	data, err := serializeToProtobuf(event)

	assert.NoError(t, err)
	assert.NotEmpty(t, data)
}
