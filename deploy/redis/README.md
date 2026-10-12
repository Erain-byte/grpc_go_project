# Redis 部署说明

Redis 已从 Identity Compose 中拆分出来，拥有独立的配置、生命周期和数据卷。Keycloak 与 PostgreSQL 仍由 `deploy/identity` 管理。

## 文件

| 文件 | 用途 |
|---|---|
| `compose.yaml` | 本地开发 Redis |
| `compose.prod.yaml` | 生产部署模板，不会自动生效 |
| `.env.example` | 本地环境变量示例 |
| `.env.prod.example` | 生产环境变量示例 |

真实 `.env` 和 `.env.prod` 已被仓库根目录的 `.gitignore` 忽略，不得提交密码。

## 本地启动

首次使用时复制示例并设置一个强密码：

```powershell
cd D:\grpc_go_project\deploy\redis
Copy-Item .env.example .env
```

当前项目迁移时已经从原 Identity `.env` 搬移了本地 Redis 密码，无需再次覆盖现有 `.env`。

启动与检查：

```powershell
docker compose up -d
docker compose ps
docker compose exec redis sh -c 'redis-cli --no-auth-warning -a "$REDIS_PASSWORD" ping'
```

正常结果为 `PONG`。查看日志和停止服务：

```powershell
docker compose logs redis
docker compose down
```

`docker compose down` 会保留数据卷。`docker compose down -v` 会删除 Redis 数据，执行前必须确认已有备份。

## 地址与密码

| 调用方位置 | 地址 |
|---|---|
| Windows 宿主机上的 Gateway/Admin/User | `127.0.0.1:6379` |
| 加入 `grpc-go-shared` 网络的本地容器 | `redis:6379` |
| 加入 `grpc-go-shared-prod` 网络的生产容器 | `redis:6379` |

Gateway、Admin、User 分别读取 `GATEWAY_REDIS_PASSWORD`、`ADMIN_REDIS_PASSWORD`、`USER_REDIS_PASSWORD`。这些值必须与本目录 `.env` 的 `REDIS_PASSWORD` 一致。

以后部署 OAuth2 Proxy 时，本地容器需加入外部网络 `grpc-go-shared`，生产容器需加入 `grpc-go-shared-prod`。配置中的 Redis 地址使用 `redis://redis:6379`，并同时提供密码。

## 持久化与隔离

Redis 启用了 AOF，使用 `appendfsync everysec`。本地数据卷固定为 `grpc-go-redis-data`，生产数据卷固定为 `grpc-go-redis-prod`，避免混用。

各模块共享一个 Redis 实例时必须使用独立 Key 前缀和 Cookie 名称。密码相同不代表业务数据可以混用。

端口只绑定 `127.0.0.1`。不要改成 `0.0.0.0:6379:6379`，也不要通过公网直接暴露 Redis。

## 生产启动

```powershell
cd D:\grpc_go_project\deploy\redis
Copy-Item .env.prod.example .env.prod
# 编辑 .env.prod，设置独立的生产强密码
docker compose --env-file .env.prod -f compose.prod.yaml config --quiet
docker compose --env-file .env.prod -f compose.prod.yaml up -d
```

生产环境优先从 Docker Secret、Vault 或云 Secret Manager 注入密码。上线前应建立数据卷备份、恢复验证和密码轮换流程。
