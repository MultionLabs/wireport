FROM caddy:2.10.0-builder AS caddy-builder

# build a custom caddy image with the l4 plugin
RUN xcaddy build \
    --with github.com/mholt/caddy-l4@87e3e5e2c7f986b34c0df373a5799670d7b8ca03

# Go build stage
FROM golang:1.24-alpine AS go-builder

WORKDIR /app

# Install build dependencies for SQLite
RUN apk add --no-cache gcc musl-dev

# Enable CGO for SQLite support
ENV CGO_ENABLED=1
ENV CGO_CFLAGS="-D_LARGEFILE64_SOURCE"

COPY ./app/ .
RUN go build -o wireport ./cmd/server/main.go

# wireguard, dnsmasq & other tools
FROM alpine:3.21

RUN apk add --no-cache -U \
    wireguard-tools \
    iptables \
    nano \
    dnsmasq \
    bind-tools \
    tcpdump \
    runit \
    docker

COPY --from=caddy-builder /usr/bin/caddy /usr/bin/caddy
COPY --from=go-builder /app/wireport /usr/bin/wireport

VOLUME /etc/caddy
VOLUME /caddy
VOLUME /app/wireport

COPY ./docker/fs/etc/service /etc/service
RUN mkdir -p /etc/caddy && mkdir -p /etc/service-disabled

COPY ./docker/fs/entry.sh /
RUN chmod +x /entry.sh

ENTRYPOINT ["/entry.sh"]
