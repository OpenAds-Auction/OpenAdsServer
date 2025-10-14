# Build Attestation Documentation

## Overview

OpenAds server includes cryptographic build attestation through the `/attestation` endpoint. This provides cryptographic proof of the build integrity, linking the binary to the exact source code commit and build timestamp.

## How It Works

### Build Process

During Docker build, the following cryptographic signature generation occurs:

1. **Generate RSA Key Pair**: A 2048-bit RSA key pair is generated
2. **Create Payload**: Payload format: `<git-commit-hash>:<iso-timestamp>:openads-server-build`
3. **Sign Payload**: The payload is signed using RSA-SHA256
4. **Encode Signature**: The signature is base64-encoded
5. **Inject Signature**: The signature is injected into the binary via ldflags
6. **Output Public Key**: The public key is output to build logs
7. **Cleanup**: Private key is securely deleted

### Signature Payload Structure

```
<git-commit-hash>:<iso-timestamp>:openads-server-build
```

**Example:**
```
2272ad2a76b687fbfede6e81229cf9709598405b:2025-09-23T20:24:30Z:openads-server-build
```

## Endpoint Usage

### GET /attestation

Returns build attestation information including the cryptographic signature.

**Response Format:**
```json
{
  "build_signature": "base64-encoded-signature",
  "version": "git-tag-version",
  "revision": "git-commit-hash"
}
```

**Example Response:**
```json
{
  "build_signature": "ZXEjKS7AwyZhbHh+c3EuKmDnTIGwIY/OhctMTJsqLks3PpN+uQDhK2EufDasXL2jnYnxlSFVRpSdXBb9lIGvk79Dl+13ztG5JZrvBermJUefdo7jm/FTdzOYt7XzJrizdBjTkUqLA13IYlCfQnuzgeVuDWGZhMZZWtPyR361kccIysFq3PeBw2p17hZlQm4sZLUTyV3jSkmSr4uko7CGaMmfYZEQPRxFFa0zmCX7CA38+xwGsmmH9Q7YWZ4e1bGmQVbePErtue5PYsgMrHIfK3YtTyO/wgUr6O/5r31BOnT12HCPUNbNX9Qh8cUYZcvtzP/WR+0mGoKmbMN6JEW5yQ==",
  "version": "3.25.0-7-g2272ad2a7",
  "revision": "2272ad2a76b687fbfede6e81229cf9709598405b"
}
```

## Signature Verification

### Prerequisites

- `openssl` command-line tool
- Public key from build output
- Signature from `/attestation` endpoint

### Verification Process

1. **Extract Signature**: Get the signature from the `/attestation` endpoint
2. **Get Public Key**: Extract the public key from the Docker build output
3. **Recreate Payload**: Reconstruct the original payload using git commit hash and timestamp
4. **Verify Signature**: Use openssl to verify the signature

### Verification Script Example

```bash
#!/bin/bash

# Extract signature from attestation endpoint
ATTESTATION_RESPONSE=$(curl -s http://localhost:8000/attestation)
SIGNATURE=$(echo "$ATTESTATION_RESPONSE" | jq -r '.build_signature')
REVISION=$(echo "$ATTESTATION_RESPONSE" | jq -r '.revision')

# Create public key file (from build output)
cat > build_key.pub << 'EOF'
-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA4AKBLKAlxdazCQ++kdEE
1TtgobRsQXp1ttbXTS3wJ1NJLG12pjyStjS0AGzaAP7a/bgWaTBfxlBGKQaKrKWx
7clN2HbxNnZOLf6jbEvJLgXfFSWc0W6ercvlEoqVDc/kCvOXgnNEPiTXtlS+I69u
Bc7NQHUH6HsP+Wxybm/2C1dAg83mzV1V6e+dY+ncm2nAH5w7Iza3HNoUTKarVDXq
j/snus1F9VPH+JUQJwT/3Mw8ahX+ocl/BnkRuM44ig0AkGPRv8zBTNRy7A7AXLfW
RBD/R/7WFRDFFqF659svdDBfRw7FPJtW194K1IVQgOW3JOQbzBOrHkwP2UYO4Sgp
mQIDAQAB
-----END PUBLIC KEY-----
EOF

# Recreate payload (timestamp from build output)
TIMESTAMP="2025-09-23T20:24:30Z"
PAYLOAD="${REVISION}:${TIMESTAMP}:openads-server-build"

# Decode signature
echo "$SIGNATURE" | base64 -d > signature.bin

# Verify signature
if echo -n "$PAYLOAD" | openssl dgst -sha256 -verify build_key.pub -signature signature.bin; then
    echo "successfully verified signature"
else
    echo "signature verification failed"
fi

# Cleanup
rm -f build_key.pub signature.bin
```

