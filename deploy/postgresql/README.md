# 共享 PostgreSQL 部署说明

本目录管理项目唯一的 PostgreSQL 实例。实例沿用原 Keycloak PostgreSQL 数据卷，以保留现有 Realm、Client、用户和角色配置；其他服务在同一实例中使用独立数据库和独立账号。

## 数据库隔离

| 服务 | 数据库 | 账号 |
|---|---|---|
| Keycloak | `keycloak` | `keycloak` |
| Admin | `admin` | `admin_service` |
| User | `user_service` | `user_service` |
| LLM | `llm_service` | `llm_service` |
| Web3 | `web3_service` | `web3_service` |

所有容器在共享网络中统一使用 `postgres:5432`。

## 本地启动

### 从旧名称一次性切换

旧目录名为 `deploy/postgres-keycloak`，旧 Compose 服务名为 `keycloak-postgres`。首次使用新配置前，先通过 `docker ps -a` 找到并停止旧容器，然后只移除旧容器，不要删除 `identity_keycloak-postgres-data` 数据卷。

新配置会创建 `grpc-go-postgres` 容器并重新挂载同一个历史数据卷，因此 Keycloak 数据仍然存在。确认备份可恢复后再进行该操作。

首次准备网络和原有数据卷：

```powershell
docker network create grpc-go-identity
docker network create grpc-go-backend
docker volume create identity_keycloak-postgres-data
Copy-Item deploy/postgresql/.env.example deploy/postgresql/.env
```

填写 `.env` 后启动：

```powershell
docker compose --env-file deploy/postgresql/.env -f deploy/postgresql/compose.yaml up -d
```

现有 Keycloak 数据卷不会再次执行 PostgreSQL entrypoint 初始化，因此首次合并时需要手动创建其他数据库和账号：

```powershell
docker compose --env-file deploy/postgresql/.env -f deploy/postgresql/compose.yaml exec postgres /opt/grpc-go/create-service-databases.sh
```

该脚本是幂等的，不会删除或重建 `keycloak` 数据库。它会创建缺失的账号和数据库，并同步四个业务账号的密码。

本机运行的 Go 服务统一连接 `127.0.0.1:5432`。容器化服务统一连接 `postgres:5432`。

## 生产启动

```bash
docker network create grpc-go-identity-prod
docker network create grpc-go-backend-prod
docker compose --env-file deploy/postgresql/.env.prod -f deploy/postgresql/compose.prod.yaml up -d
docker compose --env-file deploy/postgresql/.env.prod -f deploy/postgresql/compose.prod.yaml exec postgres /opt/grpc-go/create-service-databases.sh
```

生产 Compose 不发布 PostgreSQL 宿主机端口，应用通过 `grpc-go-backend-prod` 内部网络访问。

## 旧业务数据库迁移

如果原 Admin、User、LLM 或 Web3 数据库已经存在业务数据，先执行 `pg_dump`，再恢复到本实例对应数据库。验证行数、索引、迁移版本和业务查询前，不得删除旧 Docker Volume。

旧的独立数据库配置目录已经移除，但这不会自动删除旧数据卷。数据卷必须在备份和恢复验证完成后单独人工清理。

## 安全与备份

- 每个服务只持有自己的数据库账号；
- 禁止把 `keycloak` 超级用户密码提供给业务服务；
- PostgreSQL 不开放公网；
- 定期执行整实例物理备份与各数据库逻辑备份；
- 恢复演练必须覆盖 Keycloak 和四个业务数据库；
- `.env` 包含真实密码，不得提交 Git。

> `identity_keycloak-postgres-data` 是已有本地数据卷的历史物理名称。为了避免数据丢失暂时保留；它不再表示 PostgreSQL 只供 Keycloak 使用。全新生产环境使用新的逻辑服务名 `postgres`，已经存在生产数据卷时也不得仅为改名而重建。
