FROM ubuntu:22.04 AS build

# Pin package versions for deterministic builds
ARG GO_VERSION=1.25.3
ARG GO_CHECKSUM=0335f314b6e7bfe08c3d0cfaa7c19db961b7b99fb20be62b0a826c992ad14e0f

# Install system dependencies with pinned versions
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        wget=1.21.2-2ubuntu1.1 \
        ca-certificates=20240203~22.04.1 \
        git=1:2.34.1-1ubuntu1.15 \
        gcc=4:11.2.0-1ubuntu1 \
        build-essential=12.9ubuntu3 \
        openssl=3.0.2-0ubuntu1.20 && \
    apt-get clean && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# Download and verify Go binary with checksum validation
WORKDIR /tmp
RUN wget https://dl.google.com/go/go${GO_VERSION}.linux-amd64.tar.gz && \
    echo "${GO_CHECKSUM} go${GO_VERSION}.linux-amd64.tar.gz" | sha256sum -c - && \
    tar -xf go${GO_VERSION}.linux-amd64.tar.gz && \
    mv go /usr/local && \
    rm go${GO_VERSION}.linux-amd64.tar.gz

# Set up Go environment with deterministic settings
WORKDIR /app/prebid-server/
ENV GOROOT=/usr/local/go
ENV PATH=$GOROOT/bin:$PATH
ENV GOPROXY="https://proxy.golang.org"
ENV GOSUMDB="sum.golang.org"
ENV GOCACHE="/tmp/go-cache"
ENV GOMODCACHE="/tmp/go-mod-cache"

# CGO must be enabled because some modules depend on native C code
ENV CGO_ENABLED=1

# Copy dependency files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source code
COPY ./ ./

# Regenerate vendor directory to ensure consistency
RUN go mod vendor

# Generate modules and build with deterministic flags
RUN go generate modules/modules.go
# Generate cryptographic signature for build attestation
RUN COMMIT_HASH=$(git rev-parse HEAD) && \
    TIMESTAMP=$(date -u +%Y-%m-%dT%H:%M:%SZ) && \
    echo "Generating build signature for commit: $COMMIT_HASH at $TIMESTAMP" && \
    openssl genrsa -out build_key.pem 2048 && \
    openssl rsa -in build_key.pem -pubout -out build_key.pub && \
    PAYLOAD="${COMMIT_HASH}:${TIMESTAMP}:openads-server-build" && \
    echo "Signing payload: $PAYLOAD" && \
    echo -n "$PAYLOAD" | openssl dgst -sha256 -sign build_key.pem -out signature.bin && \
    SIGNATURE=$(base64 -w 0 signature.bin) && \
    echo "=== BUILD ATTESTATION PUBLIC KEY ===" && \
    cat build_key.pub && \
    echo "=== END PUBLIC KEY ===" && \
    echo "=== BUILD SIGNATURE (BASE64) ===" && \
    echo "$SIGNATURE" && \
    echo "=== END SIGNATURE ===" && \
    echo "=== SIGNATURE PAYLOAD (PLAINTEXT) ===" && \
    echo "$PAYLOAD" && \
    echo "=== END PAYLOAD ===" && \
    echo "=== VERIFICATION INFO ===" && \
    echo "Git Commit: $COMMIT_HASH" && \
    echo "Build Timestamp: $TIMESTAMP" && \
    echo "Payload Format: <commit-hash>:<timestamp>:openads-server-build" && \
    echo "=== END VERIFICATION INFO ===" && \
    # Create artifacts directory and save only the public key \
    mkdir -p /artifacts && \
    cp build_key.pub /artifacts/build_key.pub && \
    rm -f build_key.pem signature.bin && \
    go build \
        -mod=vendor \
        -trimpath \
        -buildmode=pie \
        -ldflags "-s -w -X github.com/prebid/prebid-server/v3/version.Ver=`git describe --tags | sed 's/^v//'` -X github.com/prebid/prebid-server/v3/version.Rev=`git rev-parse HEAD` -X github.com/prebid/prebid-server/v3/version.BuildSignature=${SIGNATURE} -X github.com/prebid/prebid-server/v3/version.BuildTimestamp=${TIMESTAMP}" \
        -o openads .

FROM ubuntu:22.04 AS release
LABEL org.opencontainers.image.authors="openads-eng@thetradedesk.com"
WORKDIR /usr/local/bin/

COPY --from=build /artifacts /artifacts
COPY --from=build /app/prebid-server/openads /usr/local/bin/openads
RUN chmod a+xr /usr/local/bin/openads
COPY --from=build /app/prebid-server/static static/
COPY --from=build /app/prebid-server/stored_requests/data stored_requests/data
RUN chmod -R a+r static/ stored_requests/data

# Installing runtime dependencies with pinned versions
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates=20240203~22.04.1 \
        mtr=0.95-1 \
        libatomic1=12.3.0-1ubuntu1~22.04.2 && \
    apt-get clean && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*
RUN addgroup --system --gid 2001 prebidgroup && adduser --system --uid 1001 --ingroup prebidgroup prebid
USER prebid
EXPOSE 8000
EXPOSE 6060
ENTRYPOINT ["/usr/local/bin/openads"]
CMD ["-v", "1", "-logtostderr"]
