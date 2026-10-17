# LLM PostgreSQL

这个目录只管理 LLM 服务的 PostgreSQL 实例。它与 Keycloak、Admin、User 的数据库、网络和数据卷完全独立。

LLM 月份分区的建表、查询、自动建分区和归档操作见 [`docs/2026-10-17_POSTGRESQL_USER_LLM分表设计与操作指南.md`](../../docs/2026-10-17_POSTGRESQL_USER_LLM分表设计与操作指南.md)。

## 本地配置

| 配置 | 值 |
|---|---|
| 主机地址 | `127.0.0.1` |
| 主机端口 | `5435` |
| 容器地址 | `llm-postgres:5432` |
| 数据库 | `llm_service` |
| 用户 | `llm_service` |
| 密码环境变量 | `LLM_DB_PASSWORD` |

首次创建外部网络和数据卷：

```powershell
docker network create grpc-go-llm-database
docker volume create grpc-go-llm-postgres-data
```

复制环境变量模板，修改密码，然后启动：

```powershell
Copy-Item deploy\postgres-llm\.env.example deploy\postgres-llm\.env
docker compose --env-file deploy\postgres-llm\.env -f deploy\postgres-llm\compose.yaml up -d
```

LLM 进程使用相同密码：

```powershell
$env:LLM_DB_PASSWORD = "与 deploy/postgres-llm/.env 相同的密码"
```

Python LLM 服务接入数据库后对应参数为：

```yaml
database:
  driver: postgres
  host: 127.0.0.1
  port: 5435
  username: llm_service
  dbname: llm_service
  ssl_mode: disable
  timezone: Asia/Shanghai
  connect_timeout: 5s
```

## 分区设计

Docker Compose 只管理 PostgreSQL 实例。表结构和分区由 LLM 服务的版本化迁移程序负责。

建议保持单表的数据：

- `conversations`
- `prompt_templates`
- `model_configs`
- 初期的 `feedback`

建议按 `created_at` 使用 PostgreSQL 原生月份分区的数据：

- `conversation_messages`
- `llm_requests`
- `llm_usage`

物理分区使用 `YYYYMM` 命名，例如：

```text
conversation_messages_202610
conversation_messages_202611
```

应用始终对父表 `conversation_messages` 执行读写，由 PostgreSQL 路由到具体月份分区。查询消息时应同时包含 `conversation_id` 和时间范围，以便数据库执行分区裁剪。

迁移程序只需提前创建当前月份、下一个月份和一个默认分区。默认分区用于防止漏建月份分区时写入中断，不应长期积累数据。

## 生产模板

生产模板不发布 PostgreSQL 主机端口。LLM 应用容器加入 `grpc-go-llm-database-prod` 网络，并使用 `llm-postgres:5432` 访问数据库。

```powershell
docker network create grpc-go-llm-database-prod
docker volume create grpc-go-llm-postgres-data-prod
Copy-Item deploy\postgres-llm\.env.prod.example deploy\postgres-llm\.env.prod
docker compose --env-file deploy\postgres-llm\.env.prod -f deploy\postgres-llm\compose.prod.yaml up -d
```

生产环境根据数据库部署方式设置 `ssl_mode`。跨主机或经过非受控网络时应使用 `require`、`verify-ca` 或 `verify-full`。
