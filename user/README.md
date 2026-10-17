# User Service

## PostgreSQL 表结构迁移

User 使用独立 PostgreSQL，固定创建 `users_00` 至 `users_07` 八张用户表，并使用不分片的 `user_identity_index` 将 Keycloak `sub` 路由到具体用户表。User 数据库不保存 Keycloak 登录密码。

先启动 `deploy/postgres-user`，然后给迁移程序设置相同密码：

```powershell
cd D:\grpc_go_project
docker network create grpc-go-user-database
docker volume create grpc-go-user-postgres-data
Copy-Item deploy\postgres-user\.env.example deploy\postgres-user\.env
docker compose --env-file deploy\postgres-user\.env -f deploy\postgres-user\compose.yaml up -d

cd user
$env:USER_DB_PASSWORD = "与 deploy/postgres-user/.env 相同的密码"
go run ./cmd/migrate -f etc/user.yaml
```

成功输出：

```text
user database migration completed: 8 shard groups
```

迁移会创建：

```text
user_identity_index
user_groups
user_group_members
users_00
users_01
users_02
users_03
users_04
users_05
users_06
users_07
user_wallets_00 ... user_wallets_07
user_fund_transactions_00 ... user_fund_transactions_07
```

`user_groups` 和 `user_group_members` 是 User 服务的业务分组，不等同于 Keycloak Group。成员关系保存 `user_id` 和 `table_index`，读取成员后可以直接通过分表 Router 查询对应用户表。成员表不建立指向八张动态用户表的数据库外键。

资金余额表和流水表与用户使用相同的 `user_id % 8` 分片，保证单个用户的资料、钱包和资金流水落在相同编号的逻辑分片。金额统一使用货币最小单位的 `BIGINT`，例如人民币 10.25 元保存为 1025，禁止使用浮点数。余额变更必须同时写入不可变流水，并使用业务类型与业务编号实现幂等。

路由固定使用 `user_id % 8`。正式写入数据后不能修改表数量或路由算法。详细设计见 `docs/2026-10-17_POSTGRESQL_USER_LLM分表设计与操作指南.md`。

生产环境版本化 SQL 位于 `migrations/`。`cmd/migrate` 适合本地开发，生产变更优先执行经过评审的 `.up.sql`，回滚脚本仅用于确认可以删除数据的重建场景。

Go 用户服务，负责用户资料、账户凭证及后续注册登录能力。模型调用、对话、RAG、Tool Calling 和 MCP 由 Python llm 服务负责。

当前是工程骨架：已提供配置加载、Consul 和 Redis 组件，cmd/user/main.go 仍为空入口，尚未提供可运行的 gRPC 用户接口。

## 目录

- cmd/user/：服务启动入口。
- etc/user.yaml：本地静态配置模板。
- internal/app/、server/、svc/：依赖组装、服务生命周期和共享依赖容器。
- internal/config/：配置结构、加载与校验。
- internal/consul/、redis/：服务注册、发现、健康检查和缓存客户端。
- internal/handler/、service/、model/：用户接口、业务逻辑和内部模型。
- internal/repository/：用户资料和账户凭证持久化。
- internal/middleware/、tracer/：鉴权、日志、追踪及请求治理。
- pkg/apperror/：错误类型与错误映射。

## 配置

在 user 目录调用 config.InitConfig("") 默认加载 etc/user.yaml；也可以显式传入配置路径。默认服务名为 user-service，gRPC 端口为 9083。

环境变量使用 USER_ 前缀：USER_ENVIRONMENT、USER_DB_PASSWORD、USER_REDIS_PASSWORD、USER_RABBITMQ_PASSWORD、USER_CONSUL_TOKEN。密钥不得写入 YAML 或提交到 Git。

限流规则预留 login、register 和 default，尚未实现限流中间件。用户服务不配置模型提供商或模型 token 配额。

## 后续开发

先定义独立的用户服务 Proto 契约，再实现 gRPC Server、用户存储和注册登录。Gateway 负责入口令牌校验，用户服务负责账户及令牌生命周期；对话和消息通过 user_id 关联用户。

proto/llm/v1 继续供 Python LLM 服务及 Gateway 使用，不作为用户服务接口。
