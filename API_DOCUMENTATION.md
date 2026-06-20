# Omni-Schema Gateway API Documentation

This document describes how to use the Omni-Schema Gateway endpoints to upload schemas and execute payload morphing.

## 1. Schema Ingestion API

Use this endpoint to upload and register custom schemas (e.g., `.proto`, `.graphql`, OpenAPI `.json`). The gateway dynamically parses these schemas from scratch and builds a Unified Intermediate Representation (UIR).

### `POST /system/schema`

> [!NOTE]
> This endpoint requires a `multipart/form-data` request containing the schema files.

#### Request Example (cURL)

```bash
curl -X POST http://localhost:8080/system/schema \
  -H "Content-Type: multipart/form-data" \
  -F "schema=@/path/to/service.proto" \
  -F "schema=@/path/to/schema.graphql"
```

#### Response Example

```json
{
  "status": "Schema successfully registered in UIR."
}
```

---

## 2. Payload Morphing API

This is the core translation execution endpoint. Send a payload utilizing your predefined source schema format, and the gateway will parse it, traverse the UIR graph, and dynamically synthesize the target format natively.

### `POST /morph/{source}/{target}`

> [!IMPORTANT]
> The source and target path parameters determine the lexer and synthesizer pipelines used. 

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

Converting a standard JSON payload into a Protobuf binary payload.

```bash
curl -X POST http://localhost:8080/morph/json/protobuf \
  -H "Content-Type: application/json" \
  -d '{"id": 1, "name": "API Morphing Gateway"}'
```

#### Response Example

*(Binary byte stream of the Protobuf output)*

```text
Morphed json to protobuf natively without dependencies. Original payload: 45 bytes.
```

---

## 3. GraphQL Subscriptions (WebSockets)

The gateway supports fully native, zero-dependency WebSockets for executing continuous GraphQL subscriptions. 

### `GET /graphql/subscriptions`

> [!NOTE]
> This endpoint requires WebSocket protocol headers.

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
