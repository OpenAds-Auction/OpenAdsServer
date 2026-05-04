package openrtb_ext

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/prebid/prebid-server/v3/util/jsonutil"
	"github.com/prebid/prebid-server/v3/util/ptrutil"
	"github.com/stretchr/testify/assert"
)

func getFlag(in bool) *bool {
	return &in
}

func TestPriceFloorRulesGetEnforcePBS(t *testing.T) {
	tests := []struct {
		name   string
		floors *PriceFloorRules
		want   bool
	}{
		{
			name: "EnforcePBS_Enabled",
			floors: &PriceFloorRules{
				Enabled: getFlag(true),
				Enforcement: &PriceFloorEnforcement{
					EnforcePBS: getFlag(true),
				},
			},
			want: true,
		},
		{
			name: "EnforcePBS_NotProvided",
			floors: &PriceFloorRules{
				Enabled:     getFlag(true),
				Enforcement: &PriceFloorEnforcement{},
			},
			want: true,
		},
		{
			name: "EnforcePBS_Disabled",
			floors: &PriceFloorRules{
				Enabled: getFlag(true),
				Enforcement: &PriceFloorEnforcement{
					EnforcePBS: getFlag(false),
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.floors.GetEnforcePBS()
			assert.Equal(t, tt.want, got, tt.name)
		})
	}
}

func TestPriceFloorRulesGetFloorsSkippedFlag(t *testing.T) {
	tests := []struct {
		name   string
		floors *PriceFloorRules
		want   bool
	}{
		{
			name: "Skipped_true",
			floors: &PriceFloorRules{
				Enabled: getFlag(true),
				Skipped: getFlag(true),
			},
			want: true,
		},
		{
			name: "Skipped_false",
			floors: &PriceFloorRules{
				Enabled: getFlag(true),
				Skipped: getFlag(false),
			},
			want: false,
		},
		{
			name: "Skipped_NotProvided",
			floors: &PriceFloorRules{
				Enabled: getFlag(true),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.floors.GetFloorsSkippedFlag()
			assert.Equal(t, tt.want, got, tt.name)
		})
	}
}

func TestPriceFloorRulesGetEnforceRate(t *testing.T) {
	tests := []struct {
		name   string
		floors *PriceFloorRules
		want   int
	}{
		{
			name: "EnforceRate_100",
			floors: &PriceFloorRules{
				Enabled: getFlag(true),
				Enforcement: &PriceFloorEnforcement{
					EnforcePBS:  getFlag(true),
					EnforceRate: 100,
				},
			},
			want: 100,
		},
		{
			name: "EnforceRate_0",
			floors: &PriceFloorRules{
				Enabled: getFlag(true),
				Enforcement: &PriceFloorEnforcement{
					EnforcePBS:  getFlag(true),
					EnforceRate: 0,
				},
			},
			want: 0,
		},
		{
			name: "EnforceRate_NotProvided",
			floors: &PriceFloorRules{
				Enabled: getFlag(true),
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.floors.GetEnforceRate()
			assert.Equal(t, tt.want, got, tt.name)
		})
	}
}

func TestPriceFloorRulesGetEnforceDealsFlag(t *testing.T) {
	tests := []struct {
		name   string
		floors *PriceFloorRules
		want   bool
	}{
		{
			name: "FloorDeals_true",
			floors: &PriceFloorRules{
				Enabled: getFlag(true),
				Enforcement: &PriceFloorEnforcement{
					EnforcePBS:  getFlag(true),
					EnforceRate: 0,
					FloorDeals:  getFlag(true),
				},
			},
			want: true,
		},
		{
			name: "FloorDeals_false",
			floors: &PriceFloorRules{
				Enabled: getFlag(true),
				Enforcement: &PriceFloorEnforcement{
					EnforcePBS: getFlag(true),
					FloorDeals: getFlag(false),
				},
				Skipped: getFlag(false),
			},
			want: false,
		},
		{
			name: "FloorDeals_NotProvided",
			floors: &PriceFloorRules{
				Enabled: getFlag(true),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.floors.GetEnforceDealsFlag()
			assert.Equal(t, tt.want, got, tt.name)
		})
	}
}

func TestPriceFloorRulesGetEnabled(t *testing.T) {
	tests := []struct {
		name   string
		floors *PriceFloorRules
		want   bool
	}{
		{
			name: "Enabled_true",
			floors: &PriceFloorRules{
				Enabled: getFlag(true),
			},
			want: true,
		},
		{
			name: "Enabled_false",
			floors: &PriceFloorRules{
				Enabled: getFlag(false),
			},
			want: false,
		},
		{
			name:   "Enabled_NotProvided",
			floors: &PriceFloorRules{},
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.floors.GetEnabled()
			assert.Equal(t, tt.want, got, tt.name)
		})
	}
}

