package auctionaudit

import (
	"fmt"

	"github.com/prebid/prebid-server/v3/analytics"
	"github.com/prebid/prebid-server/v3/errortypes"
	"github.com/prebid/prebid-server/v3/util/jsonutil"
	"google.golang.org/protobuf/proto"
)

func buildAuctionEvent(ao *analytics.AuctionObject, environment, accountID, domain, appBundle string, mediaTypes []MediaType) *AuctionEvent {
	event := &AuctionEvent{
		Environment: environment,
		TimestampMs: ao.StartTime.UnixMilli(),
		Status:      int32(ao.Status),
		AccountId:   accountID,
		Domain:      domain,
		AppBundle:   appBundle,
		MediaTypes:  mediaTypes,
	}

	if ao.RequestWrapper != nil && ao.RequestWrapper.BidRequest != nil {
		if reqJSON, err := jsonutil.Marshal(ao.RequestWrapper.BidRequest); err == nil {
			event.BidRequest = string(reqJSON)
		}
	}

	if ao.Response != nil {
		if respJSON, err := jsonutil.Marshal(ao.Response); err == nil {
			event.BidResponse = string(respJSON)
		}
	}

	if len(ao.Errors) > 0 {
		event.Errors = buildAuctionErrors(ao.Errors)
	}

	return event
}

func buildAuctionErrors(errs []error) []*AuctionError {
	result := make([]*AuctionError, len(errs))
	for i, e := range errs {
		result[i] = &AuctionError{
			Message: e.Error(),
			Code:    int32(errortypes.ReadCode(e)),
		}
	}
	return result
}

func serializeToProtobuf(event *AuctionEvent) ([]byte, error) {
	data, err := proto.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal protobuf: %w", err)
	}
	return data, nil
}
