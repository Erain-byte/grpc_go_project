ARG GO_VERSION=1.26.2
FROM golang:${GO_VERSION}-alpine AS builder

RUN apk add --no-cache ca-certificates git
WORKDIR /src
COPY admin/go.mod admin/go.sum ./
RUN go mod download
COPY admin/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/admin ./cmd/admin \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/admin-migrate ./cmd/migrate \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/admin-seed ./cmd/seed-admin

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S -g 10001 app \
    && adduser -S -D -H -u 10001 -G app app \
    && mkdir -p /app/etc /app/logs \
    && chown -R app:app /app
WORKDIR /app
COPY --from=builder /out/admin /app/admin
COPY --from=builder /out/admin-migrate /app/admin-migrate
COPY --from=builder /out/admin-seed /app/admin-seed
USER app
EXPOSE 9082
ENTRYPOINT ["/app/admin"]
CMD ["-f", "/app/etc/admin.yaml"]

