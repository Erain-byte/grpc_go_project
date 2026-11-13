ARG NODE_VERSION=22-alpine

FROM node:${NODE_VERSION} AS builder

ARG NPM_REGISTRY=https://registry.npmjs.org
ARG EXPO_PUBLIC_KEYCLOAK_ISSUER
ARG EXPO_PUBLIC_KEYCLOAK_CLIENT_ID=user-app
ARG EXPO_PUBLIC_GATEWAY_URL

ENV EXPO_NO_TELEMETRY=1 \
    EXPO_PUBLIC_KEYCLOAK_ISSUER=${EXPO_PUBLIC_KEYCLOAK_ISSUER} \
    EXPO_PUBLIC_KEYCLOAK_CLIENT_ID=${EXPO_PUBLIC_KEYCLOAK_CLIENT_ID} \
    EXPO_PUBLIC_GATEWAY_URL=${EXPO_PUBLIC_GATEWAY_URL}

WORKDIR /app

COPY frontend/user/package.json frontend/user/package-lock.json ./
RUN npm config set registry "${NPM_REGISTRY}" \
    && npm ci --no-audit --no-fund

COPY frontend/user/ ./
RUN npx expo export --platform web --output-dir dist --clear

FROM nginx:1.28-alpine

COPY deploy/production/nginx/user-web.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/dist/ /usr/share/nginx/html/

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=5 \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1
