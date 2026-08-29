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
- **Data Serialization**: [Apache Avro](https://avro.apache.org/)
- **REST APIs**: [OData](https://www.odata.org/)
- **Real-Time Streaming**: Native [GraphQL](https://graphql.org/) Subscriptions running over custom RFC 6455 WebSockets

---

## Quick Start (Live Render API)

The Omni-Schema Gateway is fully hosted on Render and ready for immediate use. You do not need to install Go, Docker, or download the repository to use the live morphing APIs.

**Production Base URL**: `https://morph-gateway.onrender.com`

> [!TIP]
> **Windows Users**: In PowerShell, `curl` is often an alias for `Invoke-WebRequest`. To use standard cURL flags like `-O -J`, type `curl.exe` instead of `curl`.

### How to Properly Use the API (Important Rules)
To ensure seamless file uploads and conversions without client-side or parsing errors, follow these essential guidelines:
1. **Execute from the Directory Containing Your File**: When passing `-F "file=@filename"`, cURL searches for `filename` inside your **current working directory**. Ensure you `cd` into the folder where your file is located before running the command (otherwise cURL throws error `(26) Failed to open/read local data`).
2. **Do NOT Override Multipart Headers**: Do **not** manually add `-H "Content-Type: multipart/form-data"` when using `-F`. cURL automatically generates the required multipart boundary parameter (e.g., `boundary=------------------------abcdef1234567890`). Overriding this header strips the boundary parameter, causing backend server parsing failures.
3. **Use `-O -J` for Automatic Local Downloads**: Adding `-O -J` (`--remote-name --remote-header-name`) tells cURL to read the server's `Content-Disposition` header and automatically download and save the converted file directly into your calling folder with its base name preserved (e.g., uploading `data.json` converts and saves locally as `data.graphql`).

---

### Complete Terminal Walkthrough (Example as `user1@user`)

Here is an end-to-end example demonstrating how a developer (`user1@user`) creates a file in their terminal, converts it via the live Render API, and receives the translated schema directly in their working directory:

```bash
# Step 1: Check your current working directory and create a sample JSON payload
user1@user:~$ pwd
/home/user1
user1@user:~$ echo '{"id": 101, "name": "Alice", "role": "admin", "active": true}' > data.json

# Step 2: Upload data.json to convert it to GraphQL (using -O -J)
# Notice we do NOT add -H "Content-Type: multipart/form-data"!
user1@user:~$ curl -O -J -X POST https://morph-gateway.onrender.com/morph/json/graphql \
  -F "file=@data.json"

# Step 3: Check your directory: data.graphql was automatically downloaded and saved!
user1@user:~$ ls -l
total 8
-rw-r--r-- 1 user1 user 64 Jul  8 12:30 data.graphql
-rw-r--r-- 1 user1 user 65 Jul  8 12:30 data.json

# Step 4: View the converted GraphQL schema
user1@user:~$ cat data.graphql
type Root {
  id: Float!
  name: String!
  role: String!
  active: Boolean!
}
```

#### Alternative Routing: Form Parameters
You can also specify the target format via form parameters instead of the URL path. If the source format is omitted, the server automatically detects it from your file's extension (`.json` -> `json`):

```bash
user1@user:~$ curl -O -J -X POST https://morph-gateway.onrender.com/morph \
  -F "file=@data.json" \
  -F "target=protobuf"
```

---

### Uploading Custom Schemas
If your target protocols require explicit structural definitions (such as custom Protobuf `.proto` or Cap'n Proto `.capnp` schemas), upload them to the system registry using a standard multipart form request:

```bash
user1@user:~$ curl -X POST https://morph-gateway.onrender.com/system/schema \
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
     -F "file=@data.json"
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

- **[API Documentation](./API_DOCUMENTATION.md)**: Full endpoint reference, capability matrix, and subset boundaries.
- **[Credits](./Credits.md)**: Acknowledgments and roles of the engineering team members who contributed to this project.
- **GraphQL Schema Ingestion**: Parses GraphQL SDL, including `interface`, `union`, `enum`, `input`, `scalar`, `schema`, fragments, aliases, and nested types (`[[Type!]!]!`).
- **Protobuf Integration**: Schema-driven translation (`.proto` files) preserving field numbers, wire types, signedness, enums, oneofs, maps, nested types, and services.
- **WebSocket Streaming**: Custom RFC 6455 broker with masked-client enforcement, close handshake, deadlines, at-most-once delivery (`DropOldest`), and format conversion.
- **Codecs**: JSON, MessagePack, Protobuf, Avro OCF, OData JSON subset, GraphQL SDL/result, plus scoped Parquet/HDF5/Cap'n Proto implementations with round-trip tests.
- **Production Telemetry**: `/healthz`, `/readyz`, `/metrics` (counters + parse/convert/encode/stream P50/P95/P99), request IDs, payload limits, structured `slog`.
- **Zero Third-Party Dependencies**: The entire project uses only the Go standard library.

### Capability matrix

Every “supported” decode/encode path has automated round-trip coverage in `internal/codec`. Status is **subset** unless noted.

| Format | Decode | Encode | Schema | HTTP morph | Streaming | Status |
| :--- | :---: | :---: | :---: | :---: | :---: | :--- |
| JSON | yes | yes | optional | yes | 1 event / text frame | complete for objects/arrays |
| MessagePack | yes | yes | no | yes | 1 **OpBinary** envelope | complete schemaless subset |
| Protobuf | yes | yes | required for faithful types | yes | 1 **OpBinary** envelope | schema-driven; subset `.proto` language |
| GraphQL | result JSON | SDL (morph) / result (stream) | SDL | yes | GraphQL-over-WS **text** envelope | subscription selection/projection; not a full execution engine |
| Avro | OCF | OCF | embedded + optional UIR | yes | **OpBinary** batched OCF | Object Container File, null codec |
| OData | JSON payload | JSON payload | EDM annotations | yes | JSON **text** envelope | **OData JSON response subset** (`@odata.context`, `@odata.type`, `value`) — not `$filter`/`$expand` |
| Cap'n Proto | yes | yes | schema for faithful layout | yes | 1 **OpBinary** envelope | single-segment struct/Text/List subset |
| Parquet | yes | yes | optional | yes | **OpBinary** batched file | Omni Parquet subset v1 (`PAR1`, PLAIN pages) — not parquet-cli certified |
| HDF5 | yes | yes | optional | yes | **OpBinary** batched file | signature + superblock v0 + contiguous datasets — not h5dump certified |

### Streaming semantics

- **Delivery**: **at-most-once / best-effort**. Bounded queues; DropOldest on overflow. Event IDs are deduplicated per subscription.
- **Replay/resume**: **none**. `cursor` in envelopes is informational only; reconnecting clients start live.
- **Ordering**: per subscription, in publish order, until a drop occurs.
- **JSON / OData**: one event per **text** frame (transport envelope).
- **GraphQL**: UIR → GraphQL result `{data:{<alias>: ...}}` → `{"type":"next","id","payload"}` text envelope. Multi-root subscriptions fan out by matching `eventType` to each root field name. Operations must be `subscription`; parse errors and unknown fields are rejected.
- **Binary targets** (Protobuf, MessagePack, Cap'n Proto, Avro, Parquet, HDF5): **OpBinary** frames with an `OMNI` header (`eventId`, `format`, `schemaVersion`) then raw codec bytes — **not** Base64-in-JSON.
- **Parquet / HDF5 / Avro**: default `batchSize=16` (override with `?batchSize=`); a batch encodes one container file.
- **Schema version**: bound at subscribe time. If that version is deleted, the subscription receives an error and is closed.
- **Liveness**: server pings every 20s; RFC close handshake on `OpClose`.
- **`/dev/events`**: `?source=` selects any advertised decoder. Disabled in production unless `OMNI_DEV_EVENTS=1`.

### Schema-dependent codecs

- `?sourceSchema=` / `?targetSchema=` and `?sourceType=` / `?targetType=` (or shared `?schema=` / `?type=`).
- Missing named types return an error (no `Children[0]` fallback). Ambiguous schemas require `type=`.
- Protobuf and Cap'n Proto use the registered schema as the transformation contract when provided.
- Persisted registry reconstruction **fails closed** for unsupported formats.

### Auth and tenants

- `OMNI_ENV=production` requires `OMNI_API_TOKEN` on morph, schema, events, and subscriptions.
- Optional `X-Tenant-ID` namespaces schema names (`tenant/name`).

### Environment

| Variable | Purpose |
| :--- | :--- |
| `PORT` | Listen port (default `8080`) |
| `REGISTRY_PATH` | Schema registry JSON file (default `registry_store.json`) |
| `OMNI_API_TOKEN` | If set, required as `Authorization: Bearer` or `X-API-Token` for schema mutation and (when set) event inject |
| `OMNI_ENV` | `production` disables `/dev/events` unless overridden |
| `OMNI_DEV_EVENTS` | `0` disables inject; `1` allows it in production |

Production on Render is **single-instance**. The JSON registry is local disk, not a multi-node shared store. Use a persistent disk or an external registry if you need durable shared schema state.

### Architecture Snapshot
- **Lexers & ASTs**: Constructed natively utilizing `text/scanner` without third-party parsing libraries.
- **Lowering Engine**: Maps complex schema abstractions down to a universal `uir.TypeMap` and `uir.TypeArray`.
- **Codecs**: Synthesizes heavily specified binary and text byte representations directly from the UIR memory pool.
- **WebSockets**: Implements TCP hijacking via `net/http` to securely facilitate real-time GraphQL subscription channels.
