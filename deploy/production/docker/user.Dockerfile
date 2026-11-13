ARG GO_VERSION=1.26.2
FROM golang:${GO_VERSION}-alpine AS builder

ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=${GOPROXY}

RUN apk add --no-cache ca-certificates git
WORKDIR /src
COPY user/go.mod user/go.sum ./
RUN go mod download
COPY user/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/user ./cmd/user \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/user-migrate ./cmd/migrate

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S -g 10001 app \
    && adduser -S -D -H -u 10001 -G app app \
    && mkdir -p /app/etc /app/logs \
    && chown -R app:app /app
WORKDIR /app
COPY --from=builder /out/user /app/user
COPY --from=builder /out/user-migrate /app/user-migrate
USER app
EXPOSE 9084
ENTRYPOINT ["/app/user"]
CMD ["-f", "/app/etc/user.yaml"]

