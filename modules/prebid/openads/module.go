package openads

import (
	"context"
	"encoding/json"

	"github.com/prebid/prebid-server/v3/hooks/hookexecution"
	"github.com/prebid/prebid-server/v3/hooks/hookstage"
	"github.com/prebid/prebid-server/v3/modules/moduledeps"
)

func Builder(rawConfig json.RawMessage, _ moduledeps.ModuleDeps) (interface{}, error) {
	return Module{}, nil
}

type Module struct{}

type OpenAdsExt struct {
	OpenAds int `json:"openads"`
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

	reqExt, err := payload.Request.GetRequestExt()
	if err != nil {
		return result, hookexecution.NewFailure("failed to get request ext: %s", err)
	}

	extMap := reqExt.GetExt()
	if extMap == nil {
		extMap = make(map[string]json.RawMessage)
	}

	openAdsStruct := OpenAdsExt{OpenAds: 1}
	openAdsBytes, err := json.Marshal(openAdsStruct)
	if err != nil {
		return result, hookexecution.NewFailure("failed to marshal openads: %s", err)
	}

	var tempMap map[string]json.RawMessage
	if err := json.Unmarshal(openAdsBytes, &tempMap); err != nil {
		return result, hookexecution.NewFailure("failed to unmarshal openads: %s", err)
	}
	extMap["openads"] = tempMap["openads"]

	reqExt.SetExt(extMap)

	return result, nil
}
