# ── Stage 1: extract the real gaiad binary from the official release image ────
FROM ghcr.io/cosmos/gaia:v27.2.0 AS gaiad-source
RUN cp "$(which gaiad)" /tmp/gaiad-bin

# ── Stage 2: compile gentool from source ─────────────────────────────────────
FROM golang:1.26.2 AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/gentool ./cmd/gentool

# ── Stage 3: lean smoke test runner ──────────────────────────────────────────
FROM debian:bookworm-slim AS runner

RUN apt-get update \
    && apt-get install -y --no-install-recommends jq curl \
    && rm -rf /var/lib/apt/lists/*

COPY --from=gaiad-source /tmp/gaiad-bin  /usr/local/bin/gaiad
COPY --from=builder      /out/gentool   /usr/local/bin/gentool
COPY tests/smoke/smoke.sh               /smoke.sh

RUN chmod +x /smoke.sh /usr/local/bin/gaiad /usr/local/bin/gentool

ENTRYPOINT ["/smoke.sh"]