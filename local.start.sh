#!/bin/bash

# stop if already running
if docker ps | grep -q wireport-host; then
    docker stop wireport-host
fi

# remove container if exists
if docker ps -a | grep -q wireport-host; then
    docker rm wireport-host
fi

# build if not built
docker build -t wireport-host .

# if network does not exist, create it
if ! docker network ls | grep -q wgp-net; then
    docker network create wgp-net
fi

docker run -d -it --privileged --sysctl "net.ipv4.ip_forward=1" --sysctl "net.ipv4.conf.all.src_valid_mark=1" \
    -p 51820:51820/udp -p 80:80/tcp -p 443:443/tcp -p 4060:4060/tcp \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v ./fs/data:/data \
    -v ./fs/app/wireport:/app/wireport \
    --name wireport-host \
    anybotsllc/wireport host

# server

# docker run --privileged --sysctl "net.ipv4.ip_forward=1" --sysctl "net.ipv4.conf.all.src_valid_mark=1" \
#     -v /var/run/docker.sock:/var/run/docker.sock \
#     -v ./fs/app/wireport:/app/wireport \
#     --name wireport-server \
#     anybotsllc/wireport join <JOIN_TOKEN>
