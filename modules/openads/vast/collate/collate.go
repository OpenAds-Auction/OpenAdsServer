package collate

import (
	"fmt"

	"github.com/beevik/etree"
	"github.com/prebid/prebid-server/v3/metrics"
	"github.com/prebid/prebid-server/v3/openrtb_ext"
)

type BidInput struct {
	BidID       string
	ImpID       string
	Seat        string
	Price       float64
	ADomain     []string
	Cat         []string
	AdM         string
	BidExp      int64
	AdapterName openrtb_ext.BidderName
}

type Result struct {
	VastXML string
	Errors  []error
}

func VAST(bids []BidInput, me metrics.MetricsEngine) Result {
	if len(bids) == 0 {
		return Result{}
	}

	type parsedBid struct {
		input   BidInput
		vast    *etree.Element
		version string
	}

	var parsed []parsedBid
	var errs []error

	for _, bid := range bids {
		if bid.AdM == "" {
			errs = append(errs, fmt.Errorf("bid %q (imp %q): AdM is empty, skipping", bid.BidID, bid.ImpID))
			continue
		}

		doc := etree.NewDocument()
		if err := doc.ReadFromString(bid.AdM); err != nil {
			errs = append(errs, fmt.Errorf("bid %q (imp %q): malformed VAST XML: %w", bid.BidID, bid.ImpID, err))
			continue
		}

		vast := doc.SelectElement("VAST")
		if vast == nil {
			errs = append(errs, fmt.Errorf("bid %q (imp %q): malformed VAST XML: missing VAST root element", bid.BidID, bid.ImpID))
			continue
		}

		version := vast.SelectAttrValue("version", "")
		parsed = append(parsed, parsedBid{input: bid, vast: vast, version: version})
	}

	if len(parsed) == 0 {
		return Result{Errors: errs}
	}

	// Pick the highest version string across all parsed bids.
	targetVersion := parsed[0].version
	for _, p := range parsed[1:] {
		if p.version > targetVersion {
			targetVersion = p.version
		}
	}

	var surviving []*etree.Element
	for _, p := range parsed {
		if p.version != targetVersion {
			me.RecordCollatedVastVersionMismatch(p.input.AdapterName)
			errs = append(errs, fmt.Errorf("bid %q (imp %q): VAST version %q does not match target %q, discarding", p.input.BidID, p.input.ImpID, p.version, targetVersion))
			continue
		}

		for _, ad := range p.vast.SelectElements("Ad") {
			child := ad.SelectElement("InLine")
			if child == nil {
				child = ad.SelectElement("Wrapper")
			}
			if child == nil {
				errs = append(errs, fmt.Errorf("bid %q (imp %q): <Ad> has neither InLine nor Wrapper, skipping", p.input.BidID, p.input.ImpID))
				continue
			}

			if child.SelectElement("Advertiser") == nil || child.SelectElement("Pricing") == nil || child.SelectElement("Category") == nil {
				me.RecordCollatedVastMissingMetadata(p.input.AdapterName)
				errs = append(errs, fmt.Errorf("bid %q (imp %q): <Ad> missing required Advertiser, Pricing, or Category metadata, discarding", p.input.BidID, p.input.ImpID))
				continue
			}

			surviving = append(surviving, ad.Copy())
		}
	}

	if len(surviving) == 0 {
		return Result{Errors: errs}
	}

	outDoc := etree.NewDocument()
	outVast := outDoc.CreateElement("VAST")
	outVast.CreateAttr("version", targetVersion)
	for _, ad := range surviving {
		outVast.AddChild(ad)
	}

	xml, err := outDoc.WriteToString()
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to serialize collated VAST: %w", err))
		return Result{Errors: errs}
	}

	return Result{
		VastXML: xml,
		Errors:  errs,
	}
}
