# OpenAds Signatures Module

## Overview

The OpenAds module:
1. Intercepts bid requests at the `bidder_request` stage
3. Sends the bid request JSON to the configured external service
4. Expects a JSON array of signatures in response. The JSON isn't validated here, so anything can be returned.
5. Adds the signatures to `ext.openads.int_sigs` in the bid request, along with a hardcoded `version`
6. Forwards the modified request to the bidder adapter

## Example Configuration

```yaml
hooks:
  enabled: true
  modules:
    openads:
      signatures:
        enabled: true
        transport: "uds"
        base_path: "/tmp/test.sock"
        request_path: "/controller/test"
        reject_on_failure: true
  host_execution_plan:
    endpoints:
      "/openrtb2/auction":
        stages:
          bidder_request:
            groups:
              - timeout: 50
                hook_sequence:
                  - module_code: "openads.signatures"
                    hook_impl_code: "openads-signatures-bidder-request-hook"
```

### Configuration Options

- `hooks.modules.openads.signatures.transport`: either "tcp" or "uds"
- `hooks.modules.openads.signatures.base_path`: 
  - For UDS: Socket path (e.g., "/tmp/test.sock")
  - For TCP: host and port (e.g., "localhost:8099", "http://localhost:8099", "https://secure.example.com:443")
- `hooks.modules.openads.signatures.request_path`: HTTP endpoint path appended to the base_path (e.g., "/controller/test")
- `hooks.modules.openads.signatures.reject_on_failure`: If `true`, reject bid requests when the external service call fails. If `false`, set openads defaults and continue on


### Output Format

The module adds an `openads` object to the bid request's `ext` field:

```json
{
  "ext": {
    "openads": {
      "version": 1,
      "int_sigs": [
        // some data
      ]
    }
  }
}
```
