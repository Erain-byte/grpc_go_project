# Production Docker deployment

This directory builds and runs the current Go backend on one Linux server.
The host does not need Go; Docker uses `golang:1.26.2-alpine` during the build
and copies only the compiled binaries into the runtime images.

## What to upload

Prefer cloning the complete Git repository on the server. The Docker build
context needs `gateway/`, `admin/`, `user/`, `web3/`, and `deploy/`.

Do not upload secrets through Git. Upload these separately:

- `/opt/grpc-go/env/production.env`
- `/opt/grpc-go/secrets/mtls/`
- public HTTPS certificates, if Nginx does not obtain them with Certbot

The mTLS directory must contain:

```text
mtls/
├── ca/ca.crt
├── consul/client.crt
├── consul/client.key
├── gateway/service.crt
├── gateway/service.key
├── admin/service.crt
├── admin/service.key
├── user/service.crt
├── user/service.key
├── web3/server.crt
└── web3/server.key
```

Certificate SAN values must match the Consul service names:
`gateway-service`, `admin-service`, `user-service`, and `web3-service`.

## Prepare configuration

```bash
sudo mkdir -p /opt/grpc-go/env /opt/grpc-go/secrets/mtls
sudo cp deploy/production/.env.example /opt/grpc-go/env/production.env
sudo chmod 600 /opt/grpc-go/env/production.env
```

Edit every placeholder in `production.env`, especially domains, database
passwords, Keycloak client secrets, cookie secret, anti-replay secret, and the
absolute `MTLS_CERTS_DIR`.

## Build images

Run from the repository root:

```bash
docker compose \
  --env-file /opt/grpc-go/env/production.env \
  -f deploy/production/compose.yaml \
  build gateway admin user web3
```

## Start infrastructure and create databases

Back up the existing Keycloak PostgreSQL volume first. The Compose file keeps
the physical volume name `grpc-go-keycloak-postgres-prod` intentionally.

```bash
docker compose \
  --env-file /opt/grpc-go/env/production.env \
  -f deploy/production/compose.yaml \
  up -d postgres redis rabbitmq consul jaeger

docker compose \
  --env-file /opt/grpc-go/env/production.env \
  -f deploy/production/compose.yaml \
  run --rm postgres-init
```

`postgres-init` is idempotent. It creates or updates the roles and databases
for Admin, User, LLM, and Web3 without deleting the Keycloak database.

## Run database migrations

Review schema changes before production migration, and take a database backup.

```bash
docker compose --env-file /opt/grpc-go/env/production.env -f deploy/production/compose.yaml --profile tools run --rm admin-migrate
docker compose --env-file /opt/grpc-go/env/production.env -f deploy/production/compose.yaml --profile tools run --rm user-migrate
docker compose --env-file /opt/grpc-go/env/production.env -f deploy/production/compose.yaml --profile tools run --rm web3-migrate
```

## Start the application

```bash
docker compose \
  --env-file /opt/grpc-go/env/production.env \
  -f deploy/production/compose.yaml \
  up -d

docker compose \
  --env-file /opt/grpc-go/env/production.env \
  -f deploy/production/compose.yaml \
  ps
```

Only loopback ports are published for Nginx and operations:

- Gateway HTTP: `127.0.0.1:9080`
- Keycloak: `127.0.0.1:8180`
- Admin OAuth2 Proxy: `127.0.0.1:4181`
- Consul UI: `127.0.0.1:8500`
- Jaeger UI: `127.0.0.1:16686`
- RabbitMQ UI: `127.0.0.1:15672`

Admin, User, Web3, PostgreSQL, Redis, RabbitMQ AMQP, Consul DNS, and internal
gRPC ports are not published publicly.

## Keycloak production changes

The realm import is mainly for a new installation. Import does not overwrite
an existing realm. For an existing realm, update these entries in the Keycloak
console before switching DNS:

- frontend URL and issuer: `https://$KEYCLOAK_DOMAIN`
- `user-app` redirect URLs for the real user domain and native app scheme
- `admin-web` redirect URL: `https://$ADMIN_WEB_DOMAIN/oauth2/callback`
- `admin-web` post-logout URL: `https://$ADMIN_WEB_DOMAIN/*`
- Web Origins for the real frontend domains
- the client secrets must match `production.env`

The public token issuer uses HTTPS, while services fetch JWKS through the
internal URL `http://keycloak:8080`. This avoids sending internal verification
traffic through public DNS without changing the issuer check.

## Useful commands

```bash
docker compose --env-file /opt/grpc-go/env/production.env -f deploy/production/compose.yaml logs -f gateway user web3
docker compose --env-file /opt/grpc-go/env/production.env -f deploy/production/compose.yaml restart gateway
docker compose --env-file /opt/grpc-go/env/production.env -f deploy/production/compose.yaml config --quiet
```

Do not use `docker compose down -v` in production. The `-v` option deletes
named data volumes.

