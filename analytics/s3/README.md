# S3 Analytics Module

This module writes analytics events (auction, amp, video) directly to Amazon S3 in newline-delimited JSON (NDJSON) format with gzip compression.

It's written in the following format -  `s3://{bucket}/{prefix}/env={environment}/type={type}/date=YYYY-MM-DD/hour=HH/{timestamp}_{pod}.jsonl.gz`


## Configuration

Add to your `pbs.yaml`:

```yaml
analytics:
  s3:
    enabled: true
    bucket: "my-analytics-bucket"
    prefix: "prebid-analytics"
    environment: "prod"   # Environment name for partitioning
    region: "us-east-1"   # Optional
    upload_timeout: "2s"  # Timeout for entire upload operation (default: 2s)
    fallback_dir: "/var/log/prebid-fallback"  # Optional, for storing failed uploads
    use_path_style: false # Set to true when using LocalStack
    
    buffers:
      buffer_size: "10MB"  # Flush when buffer reaches this size (default: 10MB)
      timeout: "1m"        # Flush after this interval (default: 15m)
```

## Metrics

The module exposes the following Prometheus metrics:

### Counter
- `analytics_s3_upload_total{destination,status}` - S3 upload outcomes

**Labels:**
- `destination`: `s3` or `local` (fallback)
- `status`: `success`, `timeout`, or `failure`
