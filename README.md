# Omni-Schema

[![License](https://img.shields.io/badge/License-MIT-yellow?style=for-the-badge)](./LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?style=for-the-badge&logo=go&logoColor=white)](./go.mod)
[![Formats](https://img.shields.io/badge/Formats-10-blue?style=for-the-badge)](./internal/codec/formats.go)

Omni-Schema converts data between ten text and binary formats through an HTTP API [URLs you call to send or receive data]. Send a file or request body and download the converted output. It also stores schemas [field names and types] and sends live events over WebSockets [connections that stay open for live messages]. See the [API reference](./API_DOCUMENTATION.md).

The server is written in Go and uses only the standard library. See [go.mod](./go.mod) and the [server code](./cmd/server/main.go).

[API reference](./API_DOCUMENTATION.md) · [Examples](./examples/README.md) · [Contributing](./CONTRIBUTING.md) · [Credits](./Credits.md)

## Run locally

Clone the repository and start the server with Docker Compose:

```bash
git clone https://github.com/Roonil03/Omni-schema.git
cd Omni-schema
docker compose -f Docker/docker-compose.yml up --build
```

The supplied [Compose configuration](./Docker/docker-compose.yml) exposes `http://localhost:8080`, stores registered schemas in a persistent volume [storage kept when containers are replaced], and disables development event injection. Local conversion requests do not require an API token [secret used to authorize requests].

To run without Docker, install the Go version listed in [go.mod](./go.mod), then run:

```bash
go run ./cmd/server
```

The [server](./cmd/server/main.go) defaults to port `8080`. Set `PORT` to change it.

## Convert data

Save this as `data.json`:

```json
{"id":42,"name":"Ada","active":true}
```

Convert it to CBOR [compact binary data format], then convert the result back to JSON:

```bash
curl --fail-with-body http://localhost:8080/morph/json/cbor \
  -H "Content-Type: application/json" \
  --data-binary @data.json -o data.cbor

curl --fail-with-body http://localhost:8080/morph/cbor/json \
  -H "Content-Type: application/cbor" \
  --data-binary @data.cbor -o roundtrip.json
```

Use `--data-binary` when sending binary files. For file uploads, use `-F`; curl supplies the upload headers. `-OJ` saves the response with the filename provided by the server. These request forms are described in the [conversion reference](./API_DOCUMENTATION.md#1-payload--schema-morphing).

```bash
curl --fail-with-body -OJ http://localhost:8080/morph/json/msgpack \
  -F "file=@data.json"
```

Replace `json` and `msgpack` in `/morph/{source}/{target}` with any [format identifier](./API_DOCUMENTATION.md#supported-formats--aliases). The server also accepts form fields and query parameters. See the [examples](./examples/README.md#convert-a-payload).

The commands above use Bash line continuation. In Windows PowerShell, use `curl.exe` and put each command on one line.

## Formats and limits

The [format registry](./internal/codec/formats.go) lists ten source and target formats. The [HTTP conversion test](./cmd/server/morph_matrix_test.go) covers all 100 ordered pairs, including conversion to the same format. Format support has limits; passing a route test does not mean every possible value can be represented.

- JSON, [MessagePack](./internal/codec/msgpack.go), and [CBOR](./internal/codec/cbor.go) carry data values. CBOR supports objects with text keys, arrays, strings, bytes, 64-bit integers, floating-point numbers, booleans, and null. It rejects tags [extra type labels], containers without a declared size, duplicate keys, and trailing data.
- [Protobuf](./internal/codec/protobuf.go) and [Cap'n Proto](./internal/codec/capnproto.go) need a registered schema to preserve field names and types faithfully.
- [GraphQL](./API_DOCUMENTATION.md#complete-conversion-matrix-100-pairwise-routes) conversion produces SDL [text that defines GraphQL types]. It describes the data's structure; it does not return the original values. GraphQL subscriptions return selected event data.
- [Avro](./internal/codec/avro.go), [Parquet](./internal/codec/parquet.go), and [HDF5](./internal/codec/hdf5.go) implement limited file formats. CBOR conversion to these targets accepts flat records or arrays of records with matching fields and types. Parquet and HDF5 are not certified against external readers.
- [OData](./internal/codec/odata.go) supports JSON response envelopes [objects containing data and metadata]. It does not execute OData queries such as `$filter` or `$expand`.

Read the [format limits](./API_DOCUMENTATION.md#complete-conversion-matrix-100-pairwise-routes) before choosing a target. For example, JSON cannot represent NaN [a special value meaning not a number] or Infinity, and Protobuf cannot preserve explicit null values. CBOR conversions outside the supported target model return `400 Bad Request`. CBOR limits are 64 nesting levels and 100,000 items, including map keys. Conversion request bodies are limited to 10 MiB. See the [CBOR validation code](./internal/codec/cbor_target.go), [CBOR parser](./internal/codec/cbor.go), and [request handler](./cmd/server/main.go).

## Use a schema

Register a `.proto`, `.capnp`, or `.graphql` file, then select it when converting. The following commands use `user.proto` and the `User` type defined in the [Protobuf example](./examples/README.md#use-a-protobuf-schema):

```bash
curl --fail-with-body http://localhost:8080/system/schema \
  -F "name=user" -F "file=@user.proto"

curl --fail-with-body -OJ \
  "http://localhost:8080/morph/json/protobuf?schema=user&type=User" \
  -F "file=@data.json"
```

Use `sourceSchema` and `sourceType` for decoding, or `targetSchema` and `targetType` for output. The shared `schema` and `type` parameters apply to both. Fields outside the selected schema may be dropped, so choose the schema that matches your data. See [schema registration](./API_DOCUMENTATION.md#2-custom-schema-ingestion) and the [projection code](./internal/uir/project.go).

## Live events

Connect to `/graphql/subscriptions` to receive events in a selected format. The included [WebSocket client](./examples/websocket-client/main.go) and [event examples](./examples/README.md#subscribe-to-live-events) show how to subscribe and publish locally.

Delivery is best effort. Full queues drop older events, and reconnecting does not replay missed events. See the [event broker](./internal/stream/broker.go) and [subscription handler](./cmd/server/main.go).

## Deploy and operate

Set `OMNI_ENV=production` and `OMNI_API_TOKEN` for a public deployment. Protected requests accept `Authorization: Bearer <token>` or `X-API-Token`. Keep `/dev/events` disabled on public deployments. These checks are implemented in the [server](./cmd/server/main.go).

The [Render configuration](./render.yaml) uses the free Singapore plan, builds `main`, and disables automatic deployment and event injection. It stores schemas at `/tmp/registry_store.json`; keep schema files for re-registration because this deployment does not configure persistent storage. Use [Docker Compose](./Docker/docker-compose.yml) for the supplied persistent local setup.

For the [hosted endpoint](https://morph-gateway.onrender.com), obtain a token from the deployment owner and follow the [hosted request example](./examples/README.md#call-the-hosted-deployment). Confirm that the deployed version supports your chosen format.

The [server](./cmd/server/main.go) provides these monitoring endpoints:

- `GET /healthz` checks that the server is running.
- `GET /readyz` checks that the schema registry is initialized.
- `GET /metrics` returns request counts and timing measurements.

`REGISTRY_PATH` sets the schema storage file. Rate-limited requests return `429` with a `Retry-After` header. See the [API reference](./API_DOCUMENTATION.md#operations--telemetry) and [rate-limit tests](./cmd/server/rate_limit_test.go).

## Code and tests

Data is parsed into UIR [shared in-memory data representation], optionally mapped to a schema, and encoded in the target format. Follow the implementation through [parsers](./internal/lexer), [UIR and schema mapping](./internal/uir), [format encoders and decoders](./internal/codec), and the [HTTP server](./cmd/server/main.go).

Run tests and static checks [checks without running the server] from the repository root:

```bash
go test -race ./...
go vet ./...
```

With the local server running, test all HTTP conversion routes and measure CBOR encoding plus decoding:

```bash
OMNI_E2E=1 OMNI_E2E_URL=http://localhost:8080 \
  go test ./cmd/server -run TestComposeMorphMatrix -count=1

go test ./internal/codec -run '^$' -bench BenchmarkCBORRoundTrip -benchmem -count=3
```

For a protected server, set `OMNI_E2E_TOKEN` as well. See the [HTTP tests](./cmd/server/morph_matrix_test.go), [CBOR value tests](./internal/codec/cbor_variations_test.go), [request tests](./cmd/server/cbor_variations_test.go), and [benchmark](./internal/codec/cbor_test.go). Contribution requirements are in [CONTRIBUTING.md](./CONTRIBUTING.md).

## Credits and license

The project is maintained and developed by [Roonil03](https://github.com/Roonil03). [Ishaan Vatus](https://github.com/ishaanvatus) and [Lakshit Verma](https://github.com/vee1e) are credited as testers. The full team credits and acknowledgments remain in [Credits.md](./Credits.md). Omni-Schema is licensed under the [MIT license](./LICENSE).

## Performance

The HTTP measurements use `{"name":"Ada","id":42,"ok":true}` at 10 concurrent requests. Docker measures JSON to CBOR over 100 requests; the hosted sample measures JSON to GraphQL over 30 requests. Times include the response body. These small-payload samples do not establish production capacity. The [recorded measurements](https://github.com/Roonil03/Omni-schema/blob/54b06cde4518cca5b0748dc34d153c70c62dcfcb/README.md) include the setup and sample sizes.

p50 [half of requests finish within this time], p95 [95 percent finish within this time], and p99 [99 percent finish within this time] describe request latency [time until the response is fully received]. The CBOR round-trip benchmark measures encoding plus decoding without HTTP or network time. Its badge reports the median of three runs. See the [benchmark code](./internal/codec/cbor_test.go).

![Measurement date](https://img.shields.io/badge/Measured-2026--10--07-lightgrey)
[![Docker HTTP conversion routes](https://img.shields.io/badge/Docker_HTTP_routes-100%2F100_passed-brightgreen)](./cmd/server/morph_matrix_test.go)
![Docker CBOR p50 at concurrency 10](https://img.shields.io/badge/Docker_CBOR_p50_c10-0.660_ms-blue)
![Docker CBOR p95 at concurrency 10](https://img.shields.io/badge/Docker_CBOR_p95_c10-3.414_ms-blue)
![Docker CBOR p99 at concurrency 10](https://img.shields.io/badge/Docker_CBOR_p99_c10-6.840_ms-blue)
![Docker CBOR success](https://img.shields.io/badge/Docker_CBOR_success-100%2F100-brightgreen)
![Hosted GraphQL p50 at concurrency 10](https://img.shields.io/badge/Hosted_GraphQL_p50_c10-268.182_ms-blue)
![Hosted GraphQL p95 at concurrency 10](https://img.shields.io/badge/Hosted_GraphQL_p95_c10-778.669_ms-blue)
![Hosted GraphQL success](https://img.shields.io/badge/Hosted_GraphQL_success-30%2F30-brightgreen)
[![CBOR codec round trip](https://img.shields.io/badge/CBOR_codec_round_trip-1259_ns%2Fop-blue)](./internal/codec/cbor_test.go)
