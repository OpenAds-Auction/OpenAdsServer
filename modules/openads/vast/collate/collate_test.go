package collate

import (
	"fmt"
	"strings"
	"testing"

	"github.com/prebid/prebid-server/v3/metrics"
	metricsConfig "github.com/prebid/prebid-server/v3/metrics/config"
	"github.com/prebid/prebid-server/v3/openrtb_ext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type collateMetricsMock struct {
	metricsConfig.NilMetricsEngine
	versionMismatch map[openrtb_ext.BidderName]int
	missingMetadata map[openrtb_ext.BidderName]int
}

func newCollateMetricsMock() *collateMetricsMock {
	return &collateMetricsMock{
		versionMismatch: make(map[openrtb_ext.BidderName]int),
		missingMetadata: make(map[openrtb_ext.BidderName]int),
	}
}

func (m *collateMetricsMock) RecordCollateVastVersionMismatch(adapterName openrtb_ext.BidderName) {
	m.versionMismatch[adapterName]++
}

func (m *collateMetricsMock) RecordCollateVastMissingMetadata(adapterName openrtb_ext.BidderName) {
	m.missingMetadata[adapterName]++
}

func nilMetrics() metrics.MetricsEngine {
	return &metricsConfig.NilMetricsEngine{}
}

func TestVAST_SingleBidWithMetadata(t *testing.T) {
	bids := []BidInput{
		{
			BidID: "bid-1", ImpID: "imp-1", Seat: "ttd", Price: 12.50,
			ADomain: []string{"example.com"}, Cat: []string{"IAB1-1"},
			AdapterName: openrtb_ext.BidderTheTradeDesk,
			AdM: `<VAST version="3.0"><Ad id="ad1"><InLine>` +
				`<AdTitle>Test Ad</AdTitle>` +
				`<Advertiser>example.com</Advertiser>` +
				`<Pricing model="CPM" currency="USD">12.50</Pricing>` +
				`<Category authority="iab">IAB1</Category>` +
				`<Creatives><Creative><Linear><Duration>00:00:15</Duration></Linear></Creative></Creatives>` +
				`</InLine></Ad></VAST>`,
		},
	}

	me := newCollateMetricsMock()
	result := VAST(bids, me)

	assert.Empty(t, result.Errors)
	assert.NotEmpty(t, result.VastXML)

	assert.Contains(t, result.VastXML, `<VAST version="3.0">`)
	assert.Contains(t, result.VastXML, `<Ad id="ad1">`)
	assert.Contains(t, result.VastXML, "<AdTitle>Test Ad</AdTitle>")
	assert.Contains(t, result.VastXML, "<Duration>00:00:15</Duration>")
	assert.Contains(t, result.VastXML, "<Advertiser>example.com</Advertiser>")
	assert.Contains(t, result.VastXML, `<Pricing model="CPM" currency="USD">12.50</Pricing>`)
	assert.Empty(t, me.versionMismatch)
	assert.Empty(t, me.missingMetadata)
}

func TestVAST_SingleBidMissingAdvertiser(t *testing.T) {
	bids := []BidInput{
		{
			BidID: "bid-1", ImpID: "imp-1", AdapterName: "dsp-a",
			AdM: `<VAST version="3.0"><Ad><InLine>` +
				`<Pricing model="CPM" currency="USD">5.00</Pricing>` +
				`<Creatives></Creatives>` +
				`</InLine></Ad></VAST>`,
		},
	}

	me := newCollateMetricsMock()
	result := VAST(bids, me)

	assert.Empty(t, result.VastXML)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Error(), "missing required Advertiser, Pricing, or Category")
	assert.Equal(t, 1, me.missingMetadata["dsp-a"])
}

func TestVAST_SingleBidMissingPricing(t *testing.T) {
	bids := []BidInput{
		{
			BidID: "bid-1", ImpID: "imp-1", AdapterName: "dsp-b",
			AdM: `<VAST version="3.0"><Ad><InLine>` +
				`<Advertiser>example.com</Advertiser>` +
				`<Creatives></Creatives>` +
				`</InLine></Ad></VAST>`,
		},
	}

	me := newCollateMetricsMock()
	result := VAST(bids, me)

	assert.Empty(t, result.VastXML)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Error(), "missing required Advertiser, Pricing, or Category")
	assert.Equal(t, 1, me.missingMetadata["dsp-b"])
}

