# 🌐 Omni-Schema Gateway

![Go Version](https://img.shields.io/badge/Go-1.25.0-00ADD8?style=flat&logo=go)
![Zero Dependencies](https://img.shields.io/badge/Dependencies-Zero-brightgreen)
![Build](https://img.shields.io/badge/Build-Passing-success)
![Docker](https://img.shields.io/badge/Docker-Supported-2496ED?style=flat&logo=docker)

Omni-Schema Gateway is an advanced, high-performance API morphing service built entirely from scratch in Go. It operates on an **Analysis-Synthesis compiler model**, translating arbitrary payloads between highly complex binary and text protocols—all while adhering to a strict **Zero-Dependency Rule** (relying purely on the Go Standard Library).

---

## 🚀 Features

The gateway parses incoming structures down to a Universal Intermediate Representation (UIR) graph, allowing it to morph seamlessly between disparate protocols natively:

- **Standard Formats**: `JSON`, `Protobuf`
- **Zero-Copy & Memory-Aligned**: `Cap'n Proto`
- **Schemaless Binary**: `MessagePack`
- **Columnar & Big Data**: `Apache Parquet`
- **Hierarchical Multidimensional**: `HDF5`
- **Real-Time Streaming**: Native `GraphQL Subscriptions` running over custom RFC 6455 WebSockets.

---

## 📚 Documentation Links

To learn how to operate the gateway, upload schemas, and map structural conversions, please refer to our official API Documentation:

- 📖 **[Comprehensive API Documentation](./API_DOCUMENTATION.md)**

*(Note: Internal design blueprints and PDFs are omitted from version control for security and proprietary reasons, but are documented locally in `Design.md`)*

---

## 🛠️ Quick Start

### Running Natively
Because the project enforces the Zero-Dependency rule, you can compile and run it directly with standard Go commands:

```bash
go run ./cmd/server
```

### Running with Docker
A highly optimized, multi-stage `scratch` build is provided. You can boot it seamlessly via Docker Compose:

```bash
cd Docker
docker-compose up --build -d
```

The server will automatically start listening on `:8080`.

---

## 🧩 Architecture Snapshot

- **Lexers & ASTs**: Built natively utilizing `text/scanner`.
- **Lowering Engine**: Maps complex schema abstractions to a universal `uir.TypeMap` and `uir.TypeArray`.
- **Codecs**: Generates heavily specified byte representations (such as Parquet Column Chunks, Cap'n Proto 64-bit arena allocators, and HDF5 B-Trees) directly from the UIR memory pool.
- **WebSockets**: TCP hijacking using `net/http` to securely map GraphQL subscription channels.
