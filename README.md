# Omni-Schema Gateway

![Go Version](https://img.shields.io/badge/Go-1.25.0-00ADD8?style=flat&logo=go)
![Zero Dependencies](https://img.shields.io/badge/Dependencies-Zero-brightgreen)
![Build](https://img.shields.io/badge/Build-Passing-success)
![Docker](https://img.shields.io/badge/Docker-Supported-2496ED?style=flat&logo=docker)

Omni-Schema Gateway is an advanced, high-performance API morphing service built entirely from scratch in Go. Operating on an elegant Analysis-Synthesis compiler model, the gateway is designed to translate arbitrary payloads between highly complex binary and text protocols. It strictly adheres to a Zero-Dependency architecture, relying purely on the Go Standard Library for all operations.

---

## Features

The gateway acts as a universal translator. It parses incoming structures into a Universal Intermediate Representation (UIR) graph, enabling seamless, native morphing between disparate protocols.

Supported formats and protocols include:
- Standard Formats: JSON, Protobuf
- Zero-Copy and Memory-Aligned: Cap'n Proto
- Schemaless Binary: MessagePack
- Columnar and Big Data: Apache Parquet
- Hierarchical Multidimensional: HDF5
- Real-Time Streaming: Native GraphQL Subscriptions running over custom RFC 6455 WebSockets

---

## Documentation

For instructions on operating the gateway, uploading schemas, and mapping structural conversions, please refer to the official documentation:

- [API Documentation](./API_DOCUMENTATION.md)

---

## Quick Start

### Running Natively
Because the project enforces a zero-dependency rule, it can be compiled and run directly with standard Go commands without the need for external package managers.

```bash
go run ./cmd/server
```

### Running with Docker
A highly optimized, multi-stage scratch build is provided for containerized environments. The service can be deployed seamlessly via Docker Compose:

```bash
cd Docker
docker-compose up --build -d
```

The server automatically binds to and listens on port 8080.

---

## Architecture Snapshot

- Lexers and ASTs: Constructed natively utilizing `text/scanner`.
- Lowering Engine: Maps complex schema abstractions to a universal `uir.TypeMap` and `uir.TypeArray`.
- Codecs: Generates heavily specified byte representations directly from the UIR memory pool.
- WebSockets: Implements TCP hijacking via `net/http` to securely facilitate GraphQL subscription channels.
