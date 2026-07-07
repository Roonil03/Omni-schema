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

### 1. Converting a File (Automatic Filename Preservation)
Upload a file from your current directory and download the converted schema directly back into the same directory. The server automatically preserves your file's base name (e.g., `egg.json` converts and saves locally as `egg.graphql`).

To let `curl` automatically save the file using the server-provided filename in the current directory, use the `-O -J` (`--remote-name --remote-header-name`) flags:

```bash
# Option A: Specify conversion in URL path
curl -O -J -X POST https://morph-gateway.onrender.com/morph/json/graphql \
  -F "file=@egg.json"

# Option B: Specify target format directly in the payload/form parameters
curl -O -J -X POST https://morph-gateway.onrender.com/morph \
  -F "file=@egg.json" \
  -F "target=graphql"
```

The server detects the source format from your file's extension (`.json`) or form parameters, synthesizes the schema, and returns `Content-Disposition: attachment; filename="egg.graphql"`. Because of `-O -J`, `curl` saves `egg.graphql` right in the folder where the command was called!

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
