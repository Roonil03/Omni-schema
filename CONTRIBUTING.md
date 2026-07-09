# Contributing to Omni-Schema

Thank you for your interest in contributing to the **Omni-Schema Gateway**! We welcome bug reports, documentation improvements, architectural suggestions, and code contributions from the open-source community.

---

## 1. Core Architectural Constraints

When submitting code changes or proposing new features, please adhere to our strict architectural design principles:

- **Zero External Dependencies**: Omni-Schema is built entirely from scratch in Go (`Go 1.25+`). All lexers, ASTs, memory graphs, network routers, and codecs **must exclusively use the Go standard library** (`net/http`, `text/scanner`, `reflect`, etc.). Do not introduce third-party Go modules or external frameworks.
- **Analysis-Synthesis Model**: The gateway operates on a two-phase compiler design:
  1. **Analysis**: Parsing incoming payloads into a Universal Intermediate Representation (UIR) memory graph (`internal/uir`).
  2. **Synthesis**: Generating target protocol bytes directly from the UIR (`internal/codec`).
  New serialization formats must cleanly integrate into this UIR pipeline.

---

## 2. Local Development Setup

To set up your local development environment:

1. **Fork and Clone the Repository**:
   ```bash
   git clone https://github.com/your-username/Omni-schema.git
   cd Omni-schema
   ```

2. **Verify Go Version**:
   Ensure you have Go 1.25 or newer installed:
   ```bash
   go version
   ```

3. **Run the Server Locally**:
   Start the gateway server (defaults to port `8080`):
   ```bash
   # Linux / macOS / Git Bash
   PORT=8080 go run cmd/server/main.go

   # Windows PowerShell
   $env:PORT="8080"; go run cmd/server/main.go
   ```

---

## 3. Testing Your Contributions

Before committing or submitting a Pull Request, verify that your changes are fully tested and do not break existing functionality:

### Automated Unit Tests
Run the entire Go test suite across all internal and server packages:
```bash
go test ./... -v
```
All tests must pass cleanly (`PASS`).

### Manual Endpoint & cURL Verification
If you modify endpoints (`cmd/server`) or codecs (`internal/codec`), test your local server (`http://localhost:8080`) using standard cURL commands:
```bash
# Verify file upload and automatic local download (-O -J)
curl -O -J -X POST http://localhost:8080/morph/json/graphql \
  -F "file=@data.json"
```

---

## 4. Submitting a Pull Request (PR)

1. **Create a Feature Branch**:
   Create a clean, descriptive branch from `main`:
   ```bash
   git checkout -b feat/add-msgpack-support
   # or
   git checkout -b fix/header-filename-parsing
   ```

2. **Make Incremental, Clear Commits**:
   Write concise commit messages describing *what* changed and *why*:
   ```bash
   git commit -m "feat: implement zero-copy memory alignment for Cap'n Proto codec"
   ```

3. **Push and Open PR**:
   Push your branch to your fork and submit a Pull Request against the `main` branch of `Roonil03/Omni-schema`. Provide a clear summary of your changes and steps to verify them.

---

## 5. Documentation & Resources

For further architectural context when contributing, refer to:
- **[API Documentation](./API_DOCUMENTATION.md)**: Full reference for existing routes, supported formats, and error codes.
- **[Credits](./Credits.md)**: Team roles and acknowledgments.