## Build Output

During Docker build, you'll see comprehensive attestation output like:

```
=== BUILD ATTESTATION PUBLIC KEY ===
-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA1d80Orcz5s2Z38sKve1R
HGJo1uQi0FQJ2Dlrs8OTms25b5eHHAXX/11UjgDdvvErWThERYKbFp2cClsTM2Q5
PrkjTI6LqkgpEIseYETkTqFGGkInLCRpC36QkEPlLQC0MSjiImjRDbBZ5MDSNt8/
2Ll64uP/0UQW1ALGbMF3IMWRY8KC5IjwnmQCjXjVWXCLTdfpKmfGMjZZ5mfs+aop
SM2S6mrIy+hjiZyw8wpx8BcX8LEwleKtPPvPXjn8vYu+W0hAOSrneiXEq1zqgvBI
c0ZjiikfJpQJtEHfkVWm7xu6BhHBnNp9WODAkDoo4tgtcS+S3PZOg6jrdX6CZeuv
FQIDAQAB
-----END PUBLIC KEY-----
=== END PUBLIC KEY ===
=== BUILD SIGNATURE (BASE64) ===
sSjU5XQRNPnyzL74ongQuhpw+zS0mNP1u2AIlFuvOo9BYzu7QtN973ZnSQ0yD198K7omlCAg1rcJYPeDEcVHRQF/mwtaHsxFPJ9It3boMmszZYI+Jx9yjbdu74cBaZWWYjEkg09kp0GETdVcTCgQIlmdCgspdNPELyIzXRJWTQjhXvU0sxTlyQTg5vRZbQxHOC8tiFG2v2g/H+1vOjXZvAPE/+giKNWSMC1klBAAtKXhfEVyaiFiNjFnDKxXy3MjKyML8MGJitFn3yqeEbtPOulUJF92dswLtCYG5UwDkXRH4TgJdVnuJtSTWOdPJPSG4G0TIsZgiMkBQI2qkPOxig==
=== END SIGNATURE ===
=== SIGNATURE PAYLOAD (PLAINTEXT) ===
2272ad2a76b687fbfede6e81229cf9709598405b:2025-09-23T20:37:53Z:openads-server-build
=== END PAYLOAD ===
=== VERIFICATION INFO ===
Git Commit: 2272ad2a76b687fbfede6e81229cf9709598405b
Build Timestamp: 2025-09-23T20:37:53Z
Payload Format: <commit-hash>:<timestamp>:openads-server-build
=== END VERIFICATION INFO ===
```

### Accessing Public Key from Build Logs

Since Docker build logs scroll quickly, save the build output to a file to easily access the public key:

```bash
# Save build logs to file
PBS_GDPR_DEFAULT_VALUE=0 docker build --no-cache -t prebid-server . > build.log 2>&1

# Extract public key from logs
grep -A 10 "BUILD ATTESTATION PUBLIC KEY" build.log

# Extract all attestation information
grep -A 30 "BUILD ATTESTATION PUBLIC KEY" build.log
```
