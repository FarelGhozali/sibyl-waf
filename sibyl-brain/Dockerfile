# ============================================================
# STAGE 1: BUILD — Compile Sibyl-Brain binary
# ============================================================
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache ca-certificates git

WORKDIR /build

# Cache dependencies layer terpisah dari kode sumber.
COPY go.mod go.sum ./
RUN go mod download

# Copy seluruh source code dan compile.
COPY . .

# CGO_ENABLED=0 → static binary murni, bisa jalan di scratch.
# -ldflags="-s -w" → strip debug symbols, memperkecil ukuran biner.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /build/sibyl-brain .

# ============================================================
# STAGE 2: RUNTIME — Image minimal untuk produksi
# ============================================================
FROM alpine:3.23

# ca-certificates diperlukan untuk TLS ke Gemini API.
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binary dari stage builder.
COPY --from=builder /build/sibyl-brain .

# Port default Cloud Run / VPS.
EXPOSE 8080

# Health check sederhana.
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:8080/ || exit 1

ENTRYPOINT ["/app/sibyl-brain"]