func TestPriceFloorRulesDeepCopy(t *testing.T) {
	type fields struct {
		FloorMin           float64
		FloorMinCur        string
		SkipRate           int
		Location           *PriceFloorEndpoint
		Data               *PriceFloorData
		Enforcement        *PriceFloorEnforcement
		Enabled            *bool
		Skipped            *bool
		FloorProvider      string
		FetchStatus        string
		PriceFloorLocation string
	}
	tests := []struct {
		name   string
		fields fields
	}{
		{
			name: "DeepCopy does not share same reference",
			fields: fields{
				FloorMin:    10,
				FloorMinCur: "INR",
				SkipRate:    0,
				Location: &PriceFloorEndpoint{
					URL: "https://test/floors",
				},
				Data: &PriceFloorData{
					Currency: "INR",
					SkipRate: 0,
					ModelGroups: []PriceFloorModelGroup{
						{
							Currency:    "INR",
							ModelWeight: ptrutil.ToPtr(1),
							SkipRate:    0,
							Values: map[string]float64{
								"banner|300x600|www.website5.com": 20,
								"*|*|*":                           50,
							},
							Schema: PriceFloorSchema{
								Fields:    []string{"mediaType", "size", "domain"},
								Delimiter: "|",
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pf := &PriceFloorRules{
				FloorMin:           tt.fields.FloorMin,
				FloorMinCur:        tt.fields.FloorMinCur,
				SkipRate:           tt.fields.SkipRate,
				Location:           tt.fields.Location,
				Data:               tt.fields.Data,
				Enforcement:        tt.fields.Enforcement,
				Enabled:            tt.fields.Enabled,
				Skipped:            tt.fields.Skipped,
				FloorProvider:      tt.fields.FloorProvider,
				FetchStatus:        tt.fields.FetchStatus,
				PriceFloorLocation: tt.fields.PriceFloorLocation,
			}
			got := pf.DeepCopy()
			if got == pf {
				t.Errorf("Rules reference are same")
			}
			if got.Data == pf.Data {
				t.Errorf("Floor data reference is same")
			}
		})
	}
}

func TestFloorRulesDeepCopy(t *testing.T) {
	type fields struct {
		FloorMin           float64
		FloorMinCur        string
		SkipRate           int
		Location           *PriceFloorEndpoint
		Data               *PriceFloorData
		Enforcement        *PriceFloorEnforcement
		Enabled            *bool
		Skipped            *bool
		FloorProvider      string
		FetchStatus        string
		PriceFloorLocation string
	}
	tests := []struct {
		name   string
		fields fields
		want   *PriceFloorRules
	}{
		{
			name: "Copy entire floors object",
			fields: fields{
				FloorMin:    10,
				FloorMinCur: "INR",
				SkipRate:    0,
				Location: &PriceFloorEndpoint{
					URL: "http://prebid.com/floor",
				},
				Data: &PriceFloorData{
					Currency:            "INR",
					SkipRate:            0,
					FloorsSchemaVersion: 2,
					ModelTimestamp:      123,
					ModelGroups: []PriceFloorModelGroup{
						{
							Currency:     "INR",
							ModelWeight:  ptrutil.ToPtr(50),
							ModelVersion: "version 1",
							SkipRate:     0,
							Schema: PriceFloorSchema{
								Fields:    []string{"a", "b", "c"},
								Delimiter: "|",
							},
							Values: map[string]float64{
								"*|*|*": 20,
							},
							Default: 1,
						},
					},
					FloorProvider: "prebid",
				},
				Enforcement: &PriceFloorEnforcement{
					EnforceJS:     ptrutil.ToPtr(true),
					EnforcePBS:    ptrutil.ToPtr(true),
					FloorDeals:    ptrutil.ToPtr(true),
					BidAdjustment: ptrutil.ToPtr(true),
					EnforceRate:   100,
				},
				Enabled:            ptrutil.ToPtr(true),
				Skipped:            ptrutil.ToPtr(false),
				FloorProvider:      "Prebid",
				FetchStatus:        "success",
				PriceFloorLocation: "fetch",
			},
			want: &PriceFloorRules{
				FloorMin:    10,
				FloorMinCur: "INR",
				SkipRate:    0,
				Location: &PriceFloorEndpoint{
					URL: "http://prebid.com/floor",
				},
				Data: &PriceFloorData{
					Currency:            "INR",
					SkipRate:            0,
					FloorsSchemaVersion: 2,
					ModelTimestamp:      123,
					ModelGroups: []PriceFloorModelGroup{
						{
							Currency:     "INR",
							ModelWeight:  ptrutil.ToPtr(50),
							ModelVersion: "version 1",
							SkipRate:     0,
							Schema: PriceFloorSchema{
								Fields:    []string{"a", "b", "c"},
								Delimiter: "|",
							},
							Values: map[string]float64{
								"*|*|*": 20,
							},
							Default: 1,
						},
					},
					FloorProvider: "prebid",
				},
				Enforcement: &PriceFloorEnforcement{
					EnforceJS:     ptrutil.ToPtr(true),
					EnforcePBS:    ptrutil.ToPtr(true),
					FloorDeals:    ptrutil.ToPtr(true),
					BidAdjustment: ptrutil.ToPtr(true),
					EnforceRate:   100,
				},
				Enabled:            ptrutil.ToPtr(true),
				Skipped:            ptrutil.ToPtr(false),
				FloorProvider:      "Prebid",
				FetchStatus:        "success",
				PriceFloorLocation: "fetch",
			},
		},
		{
			name: "Copy entire floors object",
			fields: fields{
				FloorMin:    10,
				FloorMinCur: "INR",
				SkipRate:    0,
				Location: &PriceFloorEndpoint{
					URL: "http://prebid.com/floor",
				},
				Data: nil,
				Enforcement: &PriceFloorEnforcement{
					EnforceJS:     ptrutil.ToPtr(true),
					EnforcePBS:    ptrutil.ToPtr(true),
					FloorDeals:    ptrutil.ToPtr(true),
					BidAdjustment: ptrutil.ToPtr(true),
					EnforceRate:   100,
				},
				Enabled:            ptrutil.ToPtr(true),
				Skipped:            ptrutil.ToPtr(false),
				FloorProvider:      "Prebid",
				FetchStatus:        "success",
				PriceFloorLocation: "fetch",
			},
			want: &PriceFloorRules{
				FloorMin:    10,
				FloorMinCur: "INR",
				SkipRate:    0,
				Location: &PriceFloorEndpoint{
					URL: "http://prebid.com/floor",
				},
				Data: nil,
				Enforcement: &PriceFloorEnforcement{
					EnforceJS:     ptrutil.ToPtr(true),
					EnforcePBS:    ptrutil.ToPtr(true),
					FloorDeals:    ptrutil.ToPtr(true),
					BidAdjustment: ptrutil.ToPtr(true),
					EnforceRate:   100,
				},
				Enabled:            ptrutil.ToPtr(true),
				Skipped:            ptrutil.ToPtr(false),
				FloorProvider:      "Prebid",
				FetchStatus:        "success",
				PriceFloorLocation: "fetch",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pf := &PriceFloorRules{
				FloorMin:           tt.fields.FloorMin,
				FloorMinCur:        tt.fields.FloorMinCur,
				SkipRate:           tt.fields.SkipRate,
				Location:           tt.fields.Location,
				Data:               tt.fields.Data,
				Enforcement:        tt.fields.Enforcement,
				Enabled:            tt.fields.Enabled,
				Skipped:            tt.fields.Skipped,
				FloorProvider:      tt.fields.FloorProvider,
				FetchStatus:        tt.fields.FetchStatus,
				PriceFloorLocation: tt.fields.PriceFloorLocation,
			}
			if got := pf.DeepCopy(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("PriceFloorRules.DeepCopy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFloorRuleDeepCopyNil(t *testing.T) {
	var priceFloorRule *PriceFloorRules
	got := priceFloorRule.DeepCopy()

	if got != nil {
		t.Errorf("PriceFloorRules.DeepCopy() = %v, want %v", got, nil)
	}
}

func TestExtImpUnmarshalOpenAdsAlias(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantNil   bool
		wantFloor float64
	}{
		{
			name:      "prebid key only",
			input:     `{"prebid":{"floors":{"floormin":1.5}}}`,
			wantFloor: 1.5,
		},
		{
			name:      "openads key only",
			input:     `{"openads":{"floors":{"floormin":2.5}}}`,
			wantFloor: 2.5,
		},
		{
			name:      "both keys, openads wins",
			input:     `{"prebid":{"floors":{"floormin":1}},"openads":{"floors":{"floormin":99}}}`,
			wantFloor: 99,
		},
		{
			name:    "neither key",
			input:   `{"bidder":{"x":1}}`,
			wantNil: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got ExtImp
			err := jsonutil.UnmarshalValid([]byte(tc.input), &got)
			assert.NoError(t, err)
			if tc.wantNil {
				assert.Nil(t, got.Prebid)
				return
			}
			if assert.NotNil(t, got.Prebid) {
				assert.Equal(t, tc.wantFloor, got.Prebid.Floors.FloorMin)
			}
		})
	}
}

func TestExtImpMarshalEmitsPrebid(t *testing.T) {
	in := []byte(`{"openads":{"floors":{"floormin":3}}}`)
	var ext ExtImp
	err := jsonutil.UnmarshalValid(in, &ext)
	assert.NoError(t, err)

	out, err := json.Marshal(&ext)
	assert.NoError(t, err)
	assert.Contains(t, string(out), `"prebid"`)
	assert.NotContains(t, string(out), `"openads"`)
}
