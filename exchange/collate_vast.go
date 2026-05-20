package exchange

import (
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/prebid/prebid-server/v3/metrics"
	"github.com/prebid/prebid-server/v3/openrtb_ext"
)

type VastBidInput struct {
	BidID       string
	ImpID       string
	Seat        string
	Price       float64
	ADomain     []string
	Cat         []string
	AdM         string
	AdapterName openrtb_ext.BidderName
}

type CollatedVastOutput struct {
	VastXML string
	Errors  []error
}

type vastDoc struct {
	XMLName xml.Name  `xml:"VAST"`
	Version string    `xml:"version,attr"`
	Ads     []vastAd  `xml:"Ad"`
}

type vastAd struct {
	ID    string `xml:"id,attr,omitempty"`
	Inner string `xml:",innerxml"`
}

type adContent struct {
	InLine  *adChildContent `xml:"InLine"`
	Wrapper *adChildContent `xml:"Wrapper"`
}

type adChildContent struct {
	Advertiser *string      `xml:"Advertiser"`
	Pricing    *vastPricing `xml:"Pricing"`
}

type vastPricing struct {
	Model    string `xml:"model,attr,omitempty"`
	Currency string `xml:"currency,attr,omitempty"`
	Value    string `xml:",chardata"`
}

func CollateVAST(bids []VastBidInput, me metrics.MetricsEngine) CollatedVastOutput {
	if len(bids) == 0 {
		return CollatedVastOutput{}
	}

	var surviving []string
	var errs []error
	targetVersion := ""

	for _, bid := range bids {
		rawVAST := bid.AdM
		if rawVAST == "" {
			errs = append(errs, fmt.Errorf("bid %q (imp %q): AdM is empty, skipping", bid.BidID, bid.ImpID))
			continue
		}

		var doc vastDoc
		if err := xml.Unmarshal([]byte(rawVAST), &doc); err != nil {
			errs = append(errs, fmt.Errorf("bid %q (imp %q): malformed VAST XML: %w", bid.BidID, bid.ImpID, err))
			continue
		}

		if targetVersion == "" {
			targetVersion = doc.Version
		} else if doc.Version != targetVersion {
			me.RecordCollateVastVersionMismatch(bid.AdapterName)
			errs = append(errs, fmt.Errorf("bid %q (imp %q): VAST version %q does not match target %q, discarding", bid.BidID, bid.ImpID, doc.Version, targetVersion))
			continue
		}

		for _, ad := range doc.Ads {
			var content adContent
			if err := xml.Unmarshal([]byte("<Ad>"+ad.Inner+"</Ad>"), &content); err != nil {
				errs = append(errs, fmt.Errorf("bid %q (imp %q): <Ad> has neither InLine nor Wrapper, skipping", bid.BidID, bid.ImpID))
				continue
			}

			child := content.InLine
			if child == nil {
				child = content.Wrapper
			}
			if child == nil {
				errs = append(errs, fmt.Errorf("bid %q (imp %q): <Ad> has neither InLine nor Wrapper, skipping", bid.BidID, bid.ImpID))
				continue
			}

			if child.Advertiser == nil || child.Pricing == nil {
				me.RecordCollateVastMissingMetadata(bid.AdapterName)
				errs = append(errs, fmt.Errorf("bid %q (imp %q): <Ad> missing required Advertiser or Pricing metadata, discarding", bid.BidID, bid.ImpID))
				continue
			}

			var adTag string
			if ad.ID != "" {
				adTag = fmt.Sprintf(`<Ad id="%s">`, ad.ID)
			} else {
				adTag = "<Ad>"
			}
			surviving = append(surviving, adTag+ad.Inner+"</Ad>")
		}
	}

	if len(surviving) == 0 {
		return CollatedVastOutput{Errors: errs}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<VAST version="%s">`, targetVersion))
	for _, ad := range surviving {
		sb.WriteString(ad)
	}
	sb.WriteString("</VAST>")

	return CollatedVastOutput{
		VastXML: sb.String(),
		Errors:  errs,
	}
}
