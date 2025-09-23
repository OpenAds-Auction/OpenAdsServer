# OpenAds Module

## Overview

The OpenAds module is a Prebid Server hook that automatically adds `ext.openads = 1` to all outgoing bid requests sent to any adapter.

## Usage

```yaml
hooks:
  enabled: true
  modules:
    prebid:
      openads:
        enabled: true
  host_execution_plan:
    endpoints:
      "/openrtb2/auction":
        stages:
          bidder_request:
            groups:
              - timeout: 10
                hook_sequence:
                  - module_code: "prebid.openads"
                    hook_impl_code: "prebid-openads-bidder-request-hook"
```