func TestVAST_SingleBidMissingCategory(t *testing.T) {
	bids := []BidInput{
		{
			BidID: "bid-1", ImpID: "imp-1", AdapterName: "dsp-c",
			AdM: `<VAST version="3.0"><Ad><InLine>` +
				`<Advertiser>example.com</Advertiser>` +
				`<Pricing model="CPM" currency="USD">5.00</Pricing>` +
				`<Creatives></Creatives>` +
				`</InLine></Ad></VAST>`,
		},
	}

	me := newCollateMetricsMock()
	result := VAST(bids, me)

	assert.Empty(t, result.VastXML)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Error(), "missing required Advertiser, Pricing, or Category")
	assert.Equal(t, 1, me.missingMetadata["dsp-c"])
}

func TestVAST_MultipleBidsSameVersion(t *testing.T) {
	bids := []BidInput{
		{
			BidID: "bid-1", ImpID: "imp-1", AdapterName: "seat-a",
			AdM: `<VAST version="3.0"><Ad id="a1"><InLine>` +
				`<Advertiser>a.com</Advertiser>` +
				`<Pricing model="CPM" currency="USD">10.00</Pricing>` +
				`<Category authority="iab">IAB1</Category>` +
				`<Creatives></Creatives></InLine></Ad></VAST>`,
		},
		{
			BidID: "bid-2", ImpID: "imp-2", AdapterName: "seat-b",
			AdM: `<VAST version="3.0"><Ad id="a2"><InLine>` +
				`<Advertiser>b.com</Advertiser>` +
				`<Pricing model="CPM" currency="USD">8.75</Pricing>` +
				`<Category authority="iab">IAB1</Category>` +
				`<Creatives></Creatives></InLine></Ad></VAST>`,
		},
	}

	me := newCollateMetricsMock()
	result := VAST(bids, me)

	assert.Empty(t, result.Errors)
	assert.Contains(t, result.VastXML, `<Ad id="a1">`)
	assert.Contains(t, result.VastXML, `<Ad id="a2">`)
	assert.Contains(t, result.VastXML, "<Advertiser>a.com</Advertiser>")
	assert.Contains(t, result.VastXML, "<Advertiser>b.com</Advertiser>")
	assert.Empty(t, me.versionMismatch)
	assert.Empty(t, me.missingMetadata)
}

func TestVAST_VersionMismatch(t *testing.T) {
	bids := []BidInput{
		{
			BidID: "bid-1", ImpID: "imp-1", AdapterName: "seat-a",
			AdM: `<VAST version="3.0"><Ad><InLine>` +
				`<Advertiser>a.com</Advertiser>` +
				`<Pricing model="CPM" currency="USD">10.00</Pricing>` +
				`<Category authority="iab">IAB1</Category>` +
				`<Creatives></Creatives></InLine></Ad></VAST>`,
		},
		{
			BidID: "bid-2", ImpID: "imp-2", AdapterName: "seat-b",
			AdM: `<VAST version="4.0"><Ad><InLine>` +
				`<Advertiser>b.com</Advertiser>` +
				`<Pricing model="CPM" currency="USD">8.00</Pricing>` +
				`<Category authority="iab">IAB1</Category>` +
				`<Creatives></Creatives></InLine></Ad></VAST>`,
		},
	}

	me := newCollateMetricsMock()
	result := VAST(bids, me)

	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Error(), "version")
	assert.Equal(t, 1, me.versionMismatch["seat-a"])

	assert.Contains(t, result.VastXML, `<VAST version="4.0">`)
	assert.Contains(t, result.VastXML, "<Advertiser>b.com</Advertiser>")
	assert.NotContains(t, result.VastXML, "<Advertiser>a.com</Advertiser>")
}

func TestVAST_NURLOnlyPassedViaMakeVAST(t *testing.T) {
	// NURL-only bids are wrapped by makeVAST at the call site, so VAST
	// receives them as AdM. The generated wrapper lacks Advertiser/Pricing/Category → discarded.
	wrappedNURL := `<VAST version="3.0"><Ad><Wrapper>` +
		`<AdSystem>prebid.org wrapper</AdSystem>` +
		`<VASTAdTagURI><![CDATA[https://example.com/vast-redirect?id=123]]></VASTAdTagURI>` +
		`<Impression></Impression><Creatives></Creatives>` +
		`</Wrapper></Ad></VAST>`

	bids := []BidInput{
		{
			BidID: "bid-nurl", ImpID: "imp-nurl", AdapterName: "nurl-seat",
			AdM: wrappedNURL,
		},
	}

	me := newCollateMetricsMock()
	result := VAST(bids, me)

	assert.Empty(t, result.VastXML)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Error(), "missing required Advertiser, Pricing, or Category")
	assert.Equal(t, 1, me.missingMetadata["nurl-seat"])
}

