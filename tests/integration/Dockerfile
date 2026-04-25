# ── Stage 1: extract the real gaiad binary from the official release image ────
FROM ghcr.io/cosmos/gaia:v27.2.0 AS gaiad-source
# `which gaiad` resolves the binary regardless of where the image places it,
# then copies it to a well-known path for the final stage.
RUN cp "$(which gaiad)" /tmp/gaiad-bin

# ── Stage 2: compile gentool from source ─────────────────────────────────────
FROM golang:1.26.2 AS builder
WORKDIR /src

# Download dependencies as a separate layer so they are cached across rebuilds.
COPY go.mod go.sum ./
RUN go mod download

# Copy the full source tree and build.
COPY . .
RUN CGO_ENABLED=0 go build -o /out/gentool ./cmd/gentool

# ── Stage 3: lean test runner ─────────────────────────────────────────────────
FROM debian:bookworm-slim AS runner

RUN apt-get update \
    && apt-get install -y --no-install-recommends jq \
    && rm -rf /var/lib/apt/lists/*

COPY --from=gaiad-source /tmp/gaiad-bin      /usr/local/bin/gaiad
COPY --from=builder      /out/gentool        /usr/local/bin/gentool
COPY tests/integration/scenario.sh          /scenario.sh
COPY tests/integration/validate_genesis.sh  /validate_genesis.sh

RUN chmod +x /scenario.sh /validate_genesis.sh /usr/local/bin/gaiad /usr/local/bin/gentool

ENTRYPOINT ["/scenario.sh"]