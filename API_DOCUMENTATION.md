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

#### Option A: File Upload (Recommended)

Upload a file using multipart form data. The field name must be `file`.

```bash
curl -X POST https://morph-gateway.onrender.com/morph/json/graphql \
  -F "file=@data.json" \
  -o converted.graphql
```

#### Option B: Raw Body

Send the payload directly in the request body.

```bash
curl -X POST https://morph-gateway.onrender.com/morph/json/graphql \
  -H "Content-Type: application/json" \
  -d @data.json \
  -o converted.graphql
```

#### Response

The server responds with:
- `Content-Type`: The appropriate MIME type for the target format.
- `Content-Disposition`: `attachment; filename="converted.{ext}"` to trigger a file download.
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
