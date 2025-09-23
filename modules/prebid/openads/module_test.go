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
		initialExt    map[string]json.RawMessage
		expectedValue string
		expectError   bool
	}{
		{
			name:          "add openads to empty ext",
			initialExt:    nil,
			expectedValue: "1",
		},
		{
			name: "add openads to existing ext",
			initialExt: map[string]json.RawMessage{
				"prebid": json.RawMessage(`{"debug": true}`),
			},
			expectedValue: "1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create module
			module := Module{}

			// Create test request
			bidRequest := &openrtb2.BidRequest{
				ID: "test-request",
				Imp: []openrtb2.Imp{
					{ID: "test-imp"},
				},
			}

			// Set up request wrapper with initial ext
			requestWrapper := &openrtb_ext.RequestWrapper{BidRequest: bidRequest}
			if tt.initialExt != nil {
				reqExt, _ := requestWrapper.GetRequestExt()
				reqExt.SetExt(tt.initialExt)
			}

			// Create payload
			payload := hookstage.BidderRequestPayload{
				Request: requestWrapper,
				Bidder:  "testbidder",
			}

			// Execute hook
			_, err := module.HandleBidderRequestHook(
				context.Background(),
				hookstage.ModuleInvocationContext{},
				payload,
			)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			// The payload should be modified in place, so we can check the original

			// Verify openads was added
			reqExt, err := requestWrapper.GetRequestExt()
			require.NoError(t, err)

			extMap := reqExt.GetExt()
			require.NotNil(t, extMap)

			openadsValue, exists := extMap["openads"]
			require.True(t, exists, "openads field should exist")
			assert.Equal(t, tt.expectedValue, string(openadsValue))

			// Verify existing fields are preserved
			for key, expectedValue := range tt.initialExt {
				actualValue, exists := extMap[key]
				assert.True(t, exists, "existing field %s should be preserved", key)
				assert.Equal(t, string(expectedValue), string(actualValue))
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
