# API Documentation

This document provides a comprehensive guide for interacting with the Omni-Schema Gateway's endpoints to perform high-performance schema and payload morphing.

---

## Environments & Base URLs

The gateway can be accessed via the live production deployment or run locally:

| Environment | Base URL | Description |
| :--- | :--- | :--- |
| **Production (Render)** | `https://morph-gateway.onrender.com` | Fully hosted live service (zero setup required) |
| **Local / Self-Hosted** | `http://localhost:8080` | Local Go or Docker instance (configured via `PORT` env var) |

> [!TIP]
> **Windows PowerShell Users**: In PowerShell, `curl` is often aliased to `Invoke-WebRequest`. To use standard cURL command-line flags (such as `-O` and `-J`), invoke `curl.exe` explicitly instead of `curl`.

---

## 1. Payload & Schema Morphing

Upload a source file or raw data stream and receive the synthesized output as a downloadable file in the target format.

### Endpoints
- **`POST /morph/{source}/{target}`**: Explicit routing via URL path.
- **`POST /morph`**: Dynamic routing via multipart form fields or URL query parameters.

### Supported Conversions

| Source Format | Target Format | Output Extension | MIME Content-Type |
| :--- | :--- | :--- | :--- |
| `json` | `graphql` | `.graphql` | `application/graphql` |
| `json` | `protobuf` | `.pb` | `application/protobuf` |
| `json` | `msgpack` | `.msgpack` | `application/msgpack` |
| `json` | `parquet` | `.parquet` | `application/parquet` |
| `json` | `capnproto` | `.capnp` | `application/capnproto` |
| `json` | `hdf5` | `.h5` | `application/x-hdf5` |
| `json` | `avro` | `.avro` | `application/avro` |
| `json` | `odata` | `.json` | `application/json` |
| `json` | `json` | `.json` | `application/json` |

---

### Method A: File Upload with Automatic Filename Preservation (Recommended)

When uploading a file using multipart form data (`-F "file=@data.json"`), the server automatically extracts your file's base name and preserves it in the response headers.

By combining this with cURL's `--remote-name --remote-header-name` (`-O -J` or `-OJ`) flags, the converted file is automatically downloaded and saved directly into your current working directory with the matching base name (e.g., `data.json` converts to `data.graphql`).

#### Option 1: Routing via URL Path
```bash
curl -O -J -X POST https://morph-gateway.onrender.com/morph/json/graphql \
  -F "file=@data.json"
```

#### Option 2: Routing via Form/Query Parameters (Auto-detects Source)
If you do not specify the source format, the server automatically detects it from your uploaded file's extension (`.json` -> `json`):
```bash
curl -O -J -X POST https://morph-gateway.onrender.com/morph \
  -F "file=@data.json" \
  -F "target=protobuf"
```

---

### Method B: Raw Body Streaming

For automated scripts, piping, or in-memory data buffers where file upload headers are not needed, send the raw payload directly in the request body. In raw body mode, the server defaults the return filename to `converted.{ext}`.

```bash
curl -X POST https://morph-gateway.onrender.com/morph/json/graphql \
  -H "Content-Type: application/json" \
  -d @data.json \
  -o output.graphql
```

---

### Response Structure

On a successful morphing request (`200 OK`), the server returns:
- **`Content-Type`**: The MIME type corresponding to the target format.
- **`Content-Disposition`**: `attachment; filename="{basename}.{ext}"` (e.g., `data.graphql`), instructing browsers and CLI clients (`curl -O -J`) to save the file locally.
- **`Content-Length`**: The exact byte length of the synthesized output.
- **Body**: The raw binary or text bytes of the converted schema/payload.

---

## 2. Custom Schema Ingestion

Before morphing complex binary protocols that require strict pre-defined schemas (such as Cap'n Proto or Protobuf), upload your structural definitions to the system registry. The registry automatically hashes schemas for version control. 

> [!NOTE]
> In local development or self-hosted environments, the registry state is automatically persisted to `registry_store.json` in the current working directory to ensure your uploaded schemas survive server restarts.

### `POST /system/schema`

Upload `.proto`, `.capnp`, or `.graphql` schema files using a standard multipart form request:

```bash
curl -X POST http://localhost:8080/system/schema \
  -F "name=transaction" \
  -F "file=@custom_schema.proto"
```
**Success Response (`200 OK`)**:
```json
{
  "status": "registered",
  "name": "transaction",
  "version": "76dd4e9f",
  "format": "proto"
}
```

Once registered, you can explicitly request this schema during a morph via the `?schema=` query parameter:
```bash
curl -X POST "http://localhost:8080/morph/json/graphql?schema=transaction" -F "file=@data.json"
```

---

## 3. Real-Time GraphQL Subscriptions (WebSockets)

The gateway supports fully native, zero-dependency WebSockets (RFC 6455) for executing continuous GraphQL streaming subscriptions over TCP hijacking. It includes a complete frame-level masking and binary parsing engine.

### `GET /graphql/subscriptions`

This endpoint accepts standard WebSocket connections. You can bind to a registered schema by passing `?schema=name` in the URL.

The protocol expects JSON text frames:
1. **Init**: Client sends `{"type":"connection_init"}`. Server replies with `{"type":"connection_ack"}`.
2. **Subscribe**: Client sends `{"id":"sub_1", "type":"subscribe", "payload":{"query":"...","operationName":"A"}}`. Multiple operations require `operationName`. Aliases and fragments are applied to the payload key and selection set.
3. **Events**: Server pushes `next` frames. GraphQL targets use a GraphQL-over-WebSocket envelope. Binary targets use a JSON envelope with `format`, `schemaVersion`, `eventId`, and base64 `payload`.
4. **Complete**: `{"type":"complete","id":"sub_1"}` ends that subscription only; the socket stays up.
5. Query parameters: `schema`, `target`, `batchSize` (Parquet/HDF5/Avro containers).

Delivery is **at-most-once / best-effort**. The subscription binds the active schema version at connect time.

### Event injection `POST /dev/events`

JSON: `{"type":"transactionUpdated","data":{...},"format":"json","id":"optional"}`.  
Non-JSON: `POST /dev/events?source=protobuf&type=transactionUpdated` with raw body.  
Disabled when `OMNI_ENV=production` unless `OMNI_DEV_EVENTS=1`.

### Operations & telemetry

- `GET /readyz` — registry initialized
- `GET /metrics` — counters and latency percentiles
- `GET /healthz` — liveness plus metric snapshot
- `POST /system/schema/activate?name=&version=`
- `POST /system/schema/deprecate?name=&version=`
- `GET /system/schema/diff?name=&from=&to=`
- Morph: `?schema=&type=` selects the payload type (never `Children[0]` by default)
- `X-Request-ID` is accepted or generated; `X-Conversion-Kind` reports lossless / safe_coercion / lossy
- If `OMNI_API_TOKEN` is set, schema writes require `Authorization: Bearer` or `X-API-Token`

OData is an **OData JSON response subset** (`@odata.context`, `@odata.type`, `value`) with EDM type mapping. It is not an OData query engine.

---

## 4. HTTP Status Codes & Error Handling

| Status Code | Meaning | Description |
| :--- | :--- | :--- |
| **`200 OK`** | Success | Payload successfully parsed, synthesized, and returned. |
| **`400 Bad Request`** | Client Error | Missing payload, unsupported source format, or syntax error in input data. |
| **`405 Method Not Allowed`** | Routing Error | Attempting to use `GET`, `PUT`, or `DELETE` on a `POST`-only endpoint. |
| **`500 Internal Server Error`** | Synthesis Error | Failure during UIR graph traversal or target codec byte generation. |

