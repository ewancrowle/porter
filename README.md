# Porter

Porter is a high-performance, transparent UDP relay designed specifically for QUIC traffic. It enables SNI-based routing for UDP-based protocols, allowing you to host multiple QUIC-powered services behind a single entry point.

## Overview

Unlike traditional TCP proxies, routing UDP traffic while maintaining session stickiness is challenging. Porter solves this by deeply inspecting QUIC packets to extract the Server Name Indication (SNI) during the handshake and subsequently tracking Destination Connection IDs (DCID) to ensure packets are routed to the correct backend even during client network migrations.

### Key Features

- QUIC-Aware Routing: Parses QUIC Initial packets to route traffic based on the requested SNI.
- Session Stickiness: Tracks QUIC Connection IDs to maintain session integrity throughout the connection lifecycle.
- Connection Migration Support: Seamlessly handles client IP/port changes by following the DCID.
- Dynamic Routing Strategies: Support for both Simple (static mapping) and [Agones](https://agones.dev/) (game server fleets) strategies.
- Management API: RESTful API to update routing tables in real-time.
- Horizontal Scalability: Optional Redis integration for route persistence and cross-instance synchronization via Pub/Sub.

## Architecture

1. QUIC Packet Parsing: Porter performs minimal decryption and parsing of QUIC Initial packets to find the TLS SNI.
2. Strategy Management: Once the SNI is known, Porter queries a routing strategy (Simple or [Agones](https://agones.dev/)) to find the destination backend.
3. Session Mapping: A mapping between the QUIC DCID and the backend is stored.
4. Transparent Forwarding: Subsequent packets (including Short Header packets) are forwarded to the backend based on their DCID, bypassing the need for SNI re-extraction.

## Getting Started

### Prerequisites

- Go 1.25.5 or later
- Redis (optional, for multi-instance sync)

### Installation

```bash
git clone https://github.com/ewancrowle/porter.git
cd porter
go build -o porter ./cmd/porter
```

### Running with Docker

```bash
docker build -t porter .
docker run -p 443:443/udp -p 8080:8080 porter
```

## Configuration

Porter is configured via a `config.yaml` file.

```yaml
udp:
  port: 443
  log_requests: true

api:
  port: 8080

redis:
  enabled: false
  address: "localhost:6379"

routes:
  - fqdn: "game1.example.com"
    type: "simple"
    target: "10.0.0.5:7777"
```

## Management API

Porter provides a Fiber-based API for dynamic route management.

### Update a Route

`POST /routes`

```json
{
  "fqdn": "new-game.example.com",
  "type": "simple",
  "target": "10.0.0.10:7777"
}
```

### Agones Allocation (Experimental)

`POST /allocate`

Triggers a backend allocation for an [Agones](https://agones.dev/) fleet and returns the routing information.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
