# Omni-Schema Gateway

![License](https://img.shields.io/badge/License-MIT-yellow?style=for-the-badge)
![Go](https://img.shields.io/badge/Go-1.25-00ADD8?style=for-the-badge&logo=go&logoColor=white)
![Architecture](https://img.shields.io/badge/Architecture-Compiler_Model-blueviolet?style=for-the-badge)
![Dependencies](https://img.shields.io/badge/Dependencies-0-brightgreen?style=for-the-badge)
![Protocols](https://img.shields.io/badge/Protocols-10-blue?style=for-the-badge)
![Deploy](https://img.shields.io/badge/Deploy-Render-46E3B7?style=for-the-badge&logo=render&logoColor=white)

Omni-Schema Gateway is an advanced, high-performance API morphing service built entirely from scratch in Go. Operating on an Analysis-Synthesis compiler model, the gateway translates arbitrary payloads between highly complex binary and text protocols. It strictly adheres to a Zero-Dependency architecture, relying purely on the Go Standard Library for all operations.

---

## Features

The gateway acts as a universal translator. It parses incoming structures into a Universal Intermediate Representation (UIR) graph, enabling seamless, native morphing between disparate protocols.

Supported formats and protocols include:
- Standard Formats: [JSON](https://www.json.org/), [Protobuf](https://protobuf.dev/)
- Zero-Copy and Memory-Aligned: [Cap'n Proto](https://capnproto.org/)
- Schemaless Binary: [MessagePack](https://msgpack.org/)
- Columnar and Big Data: [Apache Parquet](https://parquet.apache.org/)
- Hierarchical Multidimensional: [HDF5](https://www.hdfgroup.org/solutions/hdf5/)
- Real-Time Streaming: Native [GraphQL](https://graphql.org/) Subscriptions running over custom RFC 6455 WebSockets

---

## Documentation

For instructions on operating the gateway, uploading schemas, and mapping structural conversions, please refer to the official documentation:

- [API Documentation](./API_DOCUMENTATION.md)

---

## Quick Start

### Running Natively
Because the project enforces a zero-dependency rule, it can be compiled and run directly with standard Go commands without the need for external package managers.

```bash
go run ./cmd/server
```

### Running with Docker
A highly optimized, multi-stage scratch build is provided for containerized environments. The service can be deployed seamlessly via Docker Compose:

```bash
cd Docker
docker-compose up --build -d
```

The server automatically binds to and listens on port 8080.

---

## Deploying to Render

This project includes a `render.yaml` blueprint for one-click deployment. Follow these steps:

### Step 1: Push the Repository to GitHub
Ensure all local commits are pushed to a GitHub repository (public or private).

```bash
git remote add origin https://github.com/<your-username>/Omni-schema.git
git push -u origin main
```

### Step 2: Connect Render to GitHub
1. Navigate to [render.com](https://render.com) and sign in (or create an account).
2. From the dashboard, click **New** and select **Blueprint**.
3. Connect your GitHub account if you have not already done so.
4. Select the **Omni-schema** repository.

### Step 3: Deploy via Blueprint
1. Render will automatically detect the `render.yaml` file in the repository root.
2. Review the proposed service configuration:
   - **Service Name**: `morph-gateway`
   - **Environment**: Docker
   - **Dockerfile Path**: `./Docker/Dockerfile`
   - **Plan**: Free
3. Click **Apply** to begin the build and deployment.

### Step 4: Verify the Deployment
Once the build completes, Render will assign a public URL (e.g., `https://morph-gateway.onrender.com`). Test it with:

```bash
curl -X POST https://morph-gateway.onrender.com/morph/json/protobuf \
  -H "Content-Type: application/json" \
  -d '{"id": 1, "name": "render-test"}'
```

> **Note**: On the free plan, services spin down after periods of inactivity. The first request after idle may take 30-60 seconds as the container cold-starts.

---

## Architecture Snapshot

- **Lexers and ASTs**: Constructed natively utilizing `text/scanner`.
- **Lowering Engine**: Maps complex schema abstractions to a universal `uir.TypeMap` and `uir.TypeArray`.
- **Codecs**: Generates heavily specified byte representations directly from the UIR memory pool.
- **WebSockets**: Implements TCP hijacking via `net/http` to securely facilitate GraphQL subscription channels.
