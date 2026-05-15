# syntax=docker/dockerfile:1.7
#
FROM golang:1.26.3-alpine AS builder

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.Version=${VERSION}" \
    -o /out/mygrep ./cmd/mygrep

FROM alpine:3.20

RUN apk add --no-cache ca-certificates wget \
 && adduser -D -H -u 10001 mygrep

COPY --from=builder /out/mygrep /usr/local/bin/mygrep

USER mygrep:mygrep

EXPOSE 8081

HEALTHCHECK --interval=5s --timeout=2s --retries=5 \
    CMD wget -qO- http://localhost:8081/healthz || exit 1

ENTRYPOINT ["/usr/local/bin/mygrep"]
CMD ["--help"]
