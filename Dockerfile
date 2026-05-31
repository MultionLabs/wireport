# ==========================================
# 1. CADDY BUILD STAGE
# ==========================================
FROM caddy:2.11.3-builder AS caddy-builder

# with the latest security logic and Go 1.26+
ENV GOTOOLCHAIN=go1.26.3
ENV CGO_ENABLED=0

# CVE-2026-46595, CVE-2026-39830–34, CVE-2026-42508 → x/crypto >= v0.52.0
# CVE-2026-39821 → x/net >= v0.55.0
# CVE-2026-34986 → go-jose/v3 >= v3.0.5 (v4 already at v4.1.4 via Caddy deps)
RUN xcaddy build \
    --with github.com/mholt/caddy-l4@afd229714fb14a387f0736cab048afeb72b8946a \
    --with github.com/go-jose/go-jose/v3@v3.0.5 \
    --replace golang.org/x/crypto=golang.org/x/crypto@v0.52.0 \
    --replace golang.org/x/net=golang.org/x/net@v0.55.0

# ==========================================
# 2. COREDNS BUILD STAGE
# ==========================================
FROM golang:1.26.3-alpine AS coredns-builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git make

# 1. Disable CGO to avoid native instruction mismatches
# 2. Set GOAMD64=v1 to use the most compatible instruction set (no AVX/AVX2)
ENV CGO_ENABLED=0
ENV GOAMD64=v1

# Clone and build CoreDNS with fanout plugin
RUN git clone https://github.com/coredns/coredns.git && \
    cd coredns && \
    git checkout v1.14.3 && \
    go get golang.org/x/crypto@v0.52.0 golang.org/x/net@v0.55.0 && \
    go mod tidy && \
    echo "fanout:github.com/networkservicemesh/fanout" >> plugin.cfg && \
    make

# ==========================================
# 3. DOCKER CLI BUILD STAGE (NEW)
# ==========================================
# Compiling Docker CLI natively using Go 1.26.3 to ensure zero legacy stdlib vulnerabilities
FROM golang:1.26.3-alpine AS docker-builder

RUN apk add --no-cache git make
RUN git clone --depth 1 --branch v27.5.1 https://github.com/docker/cli.git /go/src/github.com/docker/cli
WORKDIR /go/src/github.com/docker/cli
ENV CGO_ENABLED=0
RUN make binary && cp build/docker /usr/bin/docker

# ==========================================
# 4. WIREPORT BUILD STAGE
# ==========================================
FROM golang:1.26.3-alpine AS go-builder

WORKDIR /app

# Install build dependencies for SQLite
RUN apk add --no-cache gcc musl-dev

# Enable CGO for SQLite support
ENV CGO_ENABLED=1
ENV CGO_CFLAGS="-D_LARGEFILE64_SOURCE"

COPY ./app/ .

RUN go mod tidy && go get golang.org/x/crypto@v0.52.0 golang.org/x/net@v0.55.0
RUN go build -o wireport ./cmd/server/main.go

# ==========================================
# 5. FINAL RUNTIME STAGE
# ==========================================
# wireguard, tcpdump & other tools
FROM alpine:3.21.3

# Update base image with security patches and install only the minimal runtime
# CVE-2025-11964 (libpcap < 1.10.6): pin from edge before tcpdump pulls 1.10.5-r0 on 3.21
RUN apk --no-cache upgrade && \
    apk add --no-cache --repository=http://dl-cdn.alpinelinux.org/alpine/edge/main \
        "nghttp2>=1.68.1" \
        "busybox>=1.37.0-r15" \
        "libxml2>=2.13.9-r1" \
        "libpcap>=1.10.6-r1" || apk add --no-cache busybox libxml2 && \
    apk add --no-cache \
        wireguard-tools \
        iptables \
        nano \
        bind-tools \
        tcpdump \
        runit \
        socat

# Copy clean binaries all generated uniformly from the Go 1.26.3 toolchain
COPY --from=docker-builder /usr/bin/docker /usr/bin/docker
COPY --from=caddy-builder /usr/bin/caddy /usr/bin/caddy
COPY --from=coredns-builder /app/coredns/coredns /usr/bin/coredns
COPY --from=go-builder /app/wireport /usr/bin/wireport

VOLUME /app/wireport

COPY ./docker/fs/etc/service /etc/service
RUN mkdir -p /app/wireport/caddy/fs/data /app/wireport/caddy/fs/config /etc/coredns /etc/caddy /etc/service-disabled

COPY ./docker/fs/entry.sh /
RUN chmod +x /entry.sh

ENTRYPOINT ["/entry.sh"]
