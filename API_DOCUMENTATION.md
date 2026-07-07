# API Documentation

This document provides a guide on how to interact with the Omni-Schema Gateway's endpoints to perform high-performance schema morphing.

## 1. Payload Morphing (File Upload)

Upload a source file and receive the converted schema as a downloadable file in the target format.

### `POST /morph/{source}/{target}`

- **`{source}`**: The protocol format of the file you are uploading.
- **`{target}`**: The desired protocol format for the output file.

#### Supported Conversions

| Source | Target | Output Extension |
|--------|--------|-----------------|
| `json` | `graphql` | `.graphql` |
| `json` | `protobuf` | `.pb` |
| `json` | `msgpack` | `.msgpack` |
| `json` | `parquet` | `.parquet` |
| `json` | `capnproto` | `.capnp` |
| `json` | `hdf5` | `.h5` |
| `json` | `json` | `.json` |

#### Option A: File Upload with Automatic Filename Preservation (Recommended)

Upload a file from your current directory using multipart form data (`-F "file=@egg.json"`). Use `curl -O -J` (`--remote-name --remote-header-name`) so `curl` automatically downloads and saves the converted file directly into your current directory using the base name preserved by the server (e.g., `egg.graphql`).

You can specify the formats in the URL path or directly in the form payload:

```bash
# Via URL Path:
curl -O -J -X POST https://morph-gateway.onrender.com/morph/json/graphql \
  -F "file=@egg.json"

# Via Form Payload (auto-detects source from file extension):
curl -O -J -X POST https://morph-gateway.onrender.com/morph \
  -F "file=@egg.json" \
  -F "target=graphql"
```

#### Option B: Raw Body

Send the payload directly in the request body. Since no filename is attached in raw body mode, the server defaults to `converted.{ext}`.

```bash
curl -X POST https://morph-gateway.onrender.com/morph/json/graphql \
  -H "Content-Type: application/json" \
  -d @egg.json \
  -o converted.graphql
```

#### Response

The server responds with:
- `Content-Type`: The appropriate MIME type for the target format.
- `Content-Disposition`: `attachment; filename="{basename}.{ext}"` (e.g. `egg.graphql`), preserving your uploaded file's name to trigger an automatic local download when using `curl -O -J`.
- The response body contains the raw converted file bytes.

---

## 2. Schema Ingestion

Before morphing complex binary protocols such as Cap'n Proto or Protobuf, the gateway requires structural definitions.

### `POST /system/schema`

Raw schema files (`.proto`, `.capnp`, or `.graphql`) can be uploaded using a multipart form request.

```bash
curl -X POST https://morph-gateway.onrender.com/system/schema \
  -F "file=@schema.proto"
```

---

## 3. GraphQL Subscriptions (WebSockets)

The gateway supports fully native, zero-dependency WebSockets for executing continuous GraphQL subscriptions.

### `GET /graphql/subscriptions`

This endpoint requires standard WebSocket protocol upgrade headers.

```bash
curl --http1.1 -i -N \
  -H "Connection: Upgrade" \
  -H "Upgrade: websocket" \
  -H "Sec-WebSocket-Version: 13" \
  -H "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==" \
  https://morph-gateway.onrender.com/graphql/subscriptions
```
