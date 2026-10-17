# Identity 本地与生产配置

本目录只管理 Keycloak。Keycloak 专用 PostgreSQL 已移到 `deploy/postgres-keycloak`，其他基础组件也分别拥有独立目录。

## 文件说明

| 文件 | 用途 |
|---|---|
| `compose.yaml` | 本地 Keycloak 开发环境，连接独立 PostgreSQL |
| `compose.prod.yaml` | 生产模板，不会自动生效 |
| `.env.example` | 本地变量示例 |
| `.env.prod.example` | 生产变量示例 |
| `realm/grpc-project-realm.template.json` | 已脱敏、可提交的 Realm 自动导入模板 |

真实 `.env`、`.env.prod` 与原始 Realm 导出文件均被 Git 忽略，禁止提交密码和 Client Secret。

## 本地启动

```powershell
cd D:\grpc_go_project\deploy\identity
Copy-Item .env.example .env  # 仅首次使用
docker compose --env-file ..\postgres-keycloak\.env -f ..\postgres-keycloak\compose.yaml up -d
docker compose config --quiet
docker compose up -d
docker compose ps
```

访问地址：

- Keycloak 管理台：`http://localhost:8180/admin/`
- Realm Discovery：`http://localhost:8180/realms/grpc-project/.well-known/openid-configuration`
- Readiness：`http://localhost:9000/health/ready`
- Metrics：`http://localhost:9000/metrics`

本地端口只绑定 `127.0.0.1`。`start-dev` 只适合本地开发，它允许 HTTP 并放宽部分生产检查。

## 本地环境变量

| 变量 | 用途 |
|---|---|
| `KEYCLOAK_DB_PASSWORD` | Keycloak PostgreSQL 密码 |
| `KEYCLOAK_ADMIN` | 首次启动的临时管理员名 |
| `KEYCLOAK_ADMIN_PASSWORD` | 临时管理员密码 |
| `USER_WEB_CLIENT_SECRET` | `user-web` Client Secret |
| `ADMIN_WEB_CLIENT_SECRET` | `admin-web` Client Secret |

已经存在的 `.env` 不要被示例文件覆盖。截图或聊天中出现过的 Secret 必须在生产前轮换。

## Realm 自动导入

本地与生产 Compose 都挂载 `realm/grpc-project-realm.template.json`，并使用 `--import-realm`。数据库中不存在 `grpc-project` 时会导入；Realm 已存在时不会覆盖。因此修改模板后若要更新现有 Realm，应通过管理台或 Admin CLI 修改，或在确认可丢弃本地数据后重建数据库。

模板应包含：

- Realm：`grpc-project`
- Realm Roles：`user`、`admin`、`super-admin`
- Clients：`grpc-api`、`user-web`、`admin-web`
- Client Scope：`grpc-api-audience`
- Audience Mapper：Access Token 的 `aud` 包含 `grpc-api`

原始导出可能包含用户、Client Secret 与环境地址，只能作为本地中间文件。提交前必须脱敏并保存为 `.template.json`。

## Web Client 本地约定

`user-web`：

- 回调：`http://localhost:4180/oauth2/callback`
- 登出回调：`http://localhost:4180/*`
- Standard Flow：开启
- Direct Access Grants：关闭

`admin-web`：

- 回调：`http://localhost:4181/oauth2/callback`
- 登出回调：`http://localhost:4181/*`
- Standard Flow：开启
- Direct Access Grants：关闭

两个 Client 都将 `grpc-api-audience` 设为 Default Client Scope。验证 Access Token 时必须看到 `iss=http://localhost:8180/realms/grpc-project`，且 `aud` 包含 `grpc-api`。

## 生产配置

生产模板使用 `start`，要求正式 Hostname，并假设同机 Nginx/Traefik 终止 HTTPS 后把请求转发到回环端口：

```powershell
cd D:\grpc_go_project\deploy\identity
Copy-Item .env.prod.example .env.prod
# 编辑 .env.prod
docker compose --env-file .env.prod -f compose.prod.yaml config --quiet
docker compose --env-file .env.prod -f compose.prod.yaml up -d
```

`127.0.0.1:8180:8080` 和 `127.0.0.1:9000:9000` 是端口绑定，不是生产域名。正式域名通过 `KEYCLOAK_HOSTNAME` 和反向代理配置；`9000` 不应暴露公网。

生产部署必须修改：

1. `KEYCLOAK_HOSTNAME` 为正式认证域名，不带路径。
2. Keycloak Client 的 Redirect URI、Post Logout URI 为正式 HTTPS 地址。
3. OAuth2 Proxy 和 Gateway 的 Issuer 为 `https://<认证域名>/realms/grpc-project`。
4. 开发与生产使用不同数据库、Realm 数据和 Secret。
5. 创建并验证永久 `master` Realm 管理员，再停用 Bootstrap 临时管理员。
6. 建立 PostgreSQL 备份与恢复演练。

生产 PostgreSQL 的网络、数据卷、备份和启动方式见 `deploy/postgres-keycloak/README.md`。

## 独立基础设施

| 组件 | 文档 |
|---|---|
| PostgreSQL | `deploy/postgres-keycloak/README.md` |
| Redis | `deploy/redis/README.md` |
| Consul | `deploy/consul/README.md` |
| RabbitMQ | `deploy/rabbitmq/README.md` |
| Jaeger | `deploy/jaeger/README.md` |
| OAuth2 Proxy | `deploy/oauth2-proxy/README.md` |
