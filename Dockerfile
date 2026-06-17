FROM ubuntu:22.04 AS build

# Pin package versions for deterministic builds
ARG GO_VERSION=1.23.0
ARG GO_CHECKSUM=905a297f19ead44780548933e0ff1a1b86e8327bb459e92f9c0012569f76f5e3

# Install system dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        wget \
        ca-certificates \
        git \
        gcc \
        build-essential \
        openssl && \
    dpkg-query -W -f='${Package}=${Version}\n' | sort > /build-packages.txt && \
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
ARG TEST="true"
RUN if [ "$TEST" != "false" ]; then ./validate.sh ; fi
RUN go build -mod=vendor -ldflags "-X github.com/prebid/prebid-server/v4/version.Ver=`git describe --tags | sed 's/^v//'` -X github.com/prebid/prebid-server/v4/version.Rev=`git rev-parse HEAD`" .

FROM ubuntu:22.04 AS release
LABEL org.opencontainers.image.authors="openads-eng@thetradedesk.com"
WORKDIR /usr/local/bin/

COPY --from=build /artifacts /artifacts
COPY --from=build /build-packages.txt /artifacts/build-packages.txt
COPY --from=build /app/prebid-server/openads /usr/local/bin/openads
RUN chmod a+xr /usr/local/bin/openads
COPY --from=build /app/prebid-server/static static/
COPY --from=build /app/prebid-server/stored_requests/data stored_requests/data
RUN chmod -R a+r static/ stored_requests/data

# Install runtime dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        mtr \
        libatomic1 && \
    dpkg-query -W -f='${Package}=${Version}\n' | sort > /artifacts/runtime-packages.txt && \
    apt-get clean && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*
RUN addgroup --system --gid 2001 prebidgroup && adduser --system --uid 1001 --ingroup prebidgroup prebid
USER prebid
EXPOSE 8000
EXPOSE 6060
ENTRYPOINT ["/usr/local/bin/openads"]
CMD ["-v", "1", "-logtostderr"]
