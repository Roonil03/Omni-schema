# Omni-Schema examples

These examples use the local Docker Compose gateway, which listens on `http://localhost:8080` and does not require a token. Start it from the repository root:

```bash
docker compose -f Docker/docker-compose.yml up --build
```

## Convert a payload

Create `data.json`:

```json
{"id":42,"name":"Ada","active":true}
```

Convert it to any of the ten supported targets:

```bash
curl -OJ -X POST http://localhost:8080/morph/json/msgpack -F "file=@data.json"
curl -OJ -X POST http://localhost:8080/morph/json/protobuf -F "file=@data.json"
curl -OJ -X POST http://localhost:8080/morph/json/graphql -F "file=@data.json"
curl -OJ -X POST http://localhost:8080/morph/json/avro -F "file=@data.json"
curl -OJ -X POST http://localhost:8080/morph/json/odata -F "file=@data.json"
curl -OJ -X POST http://localhost:8080/morph/json/capnproto -F "file=@data.json"
curl -OJ -X POST http://localhost:8080/morph/json/parquet -F "file=@data.json"
curl -OJ -X POST http://localhost:8080/morph/json/hdf5 -F "file=@data.json"
curl -OJ -X POST http://localhost:8080/morph/json/cbor -F "file=@data.json"
curl -OJ -X POST http://localhost:8080/morph/json/json -F "file=@data.json"
```

Every one of these formats is also accepted as a source, giving 100 source-to-target routes. See the [complete conversion matrix](../API_DOCUMENTATION.md#complete-conversion-matrix-100-pairwise-routes) and format aliases in the API documentation.

Decode the CBOR file back into JSON with a different filename to preserve the original:

```bash
curl --fail-with-body -X POST http://localhost:8080/morph/cbor/json \
  -H "Content-Type: application/cbor" --data-binary @data.cbor -o roundtrip.json
```

Use `--data-binary`, rather than `-d @file`, to preserve all CBOR bytes. Raw requests, file uploads, query routing, form routing, and `.CBOR` extension inference are covered by regression tests. Large 64-bit JSON integers and nested empty arrays are preserved. Invalid CBOR and combinations outside the [target codec subsets](../API_DOCUMENTATION.md#complete-conversion-matrix-100-pairwise-routes) return 400; they should not be treated as successful downloads. JSON byte strings are represented as base64 text. HTTP GraphQL output is a generated schema, not a payload round trip.

## Use a Protobuf schema

Create `user.proto`:

```proto
syntax = "proto3";

message User {
  int32 id = 1;
  string name = 2;
  bool active = 3;
}
```

Register it, then select both the schema and message type while converting:

```bash
curl -X POST http://localhost:8080/system/schema \
  -F "name=user" \
  -F "file=@user.proto"

curl -OJ -X POST "http://localhost:8080/morph/json/protobuf?schema=user&type=User" \
  -F "file=@data.json"
```

Fields outside the selected schema are dropped when at least one field matches. If the projection cannot produce any protobuf bytes, the gateway returns `400 Bad Request` instead of an empty file.

## Subscribe to live events

Run the included client while the Compose service is up:

```bash
go run ./examples/websocket-client
```

Compose intentionally sets `OMNI_DEV_EVENTS=0`. For local event testing only, change it to `1`, restart Compose, and publish:

```bash
curl -X POST http://localhost:8080/dev/events \
  -H "Content-Type: application/json" \
  -d '{"type":"transactionUpdated","data":{"id":42,"name":"Ada"}}'
```

Do not enable the development event endpoint in a public deployment. The supplied Render configuration runs in production mode, requires an API token, and keeps event injection disabled.

## Call the hosted deployment

All protected Render endpoints require a token issued by the deployment owner:

```bash
curl -OJ -X POST https://morph-gateway.onrender.com/morph/json/graphql \
  -H "Authorization: Bearer $OMNI_API_TOKEN" \
  -F "file=@data.json"
```

On Windows PowerShell, use `curl.exe` for these cURL commands.
