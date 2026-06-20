# Omni-Schema Gateway

![License](https://img.shields.io/badge/License-MIT-yellow?style=for-the-badge)
![Go](https://img.shields.io/badge/Go-1.25-00ADD8?style=for-the-badge&logo=go&logoColor=white)
![Architecture](https://img.shields.io/badge/Architecture-Compiler_Model-blueviolet?style=for-the-badge)
![Protocols](https://img.shields.io/badge/Protocols-10-blue?style=for-the-badge)

Omni-Schema Gateway is an advanced, high-performance API morphing service built entirely from scratch in Go. Operating on an Analysis-Synthesis compiler model, the gateway translates arbitrary payloads between highly complex binary and text protocols.

---

## Features

The gateway acts as a universal translator. It parses incoming structures into a Universal Intermediate Representation (UIR) graph, enabling seamless, native morphing between disparate protocols.

Supported formats and protocols include:
- Standard Formats: [JSON](https://www.json.org/), [Protobuf](https://protobuf.dev/)
- Zero-Copy and Memory-Aligned: [Cap'n Proto](https://capnproto.org/)
- Schemaless Binary: [MessagePack](https://msgpack.org/)
- Columnar and Big Data: [Apache Parquet](https://parquet.apache.org/)
- Hierarchical Multidimensional: [HDF5](https://www.hdfgroup.org/solutions/hdf5/)
- Real-Time Streaming: Native [GraphQL](https://graphql.org/) Subscriptions running over custom RFC 6455 WebSockets

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

## Architecture Snapshot

- **Lexers and ASTs**: Constructed natively utilizing `text/scanner`.
- **Lowering Engine**: Maps complex schema abstractions to a universal `uir.TypeMap` and `uir.TypeArray`.
- **Codecs**: Generates heavily specified byte representations directly from the UIR memory pool.
- **WebSockets**: Implements TCP hijacking via `net/http` to securely facilitate GraphQL subscription channels.
