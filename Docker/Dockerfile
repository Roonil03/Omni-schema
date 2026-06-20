# Stage 1: Build the from-scratch gateway
FROM golang:1.25.0-alpine AS builder
WORKDIR /app

# Do not download third-party dependencies as per instructions
COPY . .

# Statically compile the binary with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o morph-gateway ./cmd/server

# Stage 2: Create a minimal scratch image
FROM scratch
WORKDIR /app
COPY --from=builder /app/morph-gateway .

# Expose the standard API port
EXPOSE 8080

# Run the binary
ENTRYPOINT ["./morph-gateway"]
