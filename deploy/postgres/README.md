# PostgreSQL 部署说明

本目录独立管理 Keycloak 专用 PostgreSQL。Admin 和 User 的业务数据库不使用这个实例。

## 本地约定

| 项目 | 值 |
|---|---|
| 数据库 | `keycloak` |
| 用户 | `keycloak` |
| 容器地址 | `keycloak-postgres:5432` |
| Docker 网络 | `grpc-go-identity` |
| 数据卷 | `identity_keycloak-postgres-data` |

数据库没有映射宿主机端口，只有加入 `grpc-go-identity` 网络的 Keycloak 可以访问。

网络和数据卷声明为外部资源，避免其他 Compose 执行 `down -v` 时误删数据库。全新环境首次创建：

```powershell
docker network create grpc-go-identity
docker volume create identity_keycloak-postgres-data
```

当前开发环境已经存在这些资源，不要删除重建。

## 启动顺序

```powershell
docker compose --env-file deploy/postgres/.env -f deploy/postgres/compose.yaml up -d
docker compose --env-file deploy/postgres/.env -f deploy/postgres/compose.yaml ps
docker compose --env-file deploy/identity/.env -f deploy/identity/compose.yaml up -d
```

`deploy/postgres/.env` 与 `deploy/identity/.env` 中的 `KEYCLOAK_DB_PASSWORD` 必须一致。拆分生命周期后，Keycloak 仍需数据库密码才能建立连接。

健康检查：

```powershell
docker compose --env-file deploy/postgres/.env -f deploy/postgres/compose.yaml exec keycloak-postgres pg_isready -U keycloak -d keycloak
```

## 生产模板

生产使用网络 `grpc-go-identity-prod` 和数据卷 `grpc-go-keycloak-postgres-prod`：

```powershell
docker network create grpc-go-identity-prod
docker compose --env-file deploy/postgres/.env.prod -f deploy/postgres/compose.prod.yaml up -d
docker compose --env-file deploy/identity/.env.prod -f deploy/identity/compose.prod.yaml up -d
```

生产前必须完成 `pg_dump` 备份、恢复验证和密码轮换。不得在没有可恢复备份时删除数据库卷。
