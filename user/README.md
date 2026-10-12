# User Service

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