FROM ubuntu:22.04 AS build
RUN apt-get update && \
    apt-get -y upgrade && \
    apt-get install -y --no-install-recommends wget ca-certificates
WORKDIR /tmp
RUN wget https://dl.google.com/go/go1.24.0.linux-amd64.tar.gz && \
    tar -xf go1.24.0.linux-amd64.tar.gz && \
    mv go /usr/local
RUN mkdir -p /app/prebid-server/
WORKDIR /app/prebid-server/
ENV GOROOT=/usr/local/go
ENV PATH=$GOROOT/bin:$PATH
ENV GOPROXY="https://proxy.golang.org"

# Installing gcc as cgo uses it to build native code of some modules
# Also installing openssl for cryptographic signature generation
RUN apt-get update && \
    apt-get install -y --no-install-recommends git gcc build-essential openssl && \
    apt-get clean && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# CGO must be enabled because some modules depend on native C code
ENV CGO_ENABLED 1
COPY ./ ./
RUN go mod tidy
RUN go mod vendor
ARG TEST="true"
RUN if [ "$TEST" != "false" ]; then ./validate.sh ; fi
# Generate cryptographic signature for build attestation
RUN COMMIT_HASH=$(git rev-parse HEAD) && \
    TIMESTAMP=$(date -u +%Y-%m-%dT%H:%M:%SZ) && \
    echo "Generating build signature for commit: $COMMIT_HASH at $TIMESTAMP" && \
    openssl genrsa -out build_key.pem 2048 && \
    openssl rsa -in build_key.pem -pubout -out build_key.pub && \
    PAYLOAD="${COMMIT_HASH}:${TIMESTAMP}:prebid-server-build" && \
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
    echo "Payload Format: <commit-hash>:<timestamp>:prebid-server-build" && \
    echo "=== END VERIFICATION INFO ===" && \
    rm -f build_key.pem signature.bin && \
    go build -mod=vendor -ldflags "-X github.com/prebid/prebid-server/v3/version.Ver=`git describe --tags | sed 's/^v//'` -X github.com/prebid/prebid-server/v3/version.Rev=`git rev-parse HEAD` -X github.com/prebid/prebid-server/v3/version.BuildSignature=${SIGNATURE} -X github.com/prebid/prebid-server/v3/version.BuildTimestamp=${TIMESTAMP}" .

FROM ubuntu:22.04 AS release
LABEL maintainer="hans.hjort@xandr.com" 
WORKDIR /usr/local/bin/
COPY --from=build /app/prebid-server .
RUN chmod a+xr prebid-server
COPY static static/
COPY stored_requests/data stored_requests/data
RUN chmod -R a+r static/ stored_requests/data

# Installing libatomic1 as it is a runtime dependency for some modules
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates mtr libatomic1 && \
    apt-get clean && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*
RUN addgroup --system --gid 2001 prebidgroup && adduser --system --uid 1001 --ingroup prebidgroup prebid
USER prebid
EXPOSE 8000
EXPOSE 6060
ENTRYPOINT ["/usr/local/bin/prebid-server"]
CMD ["-v", "1", "-logtostderr"]
