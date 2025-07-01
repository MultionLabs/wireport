<p align="center">
  <img src="assets/wireport-with-slogan.png" alt="wireport logo" width="200" />
</p>

<h1 align="center" style="color:#23132d">
  wireport
</h1>

<p align="center">
  <strong>Self-hosted ingress proxy and VPN tunnel that securely exposes private local and Docker-based services to the Internet.</strong><br />
  Powered by WireGuard, CoreDNS and Caddy
</p>

<p align="center">
  <a href="#features">Features</a> •
  <a href="#quick-start">Quick Start</a> •
  <a href="#security-considerations">Security</a> •
  <a href="#troubleshooting">Troubleshooting</a>
</p>

---


**wireport** is a self-hosted ingress proxy and VPN tunnel that securely exposes private local and Docker-based services to the Internet. Powered by WireGuard (secure networking), CoreDNS and Caddy (performant reverse proxy).

- Exposing local and Docker-based services, running in a local network (e.g., on the local machine, on a corporate network, on a NAS or on a home server), to the Internet
- Secure tunneling into remote development/staging/production environments to facilitate debugging and troubleshooting of remote Docker-based services

## Features

- Secure VPN tunneling (WireGuard)
- Automatic service discovery and hostname resolution for Docker containers (CoreDNS)
- HTTP(S) and TCP/UDP (Layer-4) reverse proxy (Caddy)
- TLS termination and 100% automated certificate provisioning and renewal
- Quick and easy setup via `wireport` CLI and pre-built Docker image
- Self-hosted
- High performance with a low memory footprint
- Multiplatform CLI (Linux, macOS, Windows — ARM64 & AMD64)

## Key Concepts

- **GATEWAY** – a Linux-based machine with Docker installed, a public IP address, and the following open ports: 80/tcp, 443/tcp, 4060/tcp and 51820/udp. This node acts as the ingress gateway and an entry point to your published services.
- **CLIENT** – any number of laptops/PCs that will connect to the WireGuard network to manage the ingress network and expose services.
- **SERVER** *(optional)* – one or more Linux-based machines (with Docker) that run the workloads you want to expose. These nodes join the same private WireGuard network, provided by the GATEWAY.

## Quick Start

Get up and running in just **two commands**:


#### 1. Bring a GATEWAY online

```bash
wireport gateway up ssh-user@<GATEWAY_IP> --ssh-key-path ~/.ssh/id_rsa
```

<details>
<summary><strong>Important – firewall and other prerequisites</strong></summary>

`wireport gateway up` expects that:

1) the following ports must be reachable on the target GATEWAY machine *before* you run the command:

* 22/tcp (SSH)
* 80/tcp and 443/tcp (HTTP/HTTPS)
* 4060/tcp (Wireport control channel)
* 51820/udp (WireGuard)

Example with UFW:

```bash
sudo ufw allow 22,80,443,4060/tcp
sudo ufw allow 51820/udp
sudo ufw enable
```

2) Docker is installed on the target GATEWAY machine
3) The account used for SSH-ing into the target GATEWAY machine has all the necessary permissions for managing Docker containers, images and networks
</details>

#### 2. Publish a local service to the Internet

```bash
wireport service publish \
  --local  http://10.0.0.2:3000 \
  --public https://demo.example.com:443
```

<details>
<summary><strong>Important - DNS config and other prerequisites</strong></summary>

1) For the service to become available over the given public URL, there must be a respective `A`-record in the DNS settings of your domain name provider, pointing to the target **GATEWAY** machine's IP address.

2) After bootstrapping the GATEWAY node with `wireport gateway up ...` command, you should add the respective WireGuard tunnel on your local machine

3) There must be a service running on the GATEWAY and port specified in the `--local` flag provided to the `wireport service publish` command

</details>

<details>
<summary>Flags explained</summary>

* **--local** – URL of the service **on the machine where you run the command** (or another node from the newly created WireGuard network)
* **--public** – External protocol / hostname / port that will be reachable on the GATEWAY
* Automatically provisions a trusted TLS certificate and updates Caddy's reverse proxy

</details>

---

## Other useful operations

Need more? Here are some other useful commands:

| Purpose | Command |
|---------|---------|
| Add a workload SERVER | `wireport server up sshuser@<SERVER_IP>` |
| Remove a public endpoint | `wireport service unpublish -p https://demo.example.com:443` |
| Adjust headers/timeouts etc. | `wireport service params new -p https://demo.example.com:443 --param-value 'header_up X-Tenant-Hostname {http.request.host}'` |
| Create more CLIENTs with access to the WireGuard network | `wireport client new` |
| Tear down a GATEWAY | `wireport gateway down <GATEWAY_IP>` |
| Tear down a SERVER| `wireport server down sshuser@<SERVER_IP>` |

Refer to `wireport --help` or the documentation for the full CLI reference.

## Security Considerations

- The gateway container runs with privileged access for network configuration
- All traffic is encrypted using WireGuard
- Control traffic is encrypted (TLS)
- HTTPS is configurable for secure web access to exposed services

## Troubleshooting

If you encounter issues:
1. Check service logs: `docker logs wireport-gateway` or `docker logs wireport-server`
2. Verify firewall status & make sure all required ports are open
3. Check status of the WireGuard network inside the GATEWAY and SERVER wireport containers using `wg show` and other WireGuard commands
4. Check pingability of private services from inside GATEWAY, SERVER and CLIENT nodes
5. If a private service is not reachable, make sure the container is running and check its logs; check whether the target container (in case of the SERVER workloads) is attached to `wireport-net` docker network (wireport agent manages this automatically).

## License

[MIT](LICENSE.txt)

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
