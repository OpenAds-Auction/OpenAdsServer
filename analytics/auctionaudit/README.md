# Auction Audit Analytics Module

This module provides real-time auction event streaming. It's set up to consume filters, match bid requests against the filters, and send any that matches on a matched-events kafka topic.

## Configuration

Add to your `pbs.yaml`:

```yaml
analytics:
  auction_audit:
    enabled: true
    brokers:
      - "kafka-broker-1:9092"
      - "kafka-broker-2:9092"
    environment: "prod"

    # Filter consumer settings
    filter_topic: "auction-audit-filters"     # Topic for filter subscriptions

    # Producer settings
    matched_topic: "auction-audit-matched"    # Topic for matched events
    flush_interval: "100ms"                   # Batch flush interval
    compression: "snappy"                     # none/snappy/gzip/lz4/zstd

    # Safety limits
    max_filters: 1000                         # Max concurrent active filters
    max_filter_ttl: "1h"                      # Maximum filter TTL (caps requested expiration)
    cleanup_interval: "1m"                    # How often to clean up expired filters

    # SASL authentication (SCRAM-SHA-512)
    sasl:
      enabled: true
      username: "your-username"
      password: "your-password"
```

## Filter Schema

The filter and Event schemas are defined in the `auction_audit.pb.go` files. BidRequest/BidResponse fields are just serialized to strings for now.

### Matching Logic

string matching is case-insensitive, but exact - no fuzzy matching

- `account_id` is **required**
- `domain`, `app_bundle`, `media_types` are optional filters
- If a filter field is set, the event must match; if not set, any value matches
- For `media_types`, at least one of the filter's types must be present in the event

## Metrics

The module exposes Prometheus metrics:

| Metric | Type | Description |
|--------|------|-------------|
| `auctionaudit_active_filters` | Gauge | Number of currently active audit filters |
| `auctionaudit_filters_registered_total` | Counter | Total filters registered |
| `auctionaudit_filters_expired_total` | Counter | Total filters expired due to TTL |
| `auctionaudit_events_matched_total{account_id}` | Counter | Events matched per account |
| `auctionaudit_send_errors_total` | Counter | Errors sending to Kafka |
| `auctionaudit_consume_errors_total` | Counter | Errors consuming filter messages |

## PII Scrubbing

To ensure PII is scrubbed for this module, configure account-level activity controls:

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
