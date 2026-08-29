# Stage 1: Build the from-scratch gateway
FROM golang:1.25.0-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o morph-gateway ./cmd/server

# Stage 2: Minimal production image
FROM scratch
WORKDIR /app
COPY --from=builder /app/morph-gateway .

USER 65532:65532
EXPOSE 8080
ENV PORT=8080
ENV REGISTRY_PATH=/tmp/registry_store.json
ENTRYPOINT ["./morph-gateway"]
