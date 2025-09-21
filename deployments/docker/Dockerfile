# Dockerfile
# ---------- Build stage ----------
FROM golang:1.23-bullseye AS builder
WORKDIR /app

# Cache modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# (opsional) sanity check: pastikan folder schemas ada di build context
# RUN test -d schemas/mcp

# Validasi semua schema JSON (gagal build bila ada yang invalid)
RUN apt-get update && apt-get install -y jq && rm -rf /var/lib/apt/lists/*
RUN for f in schemas/mcp/*.json; do echo "validating $f"; jq -e . "$f" >/dev/null || exit 1; done



# Build API binary (sekali saja) dengan cap versi
ARG BUILD_VERSION=dev
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
RUN go build -ldflags="-s -w -X mcp-oilgas/internal/config.BuildVersion=${BUILD_VERSION}" \
    -o /out/api ./cmd/api

# Siapkan schemas untuk dibawa ke runtime stage
RUN mkdir -p /out/schemas && cp -r schemas/* /out/schemas/

# ---------- Runtime stage ----------
FROM gcr.io/distroless/base-debian12 AS runtime
WORKDIR /app

# Bawa binary & schemas dari builder
COPY --from=builder /out/api /app/api
COPY --from=builder /out/schemas /app/schemas

# (opsional) mount log dir
VOLUME ["/app/logs"]

# Env untuk planner
ENV MCP_SCHEMAS_DIR=/app/schemas/mcp
ENV PORT=8080
EXPOSE 8080

USER 65532:65532
ENTRYPOINT ["/app/api"]
