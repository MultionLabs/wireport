# wireport

wireport is a self-hosted VPN tunnel that securely exposes private Docker services to the Internet and local environment. Powered by WireGuard (networking) and Caddy (reverse proxy).

- Secure tunneling into remote development/staging/production environments to facilitate debugging and troubleshooting of remote Docker-based services
- Exposing Docker services, running in a local network (e.g., a NAS or a home server), to the Internet

## Features

- Secure VPN tunneling (WireGuard)
- HTTP(-S) and TCP/UDP (Level-4) (reverse-) proxy (Caddy)
- TLS termination and 100% automated certificate provisioning and renewal
- Automatic service discovery for Docker containers and easy setup via `wireport` cli
- Self-hosted

## Prerequisites

- Two **serapate**, Linux-based nodes with docker installed:
  - HOST - a Linux-based node with a public IP and open ports: 80/tcp, 443/tcp, 4060/tcp and 51820/udp
  - SERVER - a Linux-based node with Docker-based services / workloads
- Arbitrary number of CLIENT machines (laptops/PCs) that will get access to the private services

# Quick start

## 1. HOST node setup

### 1.1. Make sure to open ports in the firewall

Example for distros with ufw (e.g., Ubuntu):

```
bash
ufw allow 22/tcp
ufw allow 80/tcp
ufw allow 443/tcp
ufw allow 4060/tcp
ufw allow 51820/udp

ufw enable
ufw status
```

### 1.2. Start wireport in host mode

```
bash
mkdir -p ./fs/app/wireport && \
docker run -d -it --privileged --sysctl "net.ipv4.ip_forward=1" --sysctl "net.ipv4.conf.all.src_valid_mark=1" \
    -p 51820:51820/udp -p 80:80/tcp -p 443:443/tcp -p 4060:4060/tcp \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v ./fs/app/wireport:/app/wireport \
    --name wireport-host \
    anybotsllc/wireport host
```

### 1.3. Generate a **join-token** for connecting a SERVER-node to the HOST


```
bash
docker exec -it wireport-host bash
# ... then, from inside the contaienr:
wireport server new
```

- copy the join-token and use it for setting up the SERVER node.

## 2. SERVER node setup

```
bash
mkdir -p ./fs/app/wireport && \
docker run --privileged --sysctl "net.ipv4.ip_forward=1" --sysctl "net.ipv4.conf.all.src_valid_mark=1" \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v ./fs/app/wireport:/app/wireport \
    --name wireport-server \
    anybotsllc/wireport join <JOIN-TOKEN>
```

- here, substitute **<JOIN-TOKEN>** with the value, copied from the HOST node.

## 3. Connectiong CLIENTs to the wireport private network

### 3.1. Generating a WireGuard configuration

On the HOST node:

```
bash
docker exec -it wireport-host bash
# ... then, from inside the contaienr:
wireport client new
```

- copy the WireGuard configuration and feed it to your local WireGuard client.

## 4. Attach **wgp-net** network to docker services on SERVER node that should be exposed via wireport, e.g.:

For an already-running container:

```
bash
docker network connect wgp-net <CONTAINER-NAME>
```

When launching a new docker service:

```
bash
docker run --network=wgp-net ... <PARAMS>
```

**Done!**
Enjoy secure access to your private services, published on the SERVER, locally and remotely over the wireport.

## Security Considerations

- The host container runs with privileged access for network configuration
- All traffic is encrypted using WireGuard
- Control traffic is encrypted using AES in CBC mode
- HTTPS is configurable for secure web access to exposed services

## Troubleshooting

If you encounter issues:
1. Check service logs: `docker logs wireport-host` or `docker logs wireport-server`
2. Verify firewall status & make sure all required ports are open
3. Check status of the WireGuard network inside the HOST and SERVER wireport containers using `wg show` and other WireGuard commands
4. Check ping'ability of private services from inside HOST, SERVER and CLIENT nodes (e.g., docker services, )
5. If a private service is not reachable, check whether it's docker container is connected to `wgp-net` network

## License

[MIT](LICENSE.txt)

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
