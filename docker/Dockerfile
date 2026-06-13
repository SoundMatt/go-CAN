# Multi-stage build for go-CAN quickstart and cantool binaries.

FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /bin/quickstart ./examples/quickstart
RUN CGO_ENABLED=0 go build -trimpath -o /bin/cantool    ./cmd/cantool

# ── quickstart image ──────────────────────────────────────────────────────────
FROM alpine:3.21 AS quickstart
RUN apk add --no-cache ca-certificates
COPY --from=builder /bin/quickstart /usr/local/bin/quickstart
LABEL org.opencontainers.image.title="go-CAN quickstart" \
      org.opencontainers.image.source="https://github.com/SoundMatt/go-CAN" \
      org.opencontainers.image.licenses="MPL-2.0"
ENTRYPOINT ["quickstart"]

# ── cantool image ─────────────────────────────────────────────────────────────
FROM alpine:3.21 AS cantool
RUN apk add --no-cache ca-certificates
COPY --from=builder /bin/cantool /usr/local/bin/cantool
LABEL org.opencontainers.image.title="go-CAN cantool" \
      org.opencontainers.image.source="https://github.com/SoundMatt/go-CAN" \
      org.opencontainers.image.licenses="MPL-2.0"
ENTRYPOINT ["cantool"]
