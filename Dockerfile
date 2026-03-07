# ── builder ───────────────────────────────────────────────────────────────────
FROM golang:1.25-bookworm AS builder

ENV CGO_ENABLED=0

WORKDIR /app

COPY go.mod go.sum Makefile ./
RUN make deps-download

COPY . .
RUN make build && make link

# ── runtime ───────────────────────────────────────────────────────────────────
FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/*

ENV DEFAULT_APP="/app/bin/zaplab"

WORKDIR /app

COPY --from=builder /app/bin/zaplab /app/bin/zaplab
COPY pb_public /app/pb_public
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 8090

ENTRYPOINT ["/entrypoint.sh"]
CMD ["/app/bin/zaplab"]