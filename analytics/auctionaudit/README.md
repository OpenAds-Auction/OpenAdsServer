# Auction Audit Analytics Module

This module provides real-time auction event streaming. It's set up to consume filters, match bid requests against the filters, and send any that matches on a matched-events kafka topic.

## Configuration

Add to your `pbs.yaml`:

```yaml
analytics:
  auction_audit:
    enabled: true
    environment: "prod"

    # Filter registry settings
    max_filters: 500                          # Max concurrent active filters
    max_filter_ttl: "1h"                      # Maximum filter TTL (caps requested expiration)
    cleanup_interval: "10m"                   # How often to clean up expired filters

    kafka:
      brokers:
        - "kafka-broker-1:9092"
        - "kafka-broker-2:9092"
      sasl:
        enabled: true
        username: ""
        password: ""
        insecure_skip_verify: false
      matched_topic: "auction-audit-matched"
      filter_topic: "auction-audit-filters"
      flush_interval: "1s"                    # Batch flush interval
      compression: "snappy"                   # none/snappy/gzip/lz4/zstd
```

## Filter Schema

The filter and Event schemas are defined in the `auction_audit.pb.go` files. BidRequest/BidResponse fields are just serialized to strings for now.

### Matching Logic

string matching is case-insensitive, but exact - no fuzzy matching

- `account_id` is **required**
- `domain`, `app_bundle`, `media_types` are optional filters
- For `media_types`, at least one of the filter's types must be present in the event

## Metrics

The module exposes Prometheus metrics:

| Metric | Type | Description |
|--------|------|-------------|
| `auction_audit_actions_total` | Counter | Count of filters registered/unregistered, and count of events matched by account |
| `auction_audit_errors_total` | Counter | Count of errors by reason |
| `auction_audit_active_filters` | Gauge | Number of currently active audit filters |

## PII Scrubbing

PII scrubbing can be configured via account-level activity controls:

```json
{
  "privacy": {
    "allowactivities": {
      "transmitUfpd": {
        "default": true,
        "rules": [{"allow": false, "condition": {"componentName": ["auctionaudit"]}}]
      },
      "transmitPreciseGeo": {
        "default": true,
        "rules": [{"allow": false, "condition": {"componentName": ["auctionaudit"]}}]
      }
    }
  }
}
```