func TestVAST_MalformedXML(t *testing.T) {
	bids := []BidInput{
		{
			BidID: "bid-bad", ImpID: "imp-bad", AdapterName: "seat",
			AdM: `<VAST version="3.0"><Ad><InLine>not closed properly`,
		},
	}

	me := newCollateMetricsMock()
	result := VAST(bids, me)

	assert.Empty(t, result.VastXML)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Error(), "malformed VAST XML")
	assert.Empty(t, me.versionMismatch)
	assert.Empty(t, me.missingMetadata)
}

func TestVAST_EmptyAdM(t *testing.T) {
	bids := []BidInput{
		{BidID: "bid-nothing", ImpID: "imp-nothing", AdapterName: "seat"},
	}

	me := newCollateMetricsMock()
	result := VAST(bids, me)

	assert.Empty(t, result.VastXML)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Error(), "AdM is empty")
	assert.Empty(t, me.versionMismatch)
	assert.Empty(t, me.missingMetadata)
}

func TestVAST_AdWithNeitherInLineNorWrapper(t *testing.T) {
	bids := []BidInput{
		{
			BidID: "bid-bare", ImpID: "imp-bare", AdapterName: "seat",
			AdM: `<VAST version="3.0"><Ad id="bare"></Ad></VAST>`,
		},
	}

	me := newCollateMetricsMock()
	result := VAST(bids, me)

	assert.Empty(t, result.VastXML)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Error(), "neither InLine nor Wrapper")
}

func TestVAST_PreservesAdAttributes(t *testing.T) {
	bids := []BidInput{
		{
			BidID: "bid-1", ImpID: "imp-1", AdapterName: "seat",
			AdM: `<VAST version="3.0"><Ad id="a1" sequence="1" conditionalAd="false"><InLine>` +
				`<Advertiser>x.com</Advertiser>` +
				`<Pricing model="CPM" currency="USD">5.00</Pricing>` +
				`<Category authority="iab">IAB1</Category>` +
				`<Creatives></Creatives></InLine></Ad></VAST>`,
		},
	}

	result := VAST(bids, nilMetrics())

	assert.Empty(t, result.Errors)
	assert.Contains(t, result.VastXML, `sequence="1"`)
	assert.Contains(t, result.VastXML, `conditionalAd="false"`)
}

func TestVAST_PreservesOriginalXMLExactly(t *testing.T) {
	original := `<InLine>` +
		`<AdSystem>custom-sys</AdSystem>` +
		`<Advertiser>exact.com</Advertiser>` +
		`<Pricing model="CPC" currency="EUR">5.00</Pricing>` +
		`<Category authority="iab">IAB1</Category>` +
		`<Creatives><Creative sequence="1"><Linear><Duration>00:00:30</Duration></Linear></Creative></Creatives>` +
		`</InLine>`
	bids := []BidInput{
		{
			BidID: "bid-exact", ImpID: "imp-exact", AdapterName: "seat",
			AdM: `<VAST version="3.0"><Ad id="e1">` + original + `</Ad></VAST>`,
		},
	}

	me := newCollateMetricsMock()
	result := VAST(bids, me)

	assert.Empty(t, result.Errors)
	assert.Contains(t, result.VastXML, original)
}

func TestVAST_MultipleAdsInOneBid_MixedMetadata(t *testing.T) {
	bids := []BidInput{
		{
			BidID: "bid-multi", ImpID: "imp-multi", AdapterName: "multi-seat",
			AdM: `<VAST version="3.0">` +
				`<Ad id="m1"><InLine><Advertiser>good.com</Advertiser><Pricing model="CPM" currency="USD">6.00</Pricing><Category authority="iab">IAB1</Category><Creatives></Creatives></InLine></Ad>` +
				`<Ad id="m2"><InLine><Creatives></Creatives></InLine></Ad>` +
				`</VAST>`,
		},
	}

	me := newCollateMetricsMock()
	result := VAST(bids, me)

	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Error(), "missing required Advertiser, Pricing, or Category")
	assert.Equal(t, 1, me.missingMetadata["multi-seat"])

	assert.Contains(t, result.VastXML, `<Ad id="m1">`)
	assert.NotContains(t, result.VastXML, `<Ad id="m2">`)
}

