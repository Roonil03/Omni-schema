# API Documentation

Welcome to the Omni-Schema Gateway API manual. This document provides a straightforward guide on how to interact with the gateway's endpoints to perform high-performance schema morphing.

## 1. Schema Ingestion

Before morphing complex binary protocols like Cap'n Proto or Protobuf, the gateway needs to understand your structural definitions.

### `POST /system/schema`

Upload your raw schema files (like `.proto`, `.capnp`, or `.graphql`) using a multipart form request.

#### Request Example (cURL)

```bash
curl -X POST http://localhost:8080/system/schema \
  -H "Content-Type: multipart/form-data" \
  -F "file=@schema.proto"
```

## 2. Payload Morphing

Once your schemas are loaded, you can translate incoming payloads from one protocol format directly into another in real-time.

### `POST /morph/{source}/{target}`

- **`{source}`**: The protocol format of the data you are sending.
- **`{target}`**: The protocol format you want to receive back.

#### Valid Translators
- `protobuf`
- `graphql`
- `json`
- `odata`
- `avro`
- `thrift`
- `parquet`
- `msgpack`
- `capnproto`
- `hdf5`

#### Request Example (cURL)

```bash
curl -X POST http://localhost:8080/morph/json/protobuf \
  -H "Content-Type: application/json" \
  -d '{"id": 123, "name": "Test"}'
```

#### Response Example

```text
Morphed json to protobuf natively without dependencies. Original payload: 45 bytes.
```

---

## 3. GraphQL Subscriptions (WebSockets)

The gateway supports fully native, zero-dependency WebSockets for executing continuous GraphQL subscriptions. 

### `GET /graphql/subscriptions`

This endpoint requires WebSocket protocol upgrade headers.

#### Request Example (cURL WebSocket Emulation)

```bash
curl -i -N \
  -H "Connection: Upgrade" \
  -H "Upgrade: websocket" \
  -H "Sec-WebSocket-Version: 13" \
  -H "Sec-WebSocket-Key: SGVsbG8sIHdvcmxkIQ==" \
  http://localhost:8080/graphql/subscriptions
```

#### Response Example

```text
HTTP/1.1 101 Switching Protocols
Upgrade: websocket
Connection: Upgrade
Sec-WebSocket-Accept: <Computed-SHA1-Base64-Hash>
```
