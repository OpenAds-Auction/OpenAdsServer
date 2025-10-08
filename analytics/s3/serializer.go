package s3

import (
	"time"

	"github.com/prebid/openrtb/v20/openrtb2"
	"github.com/prebid/prebid-server/v3/analytics"
	"github.com/prebid/prebid-server/v3/config"
	"github.com/prebid/prebid-server/v3/hooks/hookexecution"
	"github.com/prebid/prebid-server/v3/openrtb_ext"
	"github.com/prebid/prebid-server/v3/util/jsonutil"
)

type logAuction struct {
	Status               int
	Errors               []error
	Request              *openrtb2.BidRequest
	Response             *openrtb2.BidResponse
	Account              *config.Account
	StartTime            time.Time
	HookExecutionOutcome []hookexecution.StageOutcome
	SeatNonBid           []openrtb_ext.SeatNonBid
}

type logAmp struct {
	Status               int
	Errors               []error
	Request              *openrtb2.BidRequest
	AuctionResponse      *openrtb2.BidResponse
	AmpTargetingValues   map[string]string
	Origin               string
	StartTime            time.Time
	HookExecutionOutcome []hookexecution.StageOutcome
	SeatNonBid           []openrtb_ext.SeatNonBid
}

type logVideo struct {
	Status        int
	Errors        []error
	Request       *openrtb2.BidRequest
	Response      *openrtb2.BidResponse
	VideoRequest  *openrtb_ext.BidRequestVideo
	VideoResponse *openrtb_ext.BidResponseVideo
	StartTime     time.Time
	SeatNonBid    []openrtb_ext.SeatNonBid
}

func serializeAuctionObject(ao *analytics.AuctionObject) ([]byte, error) {
	var request *openrtb2.BidRequest
	if ao.RequestWrapper != nil {
		request = ao.RequestWrapper.BidRequest
	}

	logEntry := &logAuction{
		Status:               ao.Status,
		Errors:               ao.Errors,
		Request:              request,
		Response:             ao.Response,
		Account:              ao.Account,
		StartTime:            ao.StartTime,
		HookExecutionOutcome: ao.HookExecutionOutcome,
		SeatNonBid:           ao.SeatNonBid,
	}

	return jsonutil.Marshal(logEntry)
}

func serializeAmpObject(ao *analytics.AmpObject) ([]byte, error) {
	var request *openrtb2.BidRequest
	if ao.RequestWrapper != nil {
		request = ao.RequestWrapper.BidRequest
	}

	logEntry := &logAmp{
		Status:               ao.Status,
		Errors:               ao.Errors,
		Request:              request,
		AuctionResponse:      ao.AuctionResponse,
		AmpTargetingValues:   ao.AmpTargetingValues,
		Origin:               ao.Origin,
		StartTime:            ao.StartTime,
		HookExecutionOutcome: ao.HookExecutionOutcome,
		SeatNonBid:           ao.SeatNonBid,
	}

	return jsonutil.Marshal(logEntry)
}

func serializeVideoObject(vo *analytics.VideoObject) ([]byte, error) {
	var request *openrtb2.BidRequest
	if vo.RequestWrapper != nil {
		request = vo.RequestWrapper.BidRequest
	}

	logEntry := &logVideo{
		Status:        vo.Status,
		Errors:        vo.Errors,
		Request:       request,
		Response:      vo.Response,
		VideoRequest:  vo.VideoRequest,
		VideoResponse: vo.VideoResponse,
		StartTime:     vo.StartTime,
		SeatNonBid:    vo.SeatNonBid,
	}

	return jsonutil.Marshal(logEntry)
}
