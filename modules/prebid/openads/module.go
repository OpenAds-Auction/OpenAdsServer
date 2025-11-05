package openads

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/prebid/prebid-server/v3/hooks/hookexecution"
	"github.com/prebid/prebid-server/v3/hooks/hookstage"
	"github.com/prebid/prebid-server/v3/modules/moduledeps"
	"github.com/tidwall/sjson"
)

func Builder(rawConfig json.RawMessage, _ moduledeps.ModuleDeps) (interface{}, error) {
	return Module{}, nil
}

type Module struct{}

type OpenAdsExt struct {
	Ver string `json:"ver"`
}

// Utilize bid request hook to add ext.openads to all outgoing requests
func (m Module) HandleBidderRequestHook(
	_ context.Context,
	miCtx hookstage.ModuleInvocationContext,
	payload hookstage.BidderRequestPayload,
) (hookstage.HookResult[hookstage.BidderRequestPayload], error) {
	result := hookstage.HookResult[hookstage.BidderRequestPayload]{}

	if payload.Request == nil || payload.Request.BidRequest == nil {
		return result, hookexecution.NewFailure("payload contains a nil bid request")
	}

	// Create ext if it doesn't exist
	var extBytes []byte
	if payload.Request.BidRequest.Ext != nil {
		extBytes = payload.Request.BidRequest.Ext
	} else {
		extBytes = []byte("{}")
	}
	
	newExt, err := sjson.SetBytes(extBytes, "openads", OpenAdsExt{Ver: "1"})
	if err != nil {
		return hookstage.HookResult[hookstage.BidderRequestPayload]{},
			fmt.Errorf("failed to set openads field: %w", err)
	}

	result.ChangeSet.AddMutation(func(payload hookstage.BidderRequestPayload) (hookstage.BidderRequestPayload, error) {
		payload.Request.BidRequest.Ext = newExt
		return payload, nil
	}, hookstage.MutationUpdate, "bidrequest", "ext.openads")

	return result, nil
}