func TestVAST_AllBidsDiscarded(t *testing.T) {
	bids := []BidInput{
		{
			BidID: "bid-1", ImpID: "imp-1", AdapterName: "seat-a",
			AdM: `<VAST version="3.0"><Ad><InLine><Creatives></Creatives></InLine></Ad></VAST>`,
		},
		{
			BidID: "bid-2", ImpID: "imp-2", AdapterName: "seat-b",
			AdM: `<VAST version="3.0"><Ad><InLine><Creatives></Creatives></InLine></Ad></VAST>`,
		},
	}

	me := newCollateMetricsMock()
	result := VAST(bids, me)

	assert.Empty(t, result.VastXML)
	assert.Len(t, result.Errors, 2)
	assert.Equal(t, 1, me.missingMetadata["seat-a"])
	assert.Equal(t, 1, me.missingMetadata["seat-b"])
}

func TestVAST_EmptyInput(t *testing.T) {
	result := VAST(nil, nilMetrics())
	assert.Empty(t, result.VastXML)
	assert.Nil(t, result.Errors)

	result2 := VAST([]BidInput{}, nilMetrics())
	assert.Empty(t, result2.VastXML)
	assert.Nil(t, result2.Errors)
}

func TestVAST_SingleVersionUsedInOutput(t *testing.T) {
	bids := []BidInput{
		{
			BidID: "bid-1", ImpID: "imp-1", AdapterName: "seat",
			AdM: `<VAST version="4.1"><Ad><InLine>` +
				`<Advertiser>x.com</Advertiser>` +
				`<Pricing model="CPM" currency="USD">1.00</Pricing>` +
				`<Category authority="iab">IAB1</Category>` +
				`<Creatives></Creatives></InLine></Ad></VAST>`,
		},
	}

	result := VAST(bids, nilMetrics())
	assert.Empty(t, result.Errors)
	assert.True(t, strings.HasPrefix(result.VastXML, `<VAST version="4.1">`))
}

func TestVAST_HighestVersionWins(t *testing.T) {
	validAd := func(version, advertiser string) string {
		return fmt.Sprintf(`<VAST version="%s"><Ad><InLine>`+
			`<Advertiser>%s</Advertiser>`+
			`<Pricing model="CPM" currency="USD">5.00</Pricing>`+
			`<Category authority="iab">IAB1</Category>`+
			`<Creatives></Creatives></InLine></Ad></VAST>`, version, advertiser)
	}

	bids := []BidInput{
		{BidID: "v3-1", ImpID: "imp-1", AdapterName: "seat-a", AdM: validAd("3.0", "a.com")},
		{BidID: "v4-1", ImpID: "imp-2", AdapterName: "seat-b", AdM: validAd("4.0", "b.com")},
		{BidID: "v4-2", ImpID: "imp-3", AdapterName: "seat-c", AdM: validAd("4.0", "c.com")},
	}

	me := newCollateMetricsMock()
	result := VAST(bids, me)

	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Error(), "v3-1")
	assert.Contains(t, result.Errors[0].Error(), `version "3.0" does not match target "4.0"`)
	assert.Equal(t, 1, me.versionMismatch["seat-a"])

	assert.Contains(t, result.VastXML, `<VAST version="4.0">`)
	assert.Contains(t, result.VastXML, "<Advertiser>b.com</Advertiser>")
	assert.Contains(t, result.VastXML, "<Advertiser>c.com</Advertiser>")
	assert.NotContains(t, result.VastXML, "<Advertiser>a.com</Advertiser>")
}

func TestVAST_HighestVersionWins_TieGoesToHigher(t *testing.T) {
	validAd := func(version, advertiser string) string {
		return fmt.Sprintf(`<VAST version="%s"><Ad><InLine>`+
			`<Advertiser>%s</Advertiser>`+
			`<Pricing model="CPM" currency="USD">5.00</Pricing>`+
			`<Category authority="iab">IAB1</Category>`+
			`<Creatives></Creatives></InLine></Ad></VAST>`, version, advertiser)
	}

	bids := []BidInput{
		{BidID: "v4-1", ImpID: "imp-1", AdapterName: "seat-a", AdM: validAd("4.0", "a.com")},
		{BidID: "v3-1", ImpID: "imp-2", AdapterName: "seat-b", AdM: validAd("3.0", "b.com")},
	}

	me := newCollateMetricsMock()
	result := VAST(bids, me)

	assert.Contains(t, result.VastXML, `<VAST version="4.0">`)
	assert.Contains(t, result.VastXML, "<Advertiser>a.com</Advertiser>")
	assert.NotContains(t, result.VastXML, "<Advertiser>b.com</Advertiser>")
	assert.Equal(t, 1, me.versionMismatch["seat-b"])
}

