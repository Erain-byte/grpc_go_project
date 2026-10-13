# gRPC Go Project 交接文档

内部 gRPC 服务身份、mTLS、RPC 白名单及后续 Consul KV 热更新方案见
[`INTERNAL_GRPC_SECURITY.md`](INTERNAL_GRPC_SECURITY.md)。

更新时间：2026-10-07

## 1. 项目目标

项目采用 Go 微服务与 Python LLM 服务：

- `gateway`：HTTP/gRPC 网关、Consul 发现、限流和追踪。
- `admin`：管理端 gRPC 服务，连接 MySQL、Redis、RabbitMQ。
- `user`：用户服务骨架，待完成业务闭环。
- `llm`：Python gRPC LLM 服务骨架。
- `proto`：共享 Protobuf 契约。

目标认证链路：Web → OAuth2 Proxy → Keycloak → Gateway → Admin/User/LLM。Keycloak 负责认证和签发 RS256 Token；Gateway 验证签名、Issuer、Audience 和有效期；内部服务执行角色与业务授权。

## 2. 已确定范围

当前优先完成可运行闭环：Keycloak、OAuth2 Proxy、Gateway RS256、Admin/User 接入。暂不建设 Kubernetes、滚动更新、Redis Cluster、Keycloak Cluster、复杂配置中心和完整 RAG/Agent 平台。

## 3. 当前基础设施

每个组件独立 Compose 管理：

| 组件 | 目录 | 主要端口 |
|---|---|---|
| Keycloak | `deploy/identity` | 8180、9000 |
| Keycloak PostgreSQL | `deploy/postgres` | 仅 Docker 内部 5432 |
| Redis | `deploy/redis` | 6379 |
| Consul | `deploy/consul` | 8500、8600 |
| RabbitMQ | `deploy/rabbitmq` | 5672、15672 |
| Jaeger | `deploy/jaeger` | 4317、4318、16686 |

本地状态：Redis healthy 且认证 PING=PONG；Consul healthy；RabbitMQ healthy；Jaeger 已启动。Consul 与 RabbitMQ 继续复用原有外部数据卷。

常用启动命令：

```powershell
docker compose -f deploy/redis/compose.yaml up -d
docker compose --env-file deploy/postgres/.env -f deploy/postgres/compose.yaml up -d
docker compose -f deploy/consul/compose.yaml up -d
docker compose --env-file deploy/rabbitmq/.env -f deploy/rabbitmq/compose.yaml up -d
docker compose -f deploy/jaeger/compose.yaml up -d
docker compose -f deploy/identity/compose.yaml up -d
```

## 4. Keycloak 当前配置

- Keycloak 版本：26.7.3；独立数据库：PostgreSQL 17。
- Realm：`grpc-project`。
- Realm Roles：`user`、`admin`、`super-admin`。
- Clients：`grpc-api`、`user-web`、`admin-web`。
- Scope：`grpc-api-audience`，以 Default 绑定到两个 Web Client。
- Access Token 的 `aud` 已验证包含 `grpc-api`。
- 测试用户：`user-test` 具备 `user`；`admin-test` 具备 `admin`。
- 永久管理用户 `kc-admin` 已创建并验证。
- 脱敏模板：`deploy/identity/realm/grpc-project-realm.template.json`。

详细手工步骤见 `docs/2026-10-04_KEYCLOAK本地安装指南.md`。

## 5. Redis 约定

宿主机服务连接 `127.0.0.1:6379`。Docker 容器加入 `grpc-go-shared` 后连接 `redis:6379`。Gateway、Admin、User 的 Redis 密码变量名不同，但当前值必须与 `deploy/redis/.env` 的 `REDIS_PASSWORD` 一致。

共享实例中的模块必须使用独立 Key 前缀和 Cookie 名称。Redis 用于 Session、限流和短期状态，不代替 JWT 签名验证。

## 6. 尚未完成

1. OAuth2 Proxy 两套本地实例已部署：User 使用 4180，Admin 使用 4181；待在浏览器完成真实账号登录验收。
2. Gateway 从旧 HS256 迁移至 Keycloak RS256/JWKS。
3. 旧 `AuthService.ValidateSession`、Gateway Session 中间件和相关 proto 已移除。
4. 增加 Gateway User Forwarder 与 HTTP 路由。
5. Admin 停止自行签发旧 Access/Refresh Token，改用可信身份 metadata。
6. 完成 User 启动入口、数据库、Redis、Consul、Tracer 和基础资料接口。
7. 解决 User 与 LLM 都使用 gRPC 端口 9083 的冲突；建议 User 改为 9084。
8. 完成认证、角色、并发和故障恢复测试。

## 7. Gateway 迁移验收

Gateway 必须：

1. 从 Keycloak Discovery/JWKS 获取公钥并按 `kid` 验证 RS256。
2. 校验 `iss` 为当前环境的 `grpc-project` Issuer。
3. 校验 `aud` 包含 `grpc-api`。
4. 校验 `exp`、`nbf` 等标准 Claims。
5. 解析 `sub`、`preferred_username` 和 `realm_access.roles`。
6. 向内部服务转发可信身份 metadata。
7. 覆盖错误 Issuer、错误 Audience、过期、未知 `kid` 和角色缺失测试。

## 8. 数据所有权

- Keycloak PostgreSQL：只保存 Keycloak 身份数据。
- Admin MySQL：Admin 自己的业务数据。
- User MySQL：User 自己的业务资料，以 Keycloak `sub` 作为外部身份唯一键。
- Redis：短期状态与缓存。
- RabbitMQ：异步消息和操作日志。

Admin 不直接查询 User 数据库；跨服务查询通过 User gRPC API，并在 User 服务执行最终权限判断。

## 9. Secret 规则

不得提交 `.env`、`.env.prod`、Token、Cookie、数据库密码、Client Secret、Redis/RabbitMQ 密码、Consul Token 或 TLS 私钥。示例文件只放占位符。任何在截图或聊天中出现过的真实值都应视为暴露，生产前必须轮换。

旧变量 `GATEWAY_ACCESS_TOKEN_SECRET`、`ADMIN_ACCESS_TOKEN_SECRET` 属于 HS256 迁移链路，完成 Keycloak 接入后删除。

## 10. 生产要求

- Keycloak 使用 `start`、正式 HTTPS 域名和反向代理转发头。
- 开发与生产使用独立数据库、Realm 数据卷和 Secret。
- Redis、PostgreSQL、MySQL、RabbitMQ 建立备份与恢复演练。
- 固定 Consul 与 Jaeger 镜像版本；当前本地仍沿用 `latest`。
- 限制管理端口来源，Keycloak 管理员启用 MFA。
- 日志禁止记录 Token、Cookie、密码和 Client Secret。

## 11. 后续开发顺序

1. 使用 `user-test` 和 `admin-test` 完成两套 OAuth2 Proxy 浏览器登录及角色隔离验收。
2. 完成 Gateway RS256/JWKS 验证，并将 OAuth2 Proxy 的临时 `static://200` 上游切换到 Gateway。
3. 完成 Admin 新认证迁移。
4. 完成 User 基础闭环。
5. 再继续 LLM Provider、流式输出、会话、配额、RAG 与 Tool Calling。

修改前先执行 `git status --short`，保留用户现有未提交内容。不要把历史 IAM 探索文档当成最终架构依据。

