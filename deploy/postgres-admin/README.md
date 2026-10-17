# Admin PostgreSQL

这个目录只管理 Admin 服务的 PostgreSQL 实例。Keycloak 数据库由 `deploy/postgres-keycloak` 管理，User 数据库由 `deploy/postgres-user` 管理。

## 本地启动

先创建一次外部网络和数据卷：

```powershell
docker network create grpc-go-admin-database
docker volume create grpc-go-admin-postgres-data
```

复制环境变量模板并修改密码：

```powershell
Copy-Item deploy\postgres-admin\.env.example deploy\postgres-admin\.env
docker compose --env-file deploy\postgres-admin\.env -f deploy\postgres-admin\compose.yaml up -d
```

本地 PostgreSQL 只绑定 `127.0.0.1:5433`，Admin 配置对应：

```yaml
database:
  driver: postgres
  host: 127.0.0.1
  port: 5433
  username: admin_service
  dbname: admin
```

Admin 进程使用同一密码：

```powershell
$env:ADMIN_DB_PASSWORD = "与 deploy/postgres-admin/.env 相同的密码"
go run ./cmd/migrate -f etc/admin.yaml
go run ./cmd/admin -f etc/admin.yaml
```

## 生产模板

生产配置不发布 PostgreSQL 主机端口。Admin 容器应加入 `grpc-go-admin-database-prod` 网络，并使用 `admin-postgres:5432` 连接数据库。

```powershell
docker network create grpc-go-admin-database-prod
docker volume create grpc-go-admin-postgres-data-prod
Copy-Item deploy\postgres-admin\.env.prod.example deploy\postgres-admin\.env.prod
docker compose --env-file deploy\postgres-admin\.env.prod -f deploy\postgres-admin\compose.prod.yaml up -d
```

生产环境把 `ssl_mode` 设置为数据库实际要求的 `require`、`verify-ca` 或 `verify-full`。如果数据库和 Admin 仅通过受控 Docker 私有网络通信，可以根据部署策略决定是否临时使用 `disable`。

数据库表结构由 Admin 的迁移程序负责，Docker Compose 只负责 PostgreSQL 实例、网络和数据卷。
