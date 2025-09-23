package modules

import (
	fiftyonedegreesDevicedetection "github.com/prebid/prebid-server/v3/modules/fiftyonedegrees/devicedetection"
	prebidOpenads "github.com/prebid/prebid-server/v3/modules/prebid/openads"
	prebidOrtb2blocking "github.com/prebid/prebid-server/v3/modules/prebid/ortb2blocking"
	prebidRulesengine "github.com/prebid/prebid-server/v3/modules/prebid/rulesengine"
)

// builders returns mapping between module name and its builder
// vendor and module names are chosen based on the module directory name
func builders() ModuleBuilders {
	return ModuleBuilders{
		"fiftyonedegrees": {
			"devicedetection": fiftyonedegreesDevicedetection.Builder,
		},
		"prebid": {
			"openads":       prebidOpenads.Builder,
			"ortb2blocking": prebidOrtb2blocking.Builder,
			"rulesengine":   prebidRulesengine.Builder,
		},
	}
}
