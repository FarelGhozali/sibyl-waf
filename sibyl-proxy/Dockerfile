# ============================================================
# STAGE 1: BUILD — Compile Sibyl-Proxy binary
# ============================================================
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache ca-certificates

WORKDIR /build

# Cache dependencies layer terpisah.
COPY go.mod ./
# Proxy tidak punya go.sum (zero external deps), skip jika tidak ada.
RUN go mod download

# Copy source dan compile.
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /build/sibyl-proxy .

# ============================================================
# STAGE 2: RUNTIME — Scratch-level minimal
# ============================================================
FROM scratch

# Copy CA certs dari builder agar bisa HTTP ke sibyl-brain.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=builder /build/sibyl-proxy /sibyl-proxy

EXPOSE 4000

ENTRYPOINT ["/sibyl-proxy"]
