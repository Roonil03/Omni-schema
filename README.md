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
- **Compact Binary**: [CBOR](https://www.rfc-editor.org/rfc/rfc8949.html), with bidirectional conversion to every other format (100 routes total)
- **Columnar & Big Data**: [Apache Parquet](https://parquet.apache.org/)
- **Hierarchical Multidimensional**: [HDF5](https://www.hdfgroup.org/solutions/hdf5/)
- **Data Serialization**: [Apache Avro](https://avro.apache.org/)
- **REST APIs**: [OData](https://www.odata.org/)
- **Real-Time Streaming**: Native [GraphQL](https://graphql.org/) Subscriptions running over custom RFC 6455 WebSockets

---

## Quick Start (Live Render API)

The Omni-Schema Gateway is hosted on Render. Production endpoints are protected by an API token; obtain a token from the deployment owner and send it as `Authorization: Bearer` or `X-API-Token`.

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
  -H "Authorization: Bearer $OMNI_API_TOKEN" \
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
  -H "Authorization: Bearer $OMNI_API_TOKEN" \
  -F "file=@data.json" \
  -F "target=protobuf"
```

---

### Uploading Custom Schemas
If your target protocols require explicit structural definitions (such as custom Protobuf `.proto` or Cap'n Proto `.capnp` schemas), upload them to the system registry using a standard multipart form request:

```bash
user1@user:~$ curl -X POST https://morph-gateway.onrender.com/system/schema \
  -H "Authorization: Bearer $OMNI_API_TOKEN" \
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
docker build -f Docker/Dockerfile -t omni-schema .
docker run -p 8080:8080 -e PORT=8080 omni-schema
```

Or from the repository root with Compose (local morph without an API token):
```bash
docker compose -f Docker/docker-compose.yml up --build
```
Production Compose should set `OMNI_ENV=production` and `OMNI_API_TOKEN`.

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
| CBOR | yes | yes | no | yes | 1 **OpBinary** envelope | RFC 8949 definite-length subset; text map keys, depth limit 64; rejects tags and indefinite lengths |
| Protobuf | yes | yes | required for faithful types | yes | 1 **OpBinary** envelope | schema-driven; subset `.proto` language |
| GraphQL | result JSON | SDL (morph) / result (stream) | SDL | yes | GraphQL-over-WS **text** envelope | subscription selection/projection; not a full execution engine |
| Avro | OCF | OCF | embedded + optional UIR | yes | **OpBinary** batched OCF | Object Container File, null codec |
| OData | JSON payload | JSON payload | EDM annotations | yes | JSON **text** envelope | **OData JSON response subset** (`@odata.context`, `@odata.type`, `value`) — not `$filter`/`$expand` |
| Cap'n Proto | yes | yes | schema for faithful layout | yes | 1 **OpBinary** envelope | single-segment struct/Text/List subset |
| Parquet | yes | yes | optional | yes | **OpBinary** batched file | Omni Parquet subset v1 (`PAR1`, PLAIN pages) — not parquet-cli certified |
| HDF5 | yes | yes | optional | yes | **OpBinary** batched file | signature + superblock v0 + contiguous datasets — not h5dump certified |

### CBOR verification

CBOR regression coverage verifies actual values across all ten formats, including signed/unsigned 64-bit boundaries, Unicode, nested maps/arrays, empty arrays, nulls, native bytes, and float16/32/64 inputs. Raw-body and multipart curl round trips are checked against Docker. Invalid data and target-subset combinations that would discard values are rejected; see the [CBOR limitations](./API_DOCUMENTATION.md#complete-conversion-matrix-100-pairwise-routes). CBOR parsing and encoding are capped at 100,000 items, including map keys, and 64 nesting levels. HTTP bodies are capped at 10 MiB.

### Streaming semantics

- **Delivery**: **at-most-once / best-effort**. Bounded queues; DropOldest on overflow. Event IDs are deduplicated per subscription.
- **Replay/resume**: **none**. `cursor` in envelopes is informational only; reconnecting clients start live.
- **Ordering**: per subscription, in publish order, until a drop occurs.
- **JSON / OData**: one event per **text** frame (transport envelope).
- **GraphQL**: UIR → GraphQL result `{data:{<alias>: ...}}` → `{"type":"next","id","payload"}` text envelope. Multi-root subscriptions fan out by matching `eventType` to each root field name. Operations must be `subscription`; parse errors and unknown fields are rejected.
- **Binary targets** (Protobuf, MessagePack, CBOR, Cap'n Proto, Avro, Parquet, HDF5): **OpBinary** frames with an `OMNI` header (`eventId`, `format`, `schemaVersion`) then raw codec bytes — **not** Base64-in-JSON.
- **Parquet / HDF5 / Avro**: default `batchSize=16` (override with `?batchSize=`); a batch encodes one container file.
- **Schema version**: bound at subscribe time. If that version is deleted, the subscription receives an error and is closed.
- **Liveness**: server pings every 20s; RFC close handshake on `OpClose`.
- **`/dev/events`**: `?source=` selects any advertised decoder. Render sets `OMNI_DEV_EVENTS=0`; production injection requires both `OMNI_DEV_EVENTS=1` and a valid API token.

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
| `OMNI_API_TOKEN` | In production, required as `Authorization: Bearer` or `X-API-Token` for morphing, schemas, event injection, and subscriptions |
| `OMNI_ENV` | `production` disables `/dev/events` unless overridden |
| `OMNI_DEV_EVENTS` | `0` disables inject; `1` allows it in production |

The Render Blueprint selects the **free Singapore region** and keeps authentication enabled. Free Render storage at `/tmp/registry_store.json` is ephemeral; schemas must be re-registered after restarts, redeploys, or idle shutdowns. Local Docker Compose retains its persistent named volume.

Render cannot move an existing service between regions. Create a new free Docker web service in Singapore from branch `codex/cbor`, using this Blueprint, and obtain its generated API token. Changing `region` in this repository alone does not migrate the current service. See [Render regions](https://render.com/docs/regions).

### Architecture Snapshot
- **Lexers & ASTs**: Constructed natively utilizing `text/scanner` without third-party parsing libraries.
- **Lowering Engine**: Maps complex schema abstractions down to a universal `uir.TypeMap` and `uir.TypeArray`.
- **Codecs**: Synthesizes heavily specified binary and text byte representations directly from the UIR memory pool.
- **WebSockets**: Implements TCP hijacking via `net/http` to securely facilitate real-time GraphQL subscription channels.

### Performance snapshot — 2026-10-07

Warm HTTP samples use the 33-byte JSON payload `{"name":"Ada","id":42,"ok":true}` and a reused HTTP client at concurrency 10. The refreshed Docker JSON → CBOR run uses three warm-up requests followed by 100 measured requests in ten batches. Latency includes the complete response body. The earlier Render JSON → GraphQL sample used one warm-up request and 30 measured requests in three batches on the existing `morph-gateway.onrender.com` service. These are small snapshots from this machine, not production capacity guarantees. The existing Render service does not support CBOR yet (HTTP 400); its region is unverified. Singapore CBOR metrics remain pending deployment from `codex/cbor`. The Blueprint on `main` builds `main`; select `codex/cbor` to deploy this additional format.

The CBOR round-trip microbenchmark uses the same three-field object directly in the codec, with Go 1.25 on Linux/amd64 in Docker on an Intel i9-11900H. It measures encoding plus decoding, excluding HTTP and network latency. Three runs measured 1,177, 1,342, and 1,259 ns/op; the badge reports their median, 1,259 ns/op. Each run allocated 1,504 bytes and 31 allocations per operation. To reproduce: `go test ./internal/codec -run '^$' -bench BenchmarkCBORRoundTrip -benchmem -count=3`. Run the full container matrix with `OMNI_E2E=1 OMNI_E2E_URL=http://localhost:8080 go test ./cmd/server -run TestComposeMorphMatrix`; for authenticated deployments, supply `OMNI_E2E_TOKEN`.

![Docker HTTP conversion routes](https://img.shields.io/badge/Docker_HTTP_routes-100%2F100_passed-brightgreen)
![Docker CBOR p50 at concurrency 10](https://img.shields.io/badge/Docker_CBOR_p50_c10-0.660_ms-blue)
![Docker CBOR p95 at concurrency 10](https://img.shields.io/badge/Docker_CBOR_p95_c10-3.414_ms-blue)
![Docker CBOR p99 at concurrency 10](https://img.shields.io/badge/Docker_CBOR_p99_c10-6.840_ms-blue)
![Docker CBOR success](https://img.shields.io/badge/Docker_CBOR_success-100%2F100-brightgreen)
![Existing Render GraphQL p50 at concurrency 10](https://img.shields.io/badge/Existing_Render_GraphQL_p50_c10-268.182_ms-blue)
![Existing Render GraphQL p95 at concurrency 10](https://img.shields.io/badge/Existing_Render_GraphQL_p95_c10-778.669_ms-blue)
![Existing Render GraphQL success](https://img.shields.io/badge/Existing_Render_GraphQL_success-30%2F30-brightgreen)
![CBOR codec round trip](https://img.shields.io/badge/CBOR_codec_round_trip-1259_ns%2Fop-blue)
