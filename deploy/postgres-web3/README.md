# Web3 PostgreSQL

该目录单独管理 Web3 服务的 PostgreSQL 实例，与 Admin、User、LLM 和 Keycloak 的数据库、网络及数据卷隔离。

Docker Compose 只负责 PostgreSQL 实例。钱包、链扫描游标、事件回执、调用预算和交易分表由 Web3 的迁移程序创建。

## 本地开发配置

| 配置 | 值 |
|---|---|
| 主机地址 | `127.0.0.1` |
| 主机端口 | `5436` |
| 容器地址 | `web3-postgres:5432` |
| 数据库 | `web3_service` |
| 用户 | `web3_service` |
| 密码环境变量 | `WEB3_DB_PASSWORD` |

首次运行时创建独立的 Docker 网络和数据卷：

```powershell
docker network create grpc-go-web3-database
docker volume create grpc-go-web3-postgres-data
```

复制环境变量模板，修改密码后启动数据库：

```powershell
Copy-Item deploy\postgres-web3\.env.example deploy\postgres-web3\.env
docker compose --env-file deploy\postgres-web3\.env -f deploy\postgres-web3\compose.yaml up -d
docker compose --env-file deploy\postgres-web3\.env -f deploy\postgres-web3\compose.yaml ps
```

启动迁移程序和 Web3 服务前，在当前 PowerShell 会话注入同一个密码：

```powershell
$env:WEB3_DB_PASSWORD = "与 deploy/postgres-web3/.env 相同的密码"
Push-Location web3
go run ./cmd/migrate -f etc/web3.yaml
go run ./cmd/web3 -f etc/web3.yaml
Pop-Location
```

`web3/etc/web3.yaml` 已对应以下连接参数：

```yaml
database:
  enabled: true
  driver: postgres
  host: 127.0.0.1
  port: 5436
  username: web3_service
  dbname: web3_service
  ssl_mode: disable
  timezone: Asia/Shanghai
```

如果 Web3 也运行在 Docker 中，应将应用容器加入 `grpc-go-web3-database` 网络，并把数据库地址改为 `web3-postgres:5432`，不能在容器内使用 `127.0.0.1:5436`。

## 生产模板

生产模板不发布 PostgreSQL 主机端口。Web3 应用容器加入 `grpc-go-web3-database-prod` 网络，通过 `web3-postgres:5432` 访问数据库。

```powershell
docker network create grpc-go-web3-database-prod
docker volume create grpc-go-web3-postgres-data-prod
Copy-Item deploy\postgres-web3\.env.prod.example deploy\postgres-web3\.env.prod
docker compose --env-file deploy\postgres-web3\.env.prod -f deploy\postgres-web3\compose.prod.yaml up -d
```

生产密码不要提交到 Git；应由部署环境、Docker Secret 或密钥管理系统注入。跨主机访问 PostgreSQL 时，应根据证书部署情况将 Web3 的 `ssl_mode` 设置为 `require`、`verify-ca` 或 `verify-full`。

## 常用检查命令

```powershell
docker exec grpc-go-web3-postgres pg_isready -U web3_service -d web3_service
docker logs grpc-go-web3-postgres
docker compose --env-file deploy\postgres-web3\.env -f deploy\postgres-web3\compose.yaml down
```

`down` 不会删除这里使用的外部数据卷。需要删除数据时必须另外处理 `grpc-go-web3-postgres-data`，执行前应确认不再需要其中的数据。
