# Omni-Schema

![License](https://img.shields.io/badge/License-MIT-yellow?style=for-the-badge)
![Go](https://img.shields.io/badge/Go-1.25-00ADD8?style=for-the-badge&logo=go&logoColor=white)
![Protocols](https://img.shields.io/badge/Protocols-10-blue?style=for-the-badge)

Omni-Schema is an advanced, high-performance API morphing service built entirely from scratch in Go with zero external dependencies. Operating on an Analysis-Synthesis compiler model, the gateway translates arbitrary payloads between highly complex binary and text protocols.

---

## Features

The gateway acts as a universal schema and payload translator. It parses incoming structures into a Universal Intermediate Representation (UIR) memory graph, enabling seamless, native morphing between disparate protocols.

Supported formats and protocols include:
- **Standard Text Formats**: [JSON](https://www.json.org/), [Protobuf](https://protobuf.dev/)
- **Zero-Copy & Memory-Aligned**: [Cap'n Proto](https://capnproto.org/)
- **Schemaless Binary**: [MessagePack](https://msgpack.org/)
- **Columnar & Big Data**: [Apache Parquet](https://parquet.apache.org/)
- **Hierarchical Multidimensional**: [HDF5](https://www.hdfgroup.org/solutions/hdf5/)
- **Real-Time Streaming**: Native [GraphQL](https://graphql.org/) Subscriptions running over custom RFC 6455 WebSockets

---

## Quick Start (Live Render API)

The Omni-Schema Gateway is fully hosted on Render and ready for immediate use. You do not need to install Go, Docker, or download the repository to use the live morphing APIs.

**Production Base URL**: `https://morph-gateway.onrender.com`

> [!TIP]
> **Windows Users**: In PowerShell, `curl` is often an alias for `Invoke-WebRequest`. To use standard cURL flags like `-O -J`, type `curl.exe` instead of `curl`.

### 1. Converting a File (Automatic Filename Preservation)
Upload any schema or data file from your current directory and download the converted output directly back into the same directory. The server automatically preserves your file's base name (e.g., uploading `data.json` converts and saves locally as `data.graphql`).

To let `curl` automatically save the file using the server-provided filename in your current directory, use the `-O -J` (`--remote-name --remote-header-name`) flags:

```bash
# Option A: Specify conversion in URL path (/morph/{source}/{target})
curl -O -J -X POST https://morph-gateway.onrender.com/morph/json/graphql \
  -F "file=@data.json"

# Option B: Specify target format in form parameters (auto-detects source from .json extension)
curl -O -J -X POST https://morph-gateway.onrender.com/morph \
  -F "file=@data.json" \
  -F "target=graphql"
```

The server detects the source format from your file's extension or parameters, synthesizes the target format, and returns `Content-Disposition: attachment; filename="data.graphql"`. Because of `-O -J`, `curl` saves `data.graphql` right in the folder where the command was executed!

### 2. Uploading Custom Schemas
If your target protocols require explicit structural definitions (such as custom Protobuf `.proto` or GraphQL `.graphql` schemas), upload them using a standard multipart form request:

```bash
curl -X POST https://morph-gateway.onrender.com/system/schema \
  -F "file=@custom_schema.proto"
```

---

## Local Development & Self-Hosting

For developers and contributors wishing to run or extend Omni-Schema locally, the project is engineered with zero external dependencies using standard Go library packages.

### Prerequisites
- **Go**: Version 1.25 or newer
- **Docker** *(Optional)*: For containerized deployments

### Running Locally with Go
1. Clone the repository:
   ```bash
   git clone https://github.com/Roonil03/Omni-schema.git
   cd Omni-schema
   ```
2. Start the server (defaults to port `8080`):
   ```bash
   # Linux / macOS / Git Bash
   PORT=8080 go run cmd/server/main.go

   # Windows PowerShell
   $env:PORT="8080"; go run cmd/server/main.go
   ```
3. Test your local instance:
   ```bash
   curl -O -J -X POST http://localhost:8080/morph/json/graphql \
     -F "file=@testing_files/sample_payload.json"
   ```

### Running with Docker
Build and run the multi-stage container:
```bash
docker build -t omni-schema .
docker run -p 8080:8080 -e PORT=8080 omni-schema
```

---

## Documentation & Architecture

For detailed API specifications, supported format matrices, WebSocket subscription protocols, and error code references, consult the official documentation:

- **[API Documentation](./API_DOCUMENTATION.md)**: Full endpoint reference and usage guide.
- **[Design Document](./Design.md)**: Architectural breakdown of the Analysis-Synthesis engine and UIR graph.

### Architecture Snapshot
- **Lexers & ASTs**: Constructed natively utilizing `text/scanner` without third-party parsing libraries.
- **Lowering Engine**: Maps complex schema abstractions down to a universal `uir.TypeMap` and `uir.TypeArray`.
- **Codecs**: Synthesizes heavily specified binary and text byte representations directly from the UIR memory pool.
- **WebSockets**: Implements TCP hijacking via `net/http` to securely facilitate real-time GraphQL subscription channels.
