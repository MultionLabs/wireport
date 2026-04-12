# caddy build stage
FROM caddy:2.11.2-builder AS caddy-builder

# build a custom caddy image with the l4 plugin
RUN xcaddy build \
    --with github.com/mholt/caddy-l4@afd229714fb14a387f0736cab048afeb72b8946a


# CoreDNS build stage
FROM golang:1.25.8-alpine AS coredns-builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git make

# Clone and build CoreDNS with fanout plugin
RUN git clone https://github.com/coredns/coredns.git && \
    cd coredns && \
    git checkout v1.14.2 && \
    echo "fanout:github.com/networkservicemesh/fanout" >> plugin.cfg && \
    make

# wireport build stage
FROM golang:1.25.8-alpine AS go-builder

WORKDIR /app

# Install build dependencies for SQLite
RUN apk add --no-cache gcc musl-dev

# Enable CGO for SQLite support
ENV CGO_ENABLED=1
ENV CGO_CFLAGS="-D_LARGEFILE64_SOURCE"

COPY ./app/ .
RUN go build -o wireport ./cmd/server/main.go

# wireguard, tcpdump & other tools
FROM alpine:3.21

# Update base image with security patches and install only the minimal runtime
RUN apk --no-cache upgrade && apk add --no-cache \
    wireguard-tools \
    iptables \
    nano \
    bind-tools \
    tcpdump \
    runit \
    docker-cli \
    socat

COPY --from=caddy-builder /usr/bin/caddy /usr/bin/caddy
COPY --from=coredns-builder /app/coredns/coredns /usr/bin/coredns
COPY --from=go-builder /app/wireport /usr/bin/wireport

VOLUME /app/wireport

COPY ./docker/fs/etc/service /etc/service
RUN mkdir -p /app/wireport/caddy/fs/data /app/wireport/caddy/fs/config /etc/coredns /etc/caddy /etc/service-disabled

COPY ./docker/fs/entry.sh /
RUN chmod +x /entry.sh

ENTRYPOINT ["/entry.sh"]
