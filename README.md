# Porter

Porter is a high-performance, transparent UDP relay for QUIC traffic. It enables SNI-based routing for UDP protocols, allowing multiple QUIC services behind a single entry point.

## Overview

Routing UDP traffic while maintaining session stickiness is challenging. Porter inspects QUIC packets to extract the Server Name Indication (SNI) during the handshake and tracks Destination Connection IDs (DCID) to ensure packets reach the correct backend, even during client network migrations.

### Key Features

- QUIC-Aware Routing: Parses QUIC Initial packets to route traffic based on SNI.
- Session Stickiness: Tracks QUIC Connection IDs to maintain session integrity.
- Connection Migration Support: Handles client IP/port changes by following the DCID.
- Dynamic Routing Strategies: Supports Simple (static) and [Agones](https://github.com/googleforgames/agones) (game server fleets) strategies.
- Management API: RESTful API to update routing tables in real-time.
- Horizontal Scalability: Optional [Redis](https://github.com/redis/redis) integration for route persistence and sync.

## How it works (for BungeeCord/Velocity users)

Think of Porter as a Layer 4 Proxy for QUIC, similar to how Velocity works for TCP Minecraft traffic.

1. SNI Extraction: Porter inspects the QUIC Initial packet to find the SNI.
2. Dynamic Routing: Porter can query Kubernetes for an available game server via [Agones](https://github.com/googleforgames/agones).
3. Session Persistence: Porter tracks the Connection ID (DCID) to maintain connections even if the client IP changes.

## Architecture

1. QUIC Packet Parsing: Minimal decryption and parsing of QUIC Initial packets to find TLS SNI.
2. Strategy Management: Queries a routing strategy to find the destination backend.
3. Session Mapping: Maps QUIC DCID to the backend.
4. Transparent Forwarding: Forwards subsequent packets based on DCID without re-extraction.

## Getting Started

### Prerequisites

- Go 1.25.5 or later
- [Redis](https://github.com/redis/redis) (optional)

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

Configure Porter via `config.yaml`.

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

Note: By default, Porter listens on port 443. Hytale's default server port is 5520. You can either configure Porter to listen on 5520 or map the host port 5520 to Porter's 443 (e.g., -p 5520:443/udp or via a Kubernetes Service).

## Agones Strategy

The [Agones](https://github.com/googleforgames/agones) strategy allows Porter to dynamically discover and allocate game servers from Agones fleets.

### Configuration

Enable [Agones](https://github.com/googleforgames/agones) support in `config.yaml`:

```yaml
agones:
  enabled: true
  namespace: "default"
  allocator_host: "agones-allocator.agones-system.svc.cluster.local:443"
  allocator_client_cert: "/path/to/tls.crt"
  allocator_client_key: "/path/to/tls.key"
  allocator_ca_cert: "/path/to/ca.crt"

routes:
  - fqdn: "game.example.com"
    type: "agones"
    target: "my-fleet-name"
```

### Fleet Requirements

Fleets must be configured with a players list. Porter looks for servers with room for an additional player.

```yaml
lists:
  players:
    minAvailable: 1
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

### Agones Allocation

`POST /allocate`

Triggers a backend allocation for an [Agones](https://github.com/googleforgames/agones) fleet, creates a temporary routing mapping, and returns the assigned FQDN. This allows servers to connect users to a specific game server instance via its own FQDN.

**Request:**

```json
{
  "fleet": "lobby",
  "domain": "example.com"
}
```

**Response:**

```json
{
  "fqdn": "lobby-xxxx-xxxx.example.com",
  "name": "lobby-xxxx-xxxx"
}
```

## License

This project is licensed under the MIT License - see the LICENSE file for details.