func TestVAST_WrapperWithMetadata(t *testing.T) {
	bids := []BidInput{
		{
			BidID: "bid-w1", ImpID: "imp-w1", AdapterName: "dsp-a",
			AdM: `<VAST version="3.0"><Ad><Wrapper>` +
				`<Advertiser>advertiser.com</Advertiser>` +
				`<Pricing model="CPM" currency="USD">5.00</Pricing>` +
				`<Category authority="iab">IAB1</Category>` +
				`<VASTAdTagURI><![CDATA[https://vast.example.com/tag]]></VASTAdTagURI>` +
				`<Impression><![CDATA[https://track.example.com/imp]]></Impression>` +
				`</Wrapper></Ad></VAST>`,
		},
	}

	me := newCollateMetricsMock()
	result := VAST(bids, me)

	assert.Empty(t, result.Errors)
	assert.NotEmpty(t, result.VastXML)
	assert.Contains(t, result.VastXML, "<Wrapper>")
	assert.Contains(t, result.VastXML, "<Advertiser>advertiser.com</Advertiser>")
	assert.Contains(t, result.VastXML, `<Pricing model="CPM" currency="USD">5.00</Pricing>`)
	assert.Empty(t, me.missingMetadata)
}

func TestVAST_MixOfGoodAndBad(t *testing.T) {
	bids := []BidInput{
		{
			BidID: "good-1", ImpID: "imp-1", AdapterName: "seat-good",
			AdM: `<VAST version="3.0"><Ad id="g1"><InLine>` +
				`<Advertiser>good.com</Advertiser>` +
				`<Pricing model="CPM" currency="USD">10.00</Pricing>` +
				`<Category authority="iab">IAB1</Category>` +
				`<Creatives></Creatives></InLine></Ad></VAST>`,
		},
		{
			BidID: "bad-1", ImpID: "imp-2", AdapterName: "seat-bad",
			AdM: `<VAST broken`,
		},
		{
			BidID: "empty-1", ImpID: "imp-3", AdapterName: "seat-empty",
		},
		{
			BidID: "good-2", ImpID: "imp-4", AdapterName: "seat-good2",
			AdM: `<VAST version="3.0"><Ad id="g2"><InLine>` +
				`<Advertiser>good2.com</Advertiser>` +
				`<Pricing model="CPM" currency="USD">7.00</Pricing>` +
				`<Category authority="iab">IAB1</Category>` +
				`<Creatives></Creatives></InLine></Ad></VAST>`,
		},
	}

	me := newCollateMetricsMock()
	result := VAST(bids, me)

	require.Len(t, result.Errors, 2)
	assert.Contains(t, result.Errors[0].Error(), "bad-1")
	assert.Contains(t, result.Errors[1].Error(), "empty-1")

	assert.NotEmpty(t, result.VastXML)
	assert.Equal(t, 2, strings.Count(result.VastXML, "<Ad "))
	assert.Contains(t, result.VastXML, "<Advertiser>good.com</Advertiser>")
	assert.Contains(t, result.VastXML, "<Advertiser>good2.com</Advertiser>")
}

func TestVAST_PriceFormattingPreserved(t *testing.T) {
	tests := []struct {
		price    string
		expected string
	}{
		{"12.50", "12.50"},
		{"0.00", "0.00"},
		{"100.00", "100.00"},
		{"3.33", "3.33"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("price_%s", tt.price), func(t *testing.T) {
			bids := []BidInput{
				{
					BidID: "bid-p", ImpID: "imp-p", AdapterName: "seat",
					AdM: fmt.Sprintf(`<VAST version="3.0"><Ad><InLine>`+
						`<Advertiser>x.com</Advertiser>`+
						`<Pricing model="CPM" currency="USD">%s</Pricing>`+
						`<Category authority="iab">IAB1</Category>`+
						`<Creatives></Creatives></InLine></Ad></VAST>`, tt.price),
				},
			}
			result := VAST(bids, nilMetrics())
			assert.Empty(t, result.Errors)
			assert.Contains(t, result.VastXML,
				fmt.Sprintf(`<Pricing model="CPM" currency="USD">%s</Pricing>`, tt.expected))
		})
	}
}
