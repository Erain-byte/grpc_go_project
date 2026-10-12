# OAuth2 Proxy 本地配置

本目录运行两套 OAuth2 Proxy：

| 实例 | 端口 | Keycloak Client | 允许的 Realm Role | Cookie |
|---|---:|---|---|---|
| User | 4180 | `user-web` | `user` | `_grpc_user_session` |
| Admin | 4181 | `admin-web` | `admin` 或 `super-admin` | `_grpc_admin_session` |

使用固定镜像 `quay.io/oauth2-proxy/oauth2-proxy:v7.15.5`。两个实例使用不同 Client Secret、Cookie Secret、Cookie 名称与端口，Session 保存到独立部署的 Redis。

## 前置条件

1. `deploy/redis` 已启动并创建网络 `grpc-go-shared`。
2. `deploy/postgres` 已启动，并与 Keycloak 共用网络 `grpc-go-identity`。
3. `deploy/identity` 已启动。
4. Keycloak 已配置 `user-web`、`admin-web`、回调 URI、角色与 `grpc-api` Audience。
5. 本目录 `.env` 的 Client Secret 与 Keycloak Credentials 页面完全一致。

## 启动

```powershell
docker compose -f deploy/redis/compose.yaml up -d
docker compose --env-file deploy/postgres/.env -f deploy/postgres/compose.yaml up -d
docker compose -f deploy/identity/compose.yaml up -d
docker compose --env-file deploy/oauth2-proxy/.env -f deploy/oauth2-proxy/compose.yaml config --quiet
docker compose --env-file deploy/oauth2-proxy/.env -f deploy/oauth2-proxy/compose.yaml up -d
docker compose --env-file deploy/oauth2-proxy/.env -f deploy/oauth2-proxy/compose.yaml ps
```

当前 `.env` 由现有 Identity/Redis 本地变量迁移生成，两个 Cookie Secret 自动随机生成，真实值不会提交 Git。

## 浏览器验证

- User：打开 `http://localhost:4180/`
- Admin：打开 `http://localhost:4181/`

当前上游使用 `static://200`，用于先验证 Keycloak 登录、回调、角色限制、Cookie 和 Redis Session。登录成功后页面可能只有空白的 200 响应，这是预期现象。

验证项目：

1. `user-test` 能进入 4180，不能进入 4181。
2. `admin-test` 能进入 4181，不能进入 4180。
3. 浏览器存在两个不同 Cookie。
4. Redis `DBSIZE` 在登录后增加。
5. OAuth2 Proxy 日志没有 Client Secret、Issuer、Audience 或角色错误。

## 为什么关闭自动 Discovery

浏览器必须使用 `http://localhost:8180`，容器间通信必须使用 `http://keycloak:8080`。配置保留外部 Issuer，并分别指定：

- Login URL：浏览器可访问的 localhost 地址。
- Redeem URL：OAuth2 Proxy 容器访问 Keycloak 的内部地址。
- JWKS URL：OAuth2 Proxy 容器访问 Keycloak 的内部地址。

Issuer 校验没有关闭。生产环境使用统一 HTTPS 域名后，应恢复 OIDC Discovery，并删除这三项本地拆分地址配置。

## 接入 Gateway

OAuth2 Proxy 通过 `http://host.docker.internal:9080` 访问运行在 Windows 主机上的 Gateway。Gateway 必须监听 `0.0.0.0:9080`。OAuth2 Proxy 将访问令牌放入 `X-Forwarded-Access-Token`，Gateway 仍会独立验证 Token 的 RS256 签名、issuer、audience 和有效期。

## 生产修改

- 两套回调地址改为正式 HTTPS 域名。
- `COOKIE_SECURE` 改为 `true`，Cookie 名称使用 `__Host-` 前缀。
- Issuer 改为正式 Keycloak HTTPS 地址并启用 Discovery。
- 使用 Secret Manager 或 Docker Secret 注入所有 Secret。
- 仅信任明确配置的反向代理地址，并由反向代理清理转发头。
- Cookie 生命周期与 Keycloak Access/Refresh Token 生命周期对齐。

官方配置参考：[Keycloak OIDC](https://oauth2-proxy.github.io/oauth2-proxy/configuration/providers/keycloak_oidc/)、[Session Storage](https://oauth2-proxy.github.io/oauth2-proxy/configuration/session_storage/)。
