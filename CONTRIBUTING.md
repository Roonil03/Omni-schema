# Contributing

Submit focused changes with tests that show the expected behavior. Read the [API reference](./API_DOCUMENTATION.md) before changing request handling or a format's supported values.

## Choose a change

Use [GitHub issues](https://github.com/Roonil03/Omni-schema/issues) to report a bug or discuss a feature. For a bug, include the source format, target format, request, expected result, actual result, and server version or commit. Attach a small input that reproduces it. Remove tokens and private data.

Code, tests, documentation, and reproducible bug reports are welcome. Credit requests follow [Credits.md](./Credits.md#request-credit).

## Set up

Install the Go version listed in [go.mod](./go.mod). Docker is needed only for container tests. Fork [the repository](https://github.com/Roonil03/Omni-schema) into your account, clone your fork, and create a branch from current `main`:

```bash
git clone https://github.com/YOUR-USERNAME/Omni-schema.git
cd Omni-schema
git remote add upstream https://github.com/Roonil03/Omni-schema.git
git fetch upstream
git switch -c fix/describe-the-change upstream/main
```

Start the server with `go run ./cmd/server`, or use:

```bash
docker compose -f Docker/docker-compose.yml up --build
```

The local URL is `http://localhost:8080`. See [configuration](./API_DOCUMENTATION.md#configuration) before changing authentication or storage.

## Keep the project constraints

The project uses only the Go standard library. Do not add third-party Go modules. New formats must use UIR [shared in-memory data representation] for decoding and encoding. See [go.mod](./go.mod), [format registration](./internal/codec/registry.go), and [conversion flow](./API_DOCUMENTATION.md#how-conversion-works).

Keep unrelated changes out of the pull request [proposed changes submitted for review]. Preserve existing user-facing behavior unless the change is needed and documented. Never commit API tokens, private payloads, generated output, or local completion notes.

## Write a regression test

A regression test [test that catches a previously observed bug] should fail before the fix and pass after it. Put it beside the affected code in a `*_test.go` file.

- For decoding or encoding, use [internal/codec](./internal/codec). Assert decoded values, types, and relevant bytes. A successful status or nonempty result alone is insufficient.
- For HTTP requests, use [cmd/server](./cmd/server). Use `httptest` [Go's tools for testing HTTP handlers] to check response status, headers, and decoded output.
- For schema parsing, use [internal/lexer](./internal/lexer). For field mapping, use [internal/uir](./internal/uir).
- For events or frames, use [internal/stream](./internal/stream) and [internal/network](./internal/network). Check selection, event format, and invalid inputs.

Here is a minimal codec [data encoder or decoder] test. Choose a test name that does not already exist, and change the input and assertions to cover your bug:

```go
package codec

import (
    "testing"

    "omni-schema/internal/lexer"
)

func TestCBORPreservesReportName(t *testing.T) {
    input, err := lexer.ParseJSON([]byte(`{"name":"Ada"}`))
    if err != nil {
        t.Fatal(err)
    }
    encoded, err := GenerateCBOR(input)
    if err != nil {
        t.Fatal(err)
    }
    decoded, err := ParseCBOR(encoded)
    if err != nil {
        t.Fatal(err)
    }
    name := decoded.ChildByKey("name")
    if name == nil || name.Value != "Ada" {
        t.Fatalf("name was not preserved: %#v", name)
    }
}
```

Run the named test while developing:

```bash
go test ./internal/codec -run '^TestCBORPreservesReportName$' -count=1
```

Cover normal input and the failure you fixed. For format changes, include empty values, Unicode, numeric limits, nested values, malformed data, and values the target cannot represent. Assert that unsupported values return an error rather than corrupt output. See the [CBOR cases](./internal/codec/cbor_variations_test.go), [HTTP cases](./cmd/server/cbor_variations_test.go), and [round-trip tests](./internal/codec/roundtrip_test.go).

For a new format, update [registration](./internal/codec/registry.go), [identifiers and aliases](./internal/codec/formats.go), [options](./internal/codec/options.go), and the server's output filename and content type. Add tests against every advertised source and target. Add streaming tests if the format supports subscriptions. Document format limits and external reader compatibility; do not claim full standard support from internal round trips.

## Run checks

Format changed Go files, then run all package tests, the race detector [check for unsafe concurrent memory access], and static checks [checks without running the server]:

```bash
gofmt -w path/to/changed.go path/to/changed_test.go
go test -count=1 ./...
go test -race ./...
go vet ./...
git diff --check
```

For an HTTP change, run the [Compose service](./Docker/docker-compose.yml) and its conversion test. The test makes 110 requests, so run it on a fresh isolated server or wait for earlier requests to leave the one-minute rate-limit window. See the [120-request limit](./API_DOCUMENTATION.md#authentication-and-client-identity).

```bash
OMNI_E2E=1 OMNI_E2E_URL=http://localhost:8080 \
  go test ./cmd/server -run TestComposeMorphMatrix -count=1
```

To run that test from a Go container against the local Compose server:

```bash
docker run --rm -v "${PWD}:/app" -w /app \
  -e OMNI_E2E=1 -e OMNI_E2E_URL=http://host.docker.internal:8080 \
  golang:1.25 go test ./cmd/server -run TestComposeMorphMatrix -count=1
```

The container URL is for Docker Desktop. On Linux, add `--add-host=host.docker.internal:host-gateway`. For authenticated servers, supply `OMNI_E2E_TOKEN`. Avoid running the full matrix against a public deployment without the owner's approval.

These examples use Bash. In PowerShell, set environment variables with `$env:OMNI_E2E="1"` and `$env:OMNI_E2E_URL="http://localhost:8080"`, then run the Go command. Use `curl.exe` for curl examples.

For parser changes, use the existing fuzz test [test with automatically varied inputs]:

```bash
go test ./internal/codec -run '^$' -fuzz FuzzCBOR -fuzztime=15s
```

See [the fuzz test](./internal/codec/cbor_test.go) before adding seeds or another target.

## Open a pull request

Commit related changes together, then push your branch:

```bash
git add path/to/changed-files
git commit -m "fix: describe the behavior corrected"
git push -u origin fix/describe-the-change
```

Open a pull request against `Roonil03/Omni-schema:main`. Describe the problem, expected behavior, changed behavior, tests run, and any limits. Link the issue if one exists. For documentation changes, check relative links, section links, and copyable commands.

Do not present unrun tests as passing. Ask for review and wait for the maintainer to merge. Add a credit request with your preferred name and profile link if you want to appear in [Credits.md](./Credits.md).
