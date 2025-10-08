package s3

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/prebid/openrtb/v20/openrtb2"
	"github.com/prebid/prebid-server/v3/analytics"
	"github.com/prebid/prebid-server/v3/openrtb_ext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSerializeAuctionObject(t *testing.T) {
	ao := &analytics.AuctionObject{
		Status: 200,
		RequestWrapper: &openrtb_ext.RequestWrapper{
			BidRequest: &openrtb2.BidRequest{
				ID: "test-auction-id",
			},
		},
		Response: &openrtb2.BidResponse{
			ID: "test-response-id",
		},
		StartTime: time.Now(),
	}

	data, err := serializeAuctionObject(ao)

	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Verify it's valid JSON
	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Equal(t, float64(200), result["Status"])
	assert.NotNil(t, result["Request"])
	assert.NotNil(t, result["Response"])
	// All fields should be present even if nil/empty
	assert.Contains(t, result, "Errors")
	assert.Contains(t, result, "Account")
	assert.Contains(t, result, "StartTime")
	assert.Contains(t, result, "HookExecutionOutcome")
	assert.Contains(t, result, "SeatNonBid")
}

func TestSerializeAuctionObject_NilRequestWrapper(t *testing.T) {
	ao := &analytics.AuctionObject{
		Status:         500,
		RequestWrapper: nil,
	}

	data, err := serializeAuctionObject(ao)

	require.NoError(t, err)
	assert.NotEmpty(t, data)

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Equal(t, float64(500), result["Status"])
	// Request field should still be present but nil
	assert.Contains(t, result, "Request")
	assert.Nil(t, result["Request"])
}

func TestSerializeAmpObject(t *testing.T) {
	ao := &analytics.AmpObject{
		Status: 200,
		RequestWrapper: &openrtb_ext.RequestWrapper{
			BidRequest: &openrtb2.BidRequest{
				ID: "test-amp-id",
			},
		},
		AuctionResponse: &openrtb2.BidResponse{
			ID: "test-response-id",
		},
		AmpTargetingValues: map[string]string{
			"hb_pb":     "1.50",
			"hb_bidder": "example",
		},
		Origin:    "https://example.com",
		StartTime: time.Now(),
	}

	data, err := serializeAmpObject(ao)

	require.NoError(t, err)
	assert.NotEmpty(t, data)

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Equal(t, float64(200), result["Status"])
	assert.NotNil(t, result["Request"])
	assert.NotNil(t, result["AuctionResponse"])
	assert.NotNil(t, result["AmpTargetingValues"])
	assert.Equal(t, "https://example.com", result["Origin"])
	// All fields should be present
	assert.Contains(t, result, "Errors")
	assert.Contains(t, result, "StartTime")
	assert.Contains(t, result, "HookExecutionOutcome")
	assert.Contains(t, result, "SeatNonBid")
}

func TestSerializeVideoObject(t *testing.T) {
	vo := &analytics.VideoObject{
		Status: 200,
		RequestWrapper: &openrtb_ext.RequestWrapper{
			BidRequest: &openrtb2.BidRequest{
				ID: "test-video-id",
			},
		},
		Response: &openrtb2.BidResponse{
			ID: "test-response-id",
		},
		StartTime: time.Now(),
	}

	data, err := serializeVideoObject(vo)

	require.NoError(t, err)
	assert.NotEmpty(t, data)

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Equal(t, float64(200), result["Status"])
	assert.NotNil(t, result["Request"])
	assert.NotNil(t, result["Response"])
	// All fields should be present
	assert.Contains(t, result, "Errors")
	assert.Contains(t, result, "VideoRequest")
	assert.Contains(t, result, "VideoResponse")
	assert.Contains(t, result, "StartTime")
	assert.Contains(t, result, "SeatNonBid")
}

func TestSerializeAuctionObject_WithErrors(t *testing.T) {
	ao := &analytics.AuctionObject{
		Status: 400,
		Errors: []error{
			assert.AnError,
		},
		StartTime: time.Now(),
	}

	data, err := serializeAuctionObject(ao)

	require.NoError(t, err)
	assert.NotEmpty(t, data)

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Equal(t, float64(400), result["Status"])
	assert.NotNil(t, result["Errors"])
}
