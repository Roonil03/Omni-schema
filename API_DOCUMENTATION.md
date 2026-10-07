# API reference

Omni-Schema receives data, maps it to a selected schema [field names and types] when requested, and returns the target bytes. The sections below describe the [server implementation](./cmd/server/main.go).

## Endpoint index

Each checkmark links to the endpoint's inputs, output, and limits.

| Method | Endpoint | Purpose | Details |
| :--- | :--- | :--- | :---: |
| POST | `/morph/{source}/{target}` | Convert a file or request body | [✓](#conversion-requests) |
| POST | `/morph` or `/morph/{target}` | Convert using query or form fields | [✓](#conversion-requests) |
| POST | `/system/schema` | Register a schema version | [✓](#register-a-schema) |
| GET | `/system/schema?name=` | List versions of a named schema | [✓](#list-schema-versions) |
| POST | `/system/schema/activate` | Select an active version | [✓](#activate-a-version) |
| POST | `/system/schema/deprecate` | Mark a version deprecated | [✓](#deprecate-a-version) |
| GET | `/system/schema/diff` | Compare two versions | [✓](#compare-versions) |
| GET | `/graphql/subscriptions` | Upgrade to a live event connection | [✓](#subscription-requests) |
| POST | `/dev/events` | Publish a development event | [✓](#event-injection) |
| GET | `/healthz` | Get server status and metrics | [✓](#health) |
| GET | `/readyz` | Check registry initialization | [✓](#readiness) |
| GET | `/metrics` | Get counters and timing measurements | [✓](#metrics) |

The lifecycle endpoints accept the methods shown by convention; their handlers do not currently reject other methods. Health and metrics handlers also do not enforce GET. Conversion, registration, and event injection do enforce their listed methods. See [route handling](./cmd/server/main.go).

[Formats](#formats) · [Conversion flow](#how-conversion-works) · [Authentication](#authentication-and-client-identity) · [Errors](#4-http-status-codes--error-handling) · [Configuration](#configuration) · [Performance](#performance-measurements)

## Base URLs

Use `http://localhost:8080` for the default local server. The hosted address is [https://morph-gateway.onrender.com](https://morph-gateway.onrender.com). Hosted requests require a deployment-issued token; confirm its deployed formats before use. [README](./README.md#use-the-render-api) shows a hosted request.

Commands here use Bash, a running local server, and no token. For a protected server, add `-H "Authorization: Bearer $OMNI_API_TOKEN"`. In PowerShell, use `curl.exe`, join continued commands onto one line, and use `$env:OMNI_API_TOKEN`.

## Authentication and client identity

Set `OMNI_ENV=production` and `OMNI_API_TOKEN` to protect conversion, schema operations, subscriptions, and event injection. Send the exact configured token as `Authorization: Bearer <token>` or `X-API-Token: <token>`. Setting a token also protects these endpoints outside production. Production without a configured token returns `401`. Health, readiness, and metrics are not token-protected. See [authentication checks](./cmd/server/main.go).

The rate limit [maximum requests in a time window] is 120 requests per minute per client key, across all routes. The key uses the first `X-Forwarded-For` address plus a token fingerprint [hash identifying the supplied token], when present. Without a forwarded address it uses the token fingerprint, or the remote host when no token is supplied. A rejected request returns `429` and `Retry-After` seconds. Configure a trusted proxy [server forwarding client requests] to replace client-supplied forwarding headers. See [rate-limit code and tests](./cmd/server/rate_limit_test.go).

`X-Tenant-ID`, or the `tenant` query parameter, prefixes names during registration, listing, conversion lookup, and subscription lookup. This is a naming convention, not separate authentication for each tenant [group with its own schema names]. Activation, deprecation, and diff require the full prefixed name in `name`; they do not apply that header themselves. See [tenant handling](./cmd/server/main.go).

`X-Request-ID` is echoed on responses, or generated when absent. Treat it as a request label, not an authentication value.

## Formats

The links in the name column lead to official format documentation. Checkmarks link to this project's conversion and streaming behavior. Sources and targets use the identifiers below; aliases are case-insensitive. See [format normalization](./internal/codec/formats.go) and [output headers](./cmd/server/main.go).

| Format | Identifier and aliases | Output file | Content type | Convert | Stream |
| :--- | :--- | :--- | :--- | :---: | :---: |
| [JSON](https://www.json.org/json-en.html) | `json` | `.json` | `application/json` | [✓](#format-limits) | [✓](#event-output) |
| [MessagePack](https://msgpack.org/) | `msgpack`, `messagepack`, `msgpck` | `.msgpack` | `application/msgpack` | [✓](#format-limits) | [✓](#event-output) |
| [CBOR](https://www.rfc-editor.org/rfc/rfc8949.html) | `cbor` | `.cbor` | `application/cbor` | [✓](#cbor-limits) | [✓](#event-output) |
| [Protobuf](https://protobuf.dev/) | `protobuf`, `proto`, `pb` | `.pb` | `application/protobuf` | [✓](#schema-selection) | [✓](#event-output) |
| [Cap'n Proto](https://capnproto.org/) | `capnproto`, `capnp` | `.capnp` | `application/capnproto` | [✓](#schema-selection) | [✓](#event-output) |
| [GraphQL](https://graphql.org/) | `graphql`, `gql` | `.graphql` | `application/graphql` | [✓](#format-limits) | [✓](#event-output) |
| [Avro](https://avro.apache.org/) | `avro` | `.avro` | `application/avro` | [✓](#format-limits) | [✓](#event-output) |
| [OData](https://www.odata.org/) | `odata` | `.json` | `application/json` | [✓](#format-limits) | [✓](#event-output) |
| [Parquet](https://parquet.apache.org/) | `parquet`, `pq` | `.parquet` | `application/parquet` | [✓](#format-limits) | [✓](#event-output) |
| [HDF5](https://www.hdfgroup.org/solutions/hdf5/) | `hdf5`, `h5`, `hdf` | `.h5` | `application/x-hdf5` | [✓](#format-limits) | [✓](#event-output) |

### Complete conversion matrix (100 pairwise routes)

All ten advertised identifiers can be used as both source and target. This gives 100 ordered routes, including same-format conversions. The [HTTP matrix](./cmd/server/morph_matrix_test.go) exercises a common fixture on every route. A checkmark means the route exists, not that all values or external files work.

### Format limits

JSON and MessagePack carry supported scalar [single value], object, and array types. JSON integer input preserves signed and unsigned 64-bit values; larger integers and floating-point overflow or nonzero underflow return `400`. JSON cannot carry NaN [a special value meaning not a number] or Infinity. See [JSON parsing](./internal/lexer/json.go) and [MessagePack](./internal/codec/msgpack.go).

Protobuf and Cap'n Proto require a registered schema for faithful field names and types. Without one, generic mappings cannot recover the original names and types. The Cap'n Proto implementation supports single-segment structs, text, and lists. See [Protobuf](./internal/codec/protobuf.go) and [Cap'n Proto](./internal/codec/capnproto.go).

HTTP GraphQL output is SDL [text that defines GraphQL types]. It is a schema description, not a data round trip. The GraphQL decoder accepts result JSON or supported SDL through [ParseGraphQL](./internal/codec/graphql.go). Subscriptions produce selected result data. The project does not implement a full GraphQL query execution engine.

Avro uses OCF [Avro's file container] with the null codec [uncompressed file blocks]. Parquet uses the project's `PAR1` subset with PLAIN pages [values stored without a page encoding transform]. HDF5 uses its signature, superblock v0 [file layout header], and contiguous datasets. Internal round trips do not establish compatibility with all external readers. See [Avro](./internal/codec/avro.go), [Parquet](./internal/codec/parquet.go), [HDF5](./internal/codec/hdf5.go), and [interoperability tests](./internal/codec/interop_test.go).

OData supports JSON response objects containing `@odata.context`, `@odata.type`, and `value`, with supported EDM [OData's data type model] annotations. It does not execute `$filter`, `$expand`, or other OData queries. See [OData](./internal/codec/odata.go).

### CBOR limits

The [CBOR implementation](./internal/codec/cbor.go) accepts objects with text keys, arrays, UTF-8 strings, bytes, 64-bit integers, floating-point values, booleans, and null. It accepts declared lengths only. It rejects tags [extra type labels], arbitrary map keys, duplicate keys, trailing bytes, more than 64 nesting levels, and more than 100,000 items including map keys.

The [target validation](./internal/codec/cbor_target.go) rejects unsupported CBOR combinations with `400`:

- JSON and OData encode bytes as base64 [text representation of binary data]. They reject NaN and Infinity.
- Protobuf and Cap'n Proto reject explicit null. Cap'n Proto requires an object.
- Avro, Parquet, and HDF5 accept flat records or arrays of records with matching fields and types. They reject nested containers and unsigned integers above `MaxInt64`. A single object may decode as a one-record array.
- Parquet and HDF5 reject explicit null fields. Parquet also rejects native bytes.
- GraphQL schema output requires an object, nonempty object types, and valid GraphQL field names.

## How conversion works

The [conversion handler](./cmd/server/main.go) follows these steps:

1. Read a file or request body and resolve source and target identifiers.
2. Load the active source and target schemas, if requested.
3. Decode the source into UIR [shared in-memory data representation].
4. If a target schema is supplied, select its type and project [map data fields to schema fields] the data. Unknown fields are ignored. Missing fields may become null, except for Protobuf and Cap'n Proto output.
5. Check supported target values, then encode the result. The HTTP GraphQL target generates SDL.
6. Return target bytes with a filename, content type, and conversion report header.

See [decoder registration](./internal/codec/registry.go), [schema type selection](./internal/uir/schema.go), [projection](./internal/uir/project.go), and [format options](./internal/codec/options.go).

## 1. Payload & schema morphing

### Conversion requests

`POST /morph/{source}/{target}` takes explicit format names. `POST /morph/{target}` takes the source from query, form, or uploaded filename. `POST /morph` takes both from query or form. Precedence is path, then query, then form; source detection from the uploaded extension is the final fallback. A raw request needs an explicit source. See [routing](./cmd/server/main.go).

Create `data.json` with `{"id":42,"name":"Ada","active":true}`, then upload it:

```bash
curl --fail-with-body -OJ http://localhost:8080/morph/json/msgpack \
  -F "file=@data.json"

curl --fail-with-body -OJ http://localhost:8080/morph \
  -F "file=@data.json" -F "target=cbor"
```

The first command saves `data.msgpack`; the second saves `data.cbor`. Do not set a multipart [request containing separate named fields] content type manually; curl supplies the boundary [separator between uploaded fields].

For raw binary input, use `--data-binary` to keep bytes unchanged:

```bash
curl --fail-with-body "http://localhost:8080/morph?source=cbor&target=json" \
  -H "Content-Type: application/cbor" --data-binary @data.cbor \
  -o roundtrip.json
```

Multipart requests accept `file`, or a text `payload` field with `data` as a fallback. The request body limit is 10 MiB, including multipart overhead. Missing data and malformed uploads return `400`. See [request reading](./cmd/server/main.go).

### Schema selection

All schema and type selectors are query parameters:

| Parameter | Use |
| :--- | :--- |
| `sourceSchema` | Active schema for source decoding |
| `sourceType` | Named source type |
| `targetSchema` | Active schema for field mapping and encoding |
| `targetType` | Named target type |
| `schema` | Fallback for both schema selectors |
| `type` | Fallback for both type selectors |

An explicitly named missing schema returns `404`. Missing or ambiguous named types return `400`; select a type when the schema contains several candidates. Unknown input fields can be dropped. An empty encoded result returns `400`. Selected output fields are returned directly without an extra root wrapper. See [type resolution](./internal/uir/schema.go) and [HTTP schema tests](./cmd/server/cbor_variations_test.go).

### Conversion output

Success is `200` with raw bytes, not a JSON download envelope [object wrapping a separate file]:

| Header | Returned value |
| :--- | :--- |
| `Content-Type` | Target content type from the format table |
| `Content-Disposition` | `attachment; filename="basename.ext"` |
| `Content-Length` | Byte length of the output |
| `X-Request-ID` | Supplied or generated request label |
| `X-Conversion-Kind` | `lossless`, `safe_coercion`, or `lossy` |

Uploads keep the filename base. Raw requests use `converted.ext`. The conversion-kind header reflects the schema projection report; it does not certify compatibility or full value preservation for every format. Inspect output when changing schemas or using limited formats. See [response writing](./cmd/server/main.go) and [conversion reporting](./internal/uir/compat.go).

## 2. Custom schema ingestion

### Register a schema

`POST /system/schema` takes multipart `file` and optional `name` (default `default`). The filename selects the parser. Supported schema inputs are `.proto`, `.capnp`, `.graphql`, and the supported JSON-shaped `.json` or `.avro` inputs. This endpoint registers schemas; it does not upload arbitrary binary data files. See [parseSchema](./cmd/server/main.go).

Save this as `user.proto`:

```proto
syntax = "proto3";
message User {
  int32 id = 1;
  string name = 2;
  bool active = 3;
}
```

Register it and convert JSON using its `User` type:

```bash
curl --fail-with-body http://localhost:8080/system/schema \
  -F "name=user" -F "file=@user.proto"

curl --fail-with-body -OJ \
  "http://localhost:8080/morph/json/protobuf?schema=user&type=User" \
  -F "file=@data.json"
```

Registration returns `200` with a JSON object containing `status: "registered"`, `name`, `version`, and `format: "protobuf"`. The version is a full 64-character SHA-256 hash [identifier calculated from the parsed schema]. Copy the actual returned value for lifecycle operations; no fixed example version will match your schema.

Registration makes the version active. Re-registering the current parsed schema returns its existing metadata. Stored versions include original schema bytes and metadata. Persistence failures return `500`; unsupported or invalid schema input returns `422`. See the [registry](./internal/registry/registry.go).

### List schema versions

`GET /system/schema?name=user` returns `200` and the named schema's metadata array. Entries include `tenant`, `name`, `version`, `format`, `raw_content`, `timestamp`, and optional `active` and `deprecated`. Original bytes in `raw_content` appear as base64. An unknown name may return JSON `null`, not `404`. This endpoint does not list every name.

```bash
curl --fail-with-body "http://localhost:8080/system/schema?name=user"
```

See [metadata and listing](./internal/registry/registry.go).

### Activate a version

`POST /system/schema/activate?name=user&version=HASH` requires the full name and version. Replace `HASH` with a returned version:

```bash
curl --fail-with-body -X POST \
  "http://localhost:8080/system/schema/activate?name=user&version=HASH"
```

Success returns `200` with `{"status":"activated","name":"user","version":"HASH"}`. The selected version becomes active and is no longer deprecated. A registry error returns `400`. See [activation](./internal/registry/registry.go).

### Deprecate a version

`POST /system/schema/deprecate?name=user&version=HASH` marks that version deprecated [retained but marked for retirement]:

```bash
curl --fail-with-body -X POST \
  "http://localhost:8080/system/schema/deprecate?name=user&version=HASH"
```

Success returns `200` with `{"status":"deprecated"}`. Deprecation does not delete the version or remove it from active lookup. A registry error returns `400`. There is no HTTP endpoint for deleting a schema version. See [deprecation](./internal/registry/registry.go).

### Compare versions

`GET /system/schema/diff?name=user&from=OLD_HASH&to=NEW_HASH` requires two stored versions:

```bash
curl --fail-with-body \
  "http://localhost:8080/system/schema/diff?name=user&from=OLD_HASH&to=NEW_HASH"
```

Success returns a JSON object with `diff` containing `added`, `removed`, and `changed` field paths, plus `compatibility` set to `backward`, `forward`, `full`, or `breaking`. Missing versions return `404`. Lifecycle and diff handlers currently do not explicitly set a JSON content type; parse their body as JSON. See [schema diff](./internal/registry/registry.go) and [compatibility rules](./internal/uir/schema.go).

## 3. Real-time GraphQL subscriptions (WebSockets)

### Subscription requests

`GET /graphql/subscriptions` upgrades to WebSocket [connection kept open for live messages]. Use a WebSocket client, not an ordinary curl download. The server uses RFC 6455 frames [individual messages on the connection]. See [connection handling](./internal/network/websocket.go) and the [example client](./examples/websocket-client/main.go).

Query parameters:

| Parameter | Meaning |
| :--- | :--- |
| `schema` | Registered schema name; defaults to `default` |
| `target` | Output format; defaults to `graphql` |
| `source` | Optional override for the event's source format |
| `sourceType` / `targetType` | Named types for schema-bound binary data |
| `batchSize` | Container events per output; defaults to 16 for Avro, Parquet, HDF5 and 1 otherwise |

Use the exact advertised format identifiers here. An explicitly requested missing schema is reported after connection upgrade; clients must handle connection closure as well as HTTP errors. Use `batchSize=1` when each container event needs immediate output. There is no timer-based flush of a partial batch. See [subscription handling](./cmd/server/main.go) and [batching](./internal/stream/broker.go).

Send these JSON text messages in order:

```json
{"type":"connection_init"}
```

The server replies `{"type":"connection_ack"}`. Then send:

```json
{"id":"sub_1","type":"subscribe","payload":{"query":"subscription { transactionUpdated { id name } }"}}
```

The query must be a subscription. Use `payload.operationName` when selecting among named operations. Supported selections include aliases and fragments; a registered schema lets the server validate selected fields. `start` is accepted as an alias for `subscribe`. Reusing an ID replaces that subscription. See [subscription parsing](./internal/stream/graphql.go).

End a subscription with `{"id":"sub_1","type":"complete"}`; `stop` is also accepted. The socket remains open. Invalid subscriptions produce an `error` message with the subscription ID and `payload.message`.

### Event output

GraphQL emits text messages with selected data:

```json
{"id":"sub_1","type":"next","payload":{"data":{"transactionUpdated":{"id":42,"name":"Ada"}}},"extensions":{"eventId":"evt-1","cursor":"evt-1","schemaVersion":"unknown"}}
```

JSON and OData also emit `next` text envelopes, with data under the event type name. Binary targets emit binary messages, not base64 JSON. The layout is:

```text
bytes 0..3    ASCII "OMNI"
byte 4        envelope version 1
bytes 5..6    JSON header length, unsigned 16-bit big-endian
next bytes    JSON header
rest          raw target-format bytes
```

The header contains `eventId`, `cursor`, `eventType`, `format`, `schema`, and `schemaVersion`. Use [DecodeBinaryEnvelope](./internal/stream/broker.go) as the decoding reference.

Delivery is best effort, at most once [an event may be missed, never replayed]. Full queues drop older events. IDs are deduplicated within the subscription's bounded remembered set. There is no replay or resume; `cursor` is only a label. A subscription binds the active schema version when its connection is established. Deprecation keeps delivery active; a missing bound version closes the subscription. The server pings every 20 seconds. See [broker behavior](./internal/stream/broker.go), [subscription state](./internal/stream/subscription.go), and [connection deadlines](./cmd/server/main.go).

### Event injection

`POST /dev/events` is for local testing. It is disabled by the supplied Compose and Render configurations. Production requires `OMNI_DEV_EVENTS=1` and a valid token to enable it. Keep it disabled on public deployments. See [event handling](./cmd/server/main.go).

For local testing only, set `OMNI_DEV_EVENTS=1`, restart the server, and publish:

```bash
curl --fail-with-body http://localhost:8080/dev/events \
  -H "Content-Type: application/json" \
  -d '{"type":"transactionUpdated","data":{"id":42,"name":"Ada"},"format":"json","id":"sample-1"}'
```

The JSON envelope accepts `type`, `data`, optional `format` (default `json`), and optional `id`. Missing data becomes `{}`. For binary input, send raw bytes with `?source=cbor&type=transactionUpdated`. `X-Event-Format` is a fallback for `source`. Missing binary event type defaults to `event`.

Success returns `200` with body `{"status":"published"}`; the handler does not explicitly set JSON content type. It confirms publication, not receipt by every subscriber. Decode or encoding errors can drop individual subscriber delivery. Event bodies are limited to 5 MiB; raw publication is not a substitute for validating conversion output.

### Operations & telemetry

### Health

`GET /healthz` returns `200`, `application/json`, and `{"status":"ok","metrics":{...}}`. This checks that the server handles requests, not registry readiness.

### Readiness

`GET /readyz` returns `200`, `application/json`, and `{"status":"ready","registry":"<storage path>"}` when the registry is initialized. Otherwise it returns `503` with `registry not loaded`.

### Metrics

`GET /metrics` returns `200` and `application/json`. Fields include `http_requests_total`, `websocket_conns_active`, `schemas_registered_total`, `events_published_total`, `events_dropped_total`, and `conversion_failures_total`.

`latency_ms` has `parse`, `convert`, `encode`, and `stream`, each containing `count`, `p50`, `p95`, and `p99`. Percentiles [times below which a percentage of samples fall] use up to 4,096 retained timings. Counts are process-local and reset on restart. These are instrumented operations, not a complete access log; not every request or dropped event increments every named counter. See [metrics implementation](./internal/telemetry/telemetry.go) and [handler instrumentation](./cmd/server/main.go).

## 4. HTTP status codes & error handling

Most failures are plain text bodies from the server, not a standard JSON error object. A request ID is included by middleware [code run around each request]. See [handlers](./cmd/server/main.go).

| Status | Meaning |
| :--- | :--- |
| `200` | Successful conversion or operation |
| `400` | Invalid conversion, missing data/type, malformed upload, empty encoded result, or lifecycle error |
| `401` | Missing or incorrect token, or production without configured token |
| `403` | Event injection disabled |
| `404` | Named schema or compared version missing, or unknown route |
| `405` | Wrong method on handlers that enforce methods |
| `422` | Invalid or unsupported schema registration input |
| `429` | Client rate limit exhausted; follow `Retry-After` |
| `500` | Registry persistence or non-CBOR target encoding failure |
| `503` | Registry not initialized |

Unknown source and target identifiers return `400`. Encoding errors involving CBOR return `400`. Other unsupported target values may still produce `500`; consult format limits and inspect the response. Use `curl --fail-with-body` to fail on error responses while retaining their text.

## Configuration

| Variable | Behavior |
| :--- | :--- |
| `PORT` | Server port; default `8080` |
| `REGISTRY_PATH` | Schema registry file; default `registry_store.json` |
| `OMNI_ENV` | `production` requires a token and restricts event injection |
| `OMNI_API_TOKEN` | Required token on protected routes when configured |
| `OMNI_DEV_EVENTS` | `0` disables injection; `1` explicitly permits it in production |

See [server defaults](./cmd/server/main.go), [Docker Compose](./Docker/docker-compose.yml), and [Render configuration](./render.yaml). Compose stores the registry in a named volume. Render selects the free Singapore service, builds `main`, and disables automatic deployment. Its registry file is under `/tmp` with no persistent disk configured. Keep original schema files for re-registration.

## Performance measurements

The README badges report small-payload samples recorded in the [measurement notes](https://github.com/Roonil03/Omni-schema/blob/54b06cde4518cca5b0748dc34d153c70c62dcfcb/README.md). They are not production capacity guarantees or current hosted availability checks.

HTTP samples use `{"name":"Ada","id":42,"ok":true}`, a reused client, and 10 concurrent requests. Docker JSON to CBOR uses three warm-up requests and 100 measured requests. Hosted JSON to GraphQL uses one warm-up request and 30 measured requests. Times include the complete response body.

p50 means half of requests finished within that time. p95 and p99 cover 95 and 99 percent. The Docker values are 0.660 ms, 3.414 ms, and 6.840 ms. Hosted p50 and p95 are 268.182 ms and 778.669 ms. The success badges report 100/100 and 30/30.

The CBOR benchmark [timed repeated code execution] encodes and decodes the same object without HTTP or network time. It ran in Go 1.25 on Linux/amd64 in Docker on an Intel i9-11900H. Three runs measured 1,177, 1,342, and 1,259 ns/op; the badge uses the median, 1,259 ns/op. Each run used 1,504 bytes and 31 allocations. Reproduce with:

```bash
go test ./internal/codec -run '^$' -bench BenchmarkCBORRoundTrip -benchmem -count=3
```

See [benchmark code](./internal/codec/cbor_test.go), [HTTP route tests](./cmd/server/morph_matrix_test.go), and [contributor testing instructions](./CONTRIBUTING.md#run-checks).

[README](./README.md) · [Credits](./Credits.md) · [Contributing](./CONTRIBUTING.md)
