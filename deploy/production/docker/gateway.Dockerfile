ARG GO_VERSION=1.26.2
FROM golang:${GO_VERSION}-alpine AS builder

RUN apk add --no-cache ca-certificates git
WORKDIR /src
COPY gateway/go.mod gateway/go.sum ./
RUN go mod download
COPY gateway/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/gateway ./cmd/gateway

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S -g 10001 app \
    && adduser -S -D -H -u 10001 -G app app \
    && mkdir -p /app/etc /app/logs \
    && chown -R app:app /app
WORKDIR /app
COPY --from=builder /out/gateway /app/gateway
USER app
EXPOSE 9080 9081
ENTRYPOINT ["/app/gateway"]
CMD ["-f", "/app/etc/gateway.yaml"]

