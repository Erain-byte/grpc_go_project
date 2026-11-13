ARG GO_VERSION=1.26.2
FROM golang:${GO_VERSION}-alpine AS builder

ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=${GOPROXY}

RUN apk add --no-cache ca-certificates git
WORKDIR /src
COPY web3/go.mod web3/go.sum ./
RUN go mod download
COPY web3/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/web3 ./cmd/web3 \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/web3-migrate ./cmd/migrate

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S -g 10001 app \
    && adduser -S -D -H -u 10001 -G app app \
    && mkdir -p /app/etc /app/logs \
    && chown -R app:app /app
WORKDIR /app
COPY --from=builder /out/web3 /app/web3
COPY --from=builder /out/web3-migrate /app/web3-migrate
USER app
EXPOSE 9086
ENTRYPOINT ["/app/web3"]
CMD ["-f", "/app/etc/web3.yaml"]

