# Omni-Schema

![License](https://img.shields.io/badge/License-MIT-yellow?style=for-the-badge)
![Go](https://img.shields.io/badge/Go-1.25-00ADD8?style=for-the-badge&logo=go&logoColor=white)
![Protocols](https://img.shields.io/badge/Protocols-10-blue?style=for-the-badge)

Omni-Schema is an advanced, high-performance API morphing service built entirely from scratch in Go. Operating on an Analysis-Synthesis compiler model, the gateway translates arbitrary payloads between highly complex binary and text protocols.

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

## Quick Start (Render API)

The Omni-Schema Gateway is fully hosted and accessible via Render. You do not need to install Go, Docker, or download the repository to use the live morphing APIs.

**Base URL**: `https://morph-gateway.onrender.com`

### 1. Converting a File
Upload a JSON file and download the converted GraphQL schema:

```bash
curl -X POST https://morph-gateway.onrender.com/morph/json/graphql \
  -F "file=@data.json" \
  -o converted.graphql
```

The server returns a downloadable file with the appropriate `Content-Disposition` header. Use `-o` to save the output directly to disk.

### 2. Uploading Custom Schemas
If your target protocols require explicit definitions (like Protobuf or GraphQL), upload them using a standard multipart form data request:

```bash
curl -X POST https://morph-gateway.onrender.com/system/schema \
  -F "file=@my_schema.proto"
```

## Architecture Snapshot

- **Lexers and ASTs**: Constructed natively utilizing `text/scanner`.
- **Lowering Engine**: Maps complex schema abstractions to a universal `uir.TypeMap` and `uir.TypeArray`.
- **Codecs**: Generates heavily specified byte representations directly from the UIR memory pool.
- **WebSockets**: Implements TCP hijacking via `net/http` to securely facilitate GraphQL subscription channels.
