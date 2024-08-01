# Spectrum

Spectrum is a blazingly fast, lightweight, and easy to use Minecraft: Bedrock Edition proxy.

[![Discord](https://img.shields.io/discord/1225942695604912279.svg?label=discord&color=7289DA&logo=discord&style=for-the-badge)](https://discord.com/invite/9TPKfeKvK2)

## Examples

Explore how to use Spectrum in the [example](example) directory.

## Implementations

Spectrum's protocol uses TCP and QUIC instead of RakNet and the standard Minecraft protocol, providing better reliability and performance. Check out compatible implementations:

- [Dragonfly](https://github.com/cooldogedev/spectrum-df)
- [PocketMine-MP](https://github.com/cooldogedev/spectrum-pm)

## Usage

### API

Spectrum offers an external TCP service for communication between downstream servers and the proxy. This versatile service supports tasks like player transfers and kicks, using packets for efficient data exchange. For guidance, see this [example](example/api.go). Official implementations also include an API client for easy integration.

### Discovery

Spectrum introduces Discovery, a method for server determination when players join. It handles connections asynchronously, determining the server address for connection or signaling disconnection errors. This process allows for blocking operations like database queries and HTTP requests. In addition, Spectrum offers an in-built static discovery that maintains a constant server address. Furthermore, the discovery feature can also function as a server load balancer.

### Processor

The `Processor` interface in Spectrum handles incoming and outgoing packets within sessions, enabling custom filtering and manipulation. This functionality supports implementing anti-cheat measures and other security features. Downstream servers are responsible for prefixing packets to indicate decoding necessity, as per Spectrum protocol specifications.

## Why Spectrum?
- **Protocol Innovation**: Utilizes TCP and QUIC for enhanced reliability and performance, unlike traditional proxies relying on RakNet and standard Minecraft protocol.

- **Efficient Packet Handling**: Maintains high throughput by bypassing unnecessary packet decoding, optimizing data transmission and reducing latency.

- **Customizability**: Provides extensive customization options, including defining transfer transitions and custom behaviors, for unique gameplay experiences.

- **Lightweight and Fast**: Built for high performance with a lightweight architecture, capable of handling various connection loads efficiently.

- **Stateless**: Simplifies scalability with a stateless design, enhancing flexibility and efficiency in server management by not keeping track of registered servers within the proxy. Transfer between servers is as easy as sending Spectrum's transfer packet to the player from downstream servers.

- **Deterministic**: Takes a unique approach by sidestepping entity translations altogether, relying solely on deterministic entity identifiers provided by the downstream servers.
