# User PostgreSQL

这个目录只管理 User 服务的 PostgreSQL 实例。它与 Keycloak、Admin 的数据库、网络和数据卷完全独立。

User 固定 8 表的建表、路由、身份索引和扩容操作见 [`docs/2026-10-17_POSTGRESQL_USER_LLM分表设计与操作指南.md`](../../docs/2026-10-17_POSTGRESQL_USER_LLM分表设计与操作指南.md)。

## 本地配置

| 配置 | 值 |
|---|---|
| 主机地址 | `127.0.0.1` |
| 主机端口 | `5434` |
| 容器地址 | `user-postgres:5432` |
| 数据库 | `user_service` |
| 用户 | `user_service` |
| 密码环境变量 | `USER_DB_PASSWORD` |

首次创建外部网络和数据卷：

```powershell
docker network create grpc-go-user-database
docker volume create grpc-go-user-postgres-data
```

复制环境变量模板，修改其中的密码，然后启动：

```powershell
Copy-Item deploy\postgres-user\.env.example deploy\postgres-user\.env
docker compose --env-file deploy\postgres-user\.env -f deploy\postgres-user\compose.yaml up -d
```

User 进程使用相同密码：

```powershell
$env:USER_DB_PASSWORD = "与 deploy/postgres-user/.env 相同的密码"
```

User 应用切换到 PostgreSQL 后对应配置为：

```yaml
database:
  driver: postgres
  host: 127.0.0.1
  port: 5434
  username: user_service
  dbname: user_service
  ssl_mode: disable
  timezone: Asia/Shanghai
  connect_timeout: 5s
```

## 用户分表边界

Docker Compose 只负责数据库实例，不负责创建业务表。计划中的 `users_00` 至 `users_07`、身份索引表和 Snowflake 路由规则应由 User 服务的版本化迁移程序管理。

固定 8 张逻辑用户表投入使用后，不应直接修改取模基数。后续分库优先按完整物理表迁移，继续保持相同的 8 个逻辑表编号。

## 生产模板

生产模板不发布 PostgreSQL 主机端口。User 应用容器加入 `grpc-go-user-database-prod` 网络，并使用 `user-postgres:5432` 访问数据库。

```powershell
docker network create grpc-go-user-database-prod
docker volume create grpc-go-user-postgres-data-prod
Copy-Item deploy\postgres-user\.env.prod.example deploy\postgres-user\.env.prod
docker compose --env-file deploy\postgres-user\.env.prod -f deploy\postgres-user\compose.prod.yaml up -d
```

生产环境根据数据库部署方式设置 `ssl_mode`。跨主机或经过非受控网络时应使用 `require`、`verify-ca` 或 `verify-full`。
