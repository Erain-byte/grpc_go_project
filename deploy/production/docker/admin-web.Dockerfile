ARG NODE_VERSION=22-alpine

FROM node:${NODE_VERSION} AS builder

ARG NPM_REGISTRY=https://registry.npmjs.org
ARG VITE_ADMIN_BFF_URL

ENV VITE_ADMIN_BFF_URL=${VITE_ADMIN_BFF_URL}

WORKDIR /app

COPY frontend/admin/package.json frontend/admin/package-lock.json ./
RUN npm config set registry "${NPM_REGISTRY}" \
    && npm ci --no-audit --no-fund

COPY frontend/admin/ ./
RUN npm run build

FROM nginx:1.28-alpine

COPY deploy/production/nginx/admin-web.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/dist/ /usr/share/nginx/html/

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=5 \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1
