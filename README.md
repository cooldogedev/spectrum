# Spectrum

Spectrum is a blazingly fast, lightweight, and easy to use Minecraft: Bedrock Edition proxy.

[![Discord](https://img.shields.io/discord/1225942695604912279.svg?label=discord&color=7289DA&logo=discord&style=for-the-badge)](https://discord.com/invite/9TPKfeKvK2)

## Examples

Explore how to use Spectrum in the [example](example) directory.

## Implementations

Spectrum's protocol uses Spectral, TCP and QUIC instead of RakNet and the standard Minecraft protocol, providing better reliability and performance. Check out compatible implementations:

- [Dragonfly](https://github.com/cooldogedev/spectrum-df)
- [PocketMine-MP](https://github.com/cooldogedev/spectrum-pm)

## Usage

### API

Spectrum provides an external TCP service for communication between downstream servers and the proxy through packets. This service supports tasks such as player transfers and kicks and is designed to be extensible, allowing you to register your own packets and handlers.

For a practical example, see the [example API implementation](example/api.go). Official implementations include an API client for easy integration. If you’re using Go, you can use the [Dial](api/dial.go) function from the API package to dial an API service.

### Discovery

Spectrum introduces Discovery, a method for server determination when players join. It handles connections asynchronously, determining the server address for connection or signaling disconnection errors. This process allows for blocking operations like database queries and HTTP requests. In addition, Spectrum offers an in-built static discovery that maintains a constant server address. Furthermore, the discovery feature can also function as a server load balancer.

### Processor

The `Processor` interface in Spectrum handles incoming and outgoing packets within sessions, enabling custom filtering and manipulation. This functionality supports implementing anti-cheat measures and other security features. Downstream servers are responsible for prefixing packets to indicate decoding necessity, as per Spectrum protocol specifications.

## Why Spectrum?
- **Protocol Innovation**: Utilizes [Spectral](https://github.com/cooldogedev/spectral) and [QUIC](https://datatracker.ietf.org/doc/html/rfc9000) for enhanced reliability and performance, unlike traditional proxies relying on RakNet and standard Minecraft protocol.

- **Efficient Packet Handling**: Maintains high throughput by bypassing unnecessary packet decoding, optimizing data transmission and reducing latency.

- **Customizability**: Provides extensive customization options, including defining transfer transitions and custom behaviors, for unique gameplay experiences.

- **Lightweight and Fast**: Built for high performance with a lightweight architecture, capable of handling various connection loads efficiently.

- **Stateless**: Simplifies scalability with a stateless design, enhancing flexibility and efficiency in server management by not keeping track of registered servers within the proxy. Transfer between servers is as easy as sending Spectrum's transfer packet to the player from downstream servers.

- **Deterministic**: Takes a unique approach by sidestepping entity translations altogether, relying solely on deterministic entity identifiers provided by the downstream servers.

## Additional Notes
- **Kernel Network Buffer Tuning (Linux):** Under high load or on systems with conservative default settings, the standard Linux kernel network buffer sizes may be insufficient to handle incoming traffic and will cause the proxy to throw errors or disconnect randomly. Example commands provided below show how to increase these buffer sizes to ~7.5MB (adjust to your needs):
  - `sysctl -w net.core.rmem_max=7500000`
  - `sysctl -w net.core.wmem_max=7500000`
  - `sysctl -w net.ipv4.tcp_rmem="4096 87380 7500000"`
