# Omni-Schema

[![License](https://img.shields.io/badge/License-MIT-yellow?style=for-the-badge)](./LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?style=for-the-badge&logo=go&logoColor=white)](./go.mod)
[![Formats](https://img.shields.io/badge/Formats-10-blue?style=for-the-badge)](./API_DOCUMENTATION.md#formats)

## Formats

Omni-Schema converts data and generates schema definitions [field names and types]. The [API reference](./API_DOCUMENTATION.md#formats) describes the supported parts of each format.

- Text data: [JSON](https://www.json.org/json-en.html).
- Binary data without a separate schema: [MessagePack](https://msgpack.org/) and [CBOR](https://www.rfc-editor.org/rfc/rfc8949.html).
- Binary data with a schema: [Protocol Buffers](https://protobuf.dev/) and [Cap'n Proto](https://capnproto.org/).
- Data files: [Apache Avro](https://avro.apache.org/), [Apache Parquet](https://parquet.apache.org/), and [HDF5](https://www.hdfgroup.org/solutions/hdf5/).
- API formats: [GraphQL](https://graphql.org/) and [OData](https://www.odata.org/).

## Use the Render API

Suppose you have a JSON record and need a GraphQL schema describing its fields. Save this as `data.json`:

```json
{"id":42,"name":"Ada","active":true}
```

Obtain an API token [secret used to authorize requests] from the deployment owner. Set `OMNI_API_TOKEN` in your shell, then upload the file:

```bash
curl --fail-with-body -OJ \
  https://morph-gateway.onrender.com/morph/json/graphql \
  -H "Authorization: Bearer $OMNI_API_TOKEN" \
  -F "file=@data.json"
```

The response saves as `data.graphql`. It contains SDL [text that defines GraphQL types], rather than the original data values. To convert data to another format, replace `graphql` with a [target identifier](./API_DOCUMENTATION.md#formats). Keep the JSON source unchanged for this example. See [conversion requests](./API_DOCUMENTATION.md#1-payload--schema-morphing) for binary files and schema selection.

The command uses Bash line continuation. In PowerShell, use `curl.exe`, put the command on one line, and replace `$OMNI_API_TOKEN` with `$env:OMNI_API_TOKEN`. Let curl set file-upload headers. Confirm that the deployed server supports your chosen format.

## Run locally

Clone the repository, then start the [Docker Compose service](./Docker/docker-compose.yml):

```bash
git clone https://github.com/Roonil03/Omni-schema.git
cd Omni-schema
docker compose -f Docker/docker-compose.yml up --build
```

For Go without Docker, use the version in [go.mod](./go.mod) and run:

```bash
go run ./cmd/server
```

Both use `http://localhost:8080` by default. Replace the Render URL in the example with this local URL and omit the authorization header for the supplied local setup. Compose keeps registered schemas in its named volume [storage kept when containers are replaced]. See [configuration](./API_DOCUMENTATION.md#configuration) for production settings.

## Documentation

- [Credits](./Credits.md) lists contributors and how to request credit.
- [API reference](./API_DOCUMENTATION.md) covers requests, responses, format limits, and live events.
- [Contributing](./CONTRIBUTING.md) explains code changes and tests.

## Statistics and performance

[![Measurement date](https://img.shields.io/badge/Measured-2026--10--07-lightgrey)](./API_DOCUMENTATION.md#performance-measurements)
[![Docker HTTP conversion routes](https://img.shields.io/badge/Docker_HTTP_routes-100%2F100_passed-brightgreen)](./API_DOCUMENTATION.md#performance-measurements)
[![Docker CBOR p50 at concurrency 10](https://img.shields.io/badge/Docker_CBOR_p50_c10-0.660_ms-blue)](./API_DOCUMENTATION.md#performance-measurements)
[![Docker CBOR p95 at concurrency 10](https://img.shields.io/badge/Docker_CBOR_p95_c10-3.414_ms-blue)](./API_DOCUMENTATION.md#performance-measurements)
[![Docker CBOR p99 at concurrency 10](https://img.shields.io/badge/Docker_CBOR_p99_c10-6.840_ms-blue)](./API_DOCUMENTATION.md#performance-measurements)
[![Docker CBOR success](https://img.shields.io/badge/Docker_CBOR_success-100%2F100-brightgreen)](./API_DOCUMENTATION.md#performance-measurements)
[![Hosted GraphQL p50 at concurrency 10](https://img.shields.io/badge/Hosted_GraphQL_p50_c10-268.182_ms-blue)](./API_DOCUMENTATION.md#performance-measurements)
[![Hosted GraphQL p95 at concurrency 10](https://img.shields.io/badge/Hosted_GraphQL_p95_c10-778.669_ms-blue)](./API_DOCUMENTATION.md#performance-measurements)
[![Hosted GraphQL success](https://img.shields.io/badge/Hosted_GraphQL_success-30%2F30-brightgreen)](./API_DOCUMENTATION.md#performance-measurements)
[![CBOR codec round trip](https://img.shields.io/badge/CBOR_codec_round_trip-1259_ns%2Fop-blue)](./API_DOCUMENTATION.md#performance-measurements)
