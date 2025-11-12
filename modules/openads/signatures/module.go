package signatures

import (
	"context"
	"encoding/json"

	"github.com/prebid/prebid-server/v3/hooks/hookexecution"
	"github.com/prebid/prebid-server/v3/hooks/hookstage"
	"github.com/prebid/prebid-server/v3/modules/moduledeps"
	"github.com/tidwall/sjson"
)

const (
	NbrCodeServiceUnavailable = 100
)

type OpenAdsExt struct {
	Version int           `json:"version"`
	IntSigs []interface{} `json:"int_sigs"`
}

type signatureRequest struct {
	RequestBody   interface{} `json:"requestBody"`
	DemandSources []string    `json:"demandSources"`
}

func Builder(rawConfig json.RawMessage, _ moduledeps.ModuleDeps) (interface{}, error) {
	cfg, err := NewConfig(rawConfig)
	if err != nil {
		return nil, err
	}

	fetcher, err := newFetcher(cfg)
	if err != nil {
		return nil, err
	}

	return Module{
		cfg:     cfg,
		fetcher: fetcher,
	}, nil
}

type Module struct {
	cfg     *Config
	fetcher SignatureFetcher
}

func (m Module) HandleBidderRequestHook(
	ctx context.Context,
	_ hookstage.ModuleInvocationContext,
	payload hookstage.BidderRequestPayload,
) (hookstage.HookResult[hookstage.BidderRequestPayload], error) {
	result := hookstage.HookResult[hookstage.BidderRequestPayload]{}

	if payload.Request == nil || payload.Request.BidRequest == nil {
		return result, hookexecution.NewFailure("payload contains a nil bid request")
	}

	var extBytes []byte
	if payload.Request.BidRequest.Ext != nil {
		extBytes = payload.Request.BidRequest.Ext
	} else {
		extBytes = []byte("{}")
	}

	request := signatureRequest{
		RequestBody:   payload.Request.BidRequest,
		DemandSources: []string{payload.Bidder},
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		if m.cfg.RejectOnFailure {
			result.Reject = true
			result.NbrCode = NbrCodeServiceUnavailable
			return result, hookexecution.NewFailure("failed to marshal bid request: %v", err)
		}
		return m.setOpenAdsExt(extBytes, []interface{}{}, result, hookexecution.NewFailure("failed to marshal bid request: %v", err))
	}

	signatures, err := m.fetcher.Fetch(ctx, requestBody)
	if err != nil {
		if m.cfg.RejectOnFailure {
			result.Reject = true
			result.NbrCode = NbrCodeServiceUnavailable
			return result, hookexecution.NewFailure("sidecar fetch: %v", err)
		}
		return m.setOpenAdsExt(extBytes, []interface{}{}, result, hookexecution.NewFailure("sidecar fetch: %v", err))
	}

	return m.setOpenAdsExt(extBytes, signatures, result, nil)
}

func (m Module) setOpenAdsExt(
	extBytes []byte,
	signatures []interface{},
	result hookstage.HookResult[hookstage.BidderRequestPayload],
	hookErr error,
) (hookstage.HookResult[hookstage.BidderRequestPayload], error) {
	openadsExt := OpenAdsExt{
		Version: m.cfg.Version,
		IntSigs: signatures,
	}

	newExt, err := sjson.SetBytes(extBytes, "openads", openadsExt)
	if err != nil {
		if m.cfg.RejectOnFailure {
			result.Reject = true
			result.NbrCode = NbrCodeServiceUnavailable
		}
		return result, hookexecution.NewFailure("failed to set ext.openads: %v", err)
	}

	result.ChangeSet.AddMutation(func(payload hookstage.BidderRequestPayload) (hookstage.BidderRequestPayload, error) {
		payload.Request.BidRequest.Ext = newExt
		return payload, nil
	}, hookstage.MutationUpdate, "bidrequest", "ext.openads")

	return result, hookErr
}
