package openads

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/prebid/openrtb/v20/openrtb2"
	"github.com/prebid/prebid-server/v3/hooks/hookstage"
	"github.com/prebid/prebid-server/v3/openrtb_ext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleBidderRequestHook(t *testing.T) {
	tests := []struct {
		name          string
		initialExt    json.RawMessage
		expectedValue string
		expectError   bool
	}{
		{
			name:          "add openads to nil ext",
			initialExt:    nil,
			expectedValue: "1",
		},
		{
			name:          "add openads to empty ext",
			initialExt:    json.RawMessage(`{}`),
			expectedValue: "1",
		},
		{
			name:          "add openads to existing ext",
			initialExt:    json.RawMessage(`{"prebid": {"debug": true}}`),
			expectedValue: "1",
		},
		{
			name:          "overwrite existing openads field",
			initialExt:    json.RawMessage(`{"openads": {"ver": "0"},"prebid": {"debug": true}}`),
			expectedValue: "1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := Module{}

			bidRequest := &openrtb2.BidRequest{
				ID: "test-request",
				Imp: []openrtb2.Imp{
					{ID: "test-imp"},
				},
				Ext: tt.initialExt,
			}

			requestWrapper := &openrtb_ext.RequestWrapper{BidRequest: bidRequest}

			payload := hookstage.BidderRequestPayload{
				Request: requestWrapper,
				Bidder:  "testbidder",
			}

			result, err := module.HandleBidderRequestHook(
				context.Background(),
				hookstage.ModuleInvocationContext{},
				payload,
			)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Apply the mutations to get the final result
			finalPayload := payload
			for _, mutation := range result.ChangeSet.Mutations() {
				finalPayload, err = mutation.Apply(finalPayload)
				require.NoError(t, err)
			}

			// Verify openads field was added/updated
			var extMap map[string]interface{}
			err = json.Unmarshal(finalPayload.Request.BidRequest.Ext, &extMap)
			require.NoError(t, err)

			openAdsExt, exists := extMap["openads"]
			require.True(t, exists, "openads obj should exist")

			openAdsVer, exists := openAdsExt.(map[string]interface{})["ver"]
			require.True(t, exists, "openads.ver should exist")

			openAdsVerValue, exists := openAdsVer.(string)
			require.True(t, exists, "openads.ver should be of type string")

			assert.Equal(t, tt.expectedValue, openAdsVerValue)

			// Verify other fields are preserved if they existed
			if len(tt.initialExt) > 2 {
				var originalExt map[string]interface{}
				json.Unmarshal(tt.initialExt, &originalExt)

				for key, expectedValue := range originalExt {
					if key != "openads" {
						actualValue, exists := extMap[key]
						assert.True(t, exists, "existing field %s should be preserved", key)
						assert.Equal(t, expectedValue, actualValue)
					}
				}
			}
		})
	}
}

func TestHandleBidderRequestHook_NilRequest(t *testing.T) {
	module := Module{}

	payload := hookstage.BidderRequestPayload{
		Request: nil,
		Bidder:  "testbidder",
	}

	_, err := module.HandleBidderRequestHook(
		context.Background(),
		hookstage.ModuleInvocationContext{},
		payload,
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "payload contains a nil bid request")
}

func TestHandleBidderRequestHook_MutationTracking(t *testing.T) {
	module := Module{}

	bidRequest := &openrtb2.BidRequest{
		ID:  "test-request",
		Ext: json.RawMessage(`{"existing": "value"}`),
	}

	requestWrapper := &openrtb_ext.RequestWrapper{BidRequest: bidRequest}

	payload := hookstage.BidderRequestPayload{
		Request: requestWrapper,
		Bidder:  "testbidder",
	}

	result, err := module.HandleBidderRequestHook(
		context.Background(),
		hookstage.ModuleInvocationContext{},
		payload,
	)

	require.NoError(t, err)

	// Verify mutation was added
	mutations := result.ChangeSet.Mutations()
	assert.Len(t, mutations, 1)

	mutation := mutations[0]
	modifiedPayload, err := mutation.Apply(payload)
	require.NoError(t, err)

	// Verify the mutation actually worked
	var extMap map[string]interface{}
	err = json.Unmarshal(modifiedPayload.Request.BidRequest.Ext, &extMap)
	require.NoError(t, err)

	openAdsExt, exists := extMap["openads"]
	assert.True(t, exists)

	openAdsVer, exists := openAdsExt.(map[string]interface{})["ver"]
	require.True(t, exists)

	openAdsVerValue, exists := openAdsVer.(string)
	require.True(t, exists)

	assert.Equal(t, "1", openAdsVerValue)
}
