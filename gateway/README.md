# Gateway

## Gateway 在系统中的角色

Gateway 同时承担两种角色：

```text
外部客户端
    │ HTTP 请求
    ▼
Gateway HTTP Server（服务端）
    │ gRPC 请求
    ▼
Gateway gRPC Client（客户端）
    │
    ▼
Admin / User / LLM gRPC Server（服务端）
```

因此需要区分两个容易混淆的目录：

- `internal/server`：Gateway 的入站 Server，接收外部 HTTP 请求；如果注册了入站 gRPC 服务，也在这里启动。
- `internal/grpc`：Gateway 调用内部服务时使用的 gRPC 客户端，包括 `ClientManager`、连接缓存、TLS、Consul resolver 和负载均衡。

Proto 生成代码的判断方法：

```go
// 客户端：Gateway 使用 ClientConn 创建客户端并发起请求。
client := adminv1.NewAdminServiceClient(conn)
resp, err := client.Login(ctx, req)

// 服务端：Admin 注册接口实现并接收请求。
adminv1.RegisterAdminServiceServer(grpcServer, adminHandler)
```

简单记忆：

```text
NewXXXServiceClient      = 客户端
RegisterXXXServiceServer = 服务端
```

## 一次请求的完整流程

以登录请求为例：

```text
HTTP Client
→ Gin Router
→ Gateway HTTP Middleware
→ Gateway Handler（JSON 转为 protobuf 请求）
→ AdminServiceClient.Login
→ grpc.ClientConn
→ Consul resolver 提供健康实例地址
→ grpc-go round_robin 选择一个 READY 后端
→ Admin gRPC Server
→ OpenTelemetry StatsHandler
→ RequestID / Error / Auth Interceptor
→ Admin Handler
→ Service
→ Repository / Redis / MySQL
→ protobuf Response
→ Gateway 转换成 HTTP JSON Response
```

各层职责：

- Handler：处理协议转换、参数和响应，不承载复杂业务规则。
- Service：处理登录、Token、权限等业务流程。
- Repository：封装 MySQL、Redis 等数据访问。
- ClientManager：创建和缓存 `grpc.ClientConn`，不处理具体业务。
- Consul resolver：只负责发现实例并把地址列表交给 grpc-go。
- grpc-go：负责连接状态和 `round_robin` 负载均衡。

## ClientConn、Consul 与 round_robin

`grpc.ClientConn` 不是单条 TCP 连接。它代表一个逻辑目标，例如：

```text
consul:///admin-service-grpc
```

完整更新过程：

```text
Consul 中的实例发生变化
→ 自定义 resolver 查询健康实例
→ 生成 []resolver.Address
→ resolver.ClientConn.UpdateState(...)
→ grpc.ClientConn 更新后端连接（SubConn）
→ round_robin 从 READY 连接中轮流选择
```

负载均衡配置：

```go
grpc.WithDefaultServiceConfig(
    `{"loadBalancingConfig":[{"round_robin":{}}]}`,
)
```

该配置启用的是 grpc-go 的 `round_robin`，不是 Consul 的负载均衡。Consul负责提供地址，grpc-go 负责选择地址。

## HTTP Context 与 gRPC metadata

Gateway 验证外部 Access Token 后，需要把可信身份传给内部服务：

```text
HTTP Authorization Header
→ Gateway JWT Middleware
→ Gin Context 中的用户身份
→ gRPC outgoing metadata
→ Admin incoming metadata
→ Admin AuthInfo context
```

客户端写入 outgoing metadata：

```go
md := metadata.Pairs(
    "x-user-id", userID,
    "x-user-role", role,
    "x-session-id", sessionID,
    "x-token-id", tokenID,
)
grpcCtx := metadata.NewOutgoingContext(ctx, md)
```

内部服务读取 incoming metadata：

```go
md, ok := metadata.FromIncomingContext(ctx)
```

`metadata.MD` 的本质是：

```go
type MD map[string][]string
```

它与 `http.Header` 类似。客户端使用 `NewOutgoingContext`，服务端使用 `FromIncomingContext`；单元测试可以用 `NewIncomingContext` 模拟服务端收到的 metadata。

这些身份字段不能在开放网络中直接信任。Gateway 与内部服务之间应使用 mTLS 或可靠的网络隔离，保证 metadata 来自可信 Gateway。

## Proto Auth Option 与认证拦截器

业务方法通过 Proto 自定义 Option 声明是否公开以及允许访问的角色：

```proto
rpc Login(LoginRequest) returns (LoginResponse) {
  option (common.v1.auth) = { public: true };
}

rpc GetAdminInfo(GetAdminInfoRequest) returns (GetAdminInfoResponse) {
  option (common.v1.auth) = {
    roles: "admin"
    roles: "super_admin"
  };
}
```

认证拦截器的主流程：

```text
健康检查 → 明确放行
Proto public 方法 → 放行
其他方法 → 读取身份 metadata
→ 检查 Proto roles
→ AuthInfo 写入 context
→ 调用业务 Handler
```

拦截器通过 Protobuf 反射读取方法 Option：

```text
/admin.v1.AdminService/GetAdminInfo
→ admin.v1.AdminService.GetAdminInfo
→ protoregistry 查找 MethodDescriptor
→ 读取 common.v1.auth Option
```

`protoreflect.FullName` 只是 Protobuf 反射使用的完整名称类型。这里使用的是 Protobuf 反射，不是 Go 的 `reflect` 包。

没有声明 `auth` Option 的业务方法默认需要认证（fail closed），防止新增接口漏配权限后被匿名访问。

### 健康检查为什么单独放行

标准健康检查方法是：

```go
healthpb.Health_Check_FullMethodName
```

它的值为：

```text
/grpc.health.v1.Health/Check
```

该方法来自 grpc-go 官方 Health Proto，项目无法给它添加自定义 `auth` Option。Consul 需要调用它判断实例状态，因此认证拦截器使用官方常量明确放行，而不是手写路径字符串。

注册健康服务：

```go
healthServer := health.NewServer()
healthpb.RegisterHealthServer(grpcServer, healthServer)
```

注册表示把 Health 服务实现保存到 `grpc.Server` 的服务表中。收到对应 RPC 后，grpc-go 才能找到并执行 `Check` 方法。

## Context 和链路追踪

一次请求应继续使用传入的 `context.Context`，不要在调用链中随意改成 `context.Background()`，否则可能丢失：

- 请求取消信号
- Deadline
- Trace 信息
- gRPC metadata
- Request ID 和认证身份

OpenTelemetry 的客户端和服务端 StatsHandler 会负责在 gRPC metadata 中注入、提取 Trace Context：

```text
Gateway HTTP Span
→ Gateway gRPC Client Span
→ Admin gRPC Server Span
→ Admin 业务 Span
```

业务身份 metadata 和 Trace metadata 都通过 gRPC metadata 传播，但用途不同：前者用于认证授权，后者用于可观测性。

使用 Go 编写的 HTTP/gRPC 网关。Gateway 接收外部 HTTP 请求，通过共享的
`ClientManager` 调用内部 gRPC 服务，并通过 Consul resolver 获取健康实例，
由 grpc-go `round_robin` 完成客户端负载均衡。

## 请求链路

普通请求：

```text
HTTP JSON
→ Gin Router
→ HTTP Handler
→ gRPC Forwarder
→ ClientManager
→ Consul resolver
→ grpc.ClientConn / round_robin
→ 下游 gRPC 服务
```

LLM 流式请求：

```text
HTTP JSON
→ LlmHTTPHandler.StreamChat
→ gRPC Server Streaming
→ stream.Recv()
→ HTTP SSE
```

## 本地开发

```bash
go run ./cmd/gateway
go test ./...
go vet ./...
```

默认配置文件为 `etc/gateway.yaml`，也可以通过 `-f` 指定：

```bash
go run ./cmd/gateway -f etc/gateway.yaml
```

## 依赖注入与生命周期

`internal/app.Run` 是应用的组装入口，负责按顺序创建长生命周期依赖：

```text
Config
→ Logger / Redis
→ ConsulRegistry
→ ClientManager
→ HTTP Server
```

`ClientManager` 在应用启动时只创建一次，通过 `server.NewHTTPServer` 和
`server.NewGRPCServer` 注入两个入站 Server，再由 Server 注入 Handler 和 Forwarder。所有请求共享其中缓存的
`grpc.ClientConn`；应用停止后，由 `ClientManager.Close` 统一关闭连接。

HTTP 和 gRPC 在不同 goroutine 中并发运行，由 `app.Run` 统一监听
`SIGINT`/`SIGTERM` 和 Server 错误。退出时并发执行：

```text
HTTPServer.Shutdown(ctx)
GRPCServer.Shutdown(ctx)
→ ClientManager.Close()
→ ServiceContext.Close()
→ Logger.Sync()
```

gRPC 首先使用 `GracefulStop` 等待正在执行的 RPC 和 Stream；超过配置的
`shutdown.timeout` 后使用 `Stop` 强制结束。

## HTTP 路由

健康检查：

```text
GET /health
```

Admin：

```text
POST /admin/login
POST /admin/logout
POST /admin/info
POST /admin/create
POST /admin/list
```

LLM：

```text
POST /llm/chat
POST /llm/stream-chat
POST /llm/chat-history
POST /llm/chat-list
POST /llm/call-model
```

除 `/llm/stream-chat` 使用 SSE 外，其余当前路由均为普通 JSON 请求和响应。

## Proto 编辑器支持

使用 VS Code 打开本项目时，编辑器会推荐安装
`Protobuf Support (Protols Language Server)` 扩展。该扩展为 `.proto` 文件提供
语法高亮、代码补全、定义跳转和错误诊断。

也可以通过命令手动安装：

```bash
code --install-extension ianandhum.protobuf-support
```

安装完成后打开任意 `.proto` 文件即可启用。首次使用时，扩展可能会提示自动安装
`protols` 语言服务器，请按提示确认。

## 目录结构

```text
gateway/
├── cmd/gateway/
│   └── main.go               解析 -f 配置参数并调用 app.Run
├── etc/
│   └── gateway.yaml          本地运行配置示例
├── internal/
│   ├── app/
│   │   └── app.go            依赖初始化、健康检查、服务启动及优雅关闭
│   ├── config/
│   │   ├── config.go         配置结构、文件加载、环境变量覆盖
│   │   └── config_test.go    配置加载测试
│   ├── consul/
│   │   ├── consul.go         服务注册、注销、发现、缓存、Watch 和关闭
│   │   ├── grpc_discovery.go grpc-go resolver，向 ClientConn 发布地址
│   │   └── *_test.go         Consul 与 resolver 单元测试
│   ├── forwarder/
│   │   ├── base.go           泛型 Forwarder，统一获得下游 gRPC Client
│   │   ├── admin.go          Admin RPC 转发实现
│   │   ├── llm.go            LLM 普通请求与流式 RPC 转发实现
│   │   ├── register.go       将全部 gRPC Forwarder 注册到入站 Server
│   │   └── doc.go            forwarder 包说明
│   ├── grpc/
│   │   ├── client.go         ClientManager、连接缓存、TLS、证书监控和客户端工厂
│   │   └── client_test.go    连接复用、并发及证书检查测试
│   ├── handler/
│   │   ├── grpc_handler.go   HTTP→gRPC 泛型 Handler、SSE 和小文件上传适配
│   │   └── doc.go            handler 包说明
│   ├── logger/
│   │   ├── logger.go         Zap 初始化及带 Trace ID 的 Context Logger
│   │   └── logger_test.go    日志 Trace ID 测试
│   ├── middleware/
│   │   ├── cors.go           CORS 来源、方法及请求头控制
│   │   ├── error.go          Panic 恢复、统一错误响应及 gRPC→HTTP 转换
│   │   ├── grpc_tracing.go   gRPC 客户端和服务端 OpenTelemetry StatsHandler
│   │   ├── jwt.go            Access Token 解析、校验及 Gin 身份写入
│   │   ├── logger.go         HTTP 请求日志
│   │   ├── request_id.go     读取或生成 Request ID
│   │   ├── tracering.go      Gin OpenTelemetry 链路追踪中间件
│   │   └── *_test.go         错误、JWT、日志与 Request ID 测试
│   ├── redis/
│   │   ├── client.go         单机/集群 Redis 适配及常用操作接口
│   │   └── client_test.go    Redis 配置和持续时间解析测试
│   ├── repository/
│   │   └── doc.go            Gateway 数据访问层预留说明
│   ├── server/
│   │   ├── http.go           HTTPServer 创建、启动与关闭
│   │   ├── grpc.go           入站 GRPCServer 创建、启动与优雅关闭
│   │   ├── routes.go         路由注册总入口
│   │   ├── routes_health.go  健康检查路由
│   │   ├── routes_admin.go   Admin 路由
│   │   └── routes_llm.go     LLM 路由
│   ├── svc/
│   │   ├── servicecontext.go 共享 Config、Redis 和 Consul
│   │   └── servicecontext_test.go
│   └── tracer/
│       └── tracer.go         OTLP Exporter、TracerProvider、采样及关闭
├── pkg/
│   ├── apperror/
│   │   ├── error.go          应用错误模型和 HTTP/gRPC 状态转换
│   │   └── error_test.go
│   └── doc.go                pkg 包说明
├── go.mod                    Gateway Go Module 依赖声明
├── go.sum                    依赖校验和
├── Makefile                  常用构建、测试和运行命令
└── README.md                 Gateway 架构与开发说明
```

根仓库中的 `../proto` 是独立 Go Module，保存 Admin、LLM 的 protobuf 定义和生成代码。

### 关键脚本的创建与调用关系

| 脚本 | 创建/调用位置 | 主要输出或作用 |
| --- | --- | --- |
| `cmd/gateway/main.go` | 操作系统启动进程 | 调用 `app.Run`，并处理无法启动时的最终错误 |
| `internal/app/app.go` | `main.go` | 解析 `-f`、创建长生命周期依赖并管理退出顺序 |
| `internal/config/config.go` | `app.Run` | 返回 `*config.Config` |
| `internal/logger/logger.go` | `app.Run` | 初始化全局 Zap Logger |
| `internal/redis/client.go` | `app.Run` | 返回 `RedisClient`，关闭时释放连接池 |
| `internal/consul/consul.go` | `app.Run` | 返回 `*ConsulRegistry`，负责注册和发现 |
| `internal/tracer/tracer.go` | `app.Run` | 返回 Tracer Manager，关闭时 Flush Span |
| `internal/svc/servicecontext.go` | `app.Run` | 保存共享依赖，注入 Server、Handler 和 Forwarder |
| `internal/grpc/client.go` | `app.Run` | 返回共享 `*ClientManager`，按服务缓存 ClientConn |
| `internal/server/http.go` | `app.Run` | 创建 Gin Engine、注册中间件和 HTTP 路由 |
| `internal/server/grpc.go` | `app.Run` | 创建 Gateway 入站 gRPC Server 并注册 Forwarder |
| `internal/server/routes_*.go` | `NewHTTPServer` | 把业务 Handler 挂到具体 Gin 路径 |
| `internal/handler/grpc_handler.go` | `routes_*.go` | 绑定 HTTP 参数，调用 gRPC，写回 JSON/SSE |
| `internal/forwarder/*.go` | `server/grpc.go` | 接收入站 RPC，再调用对应下游 gRPC 服务 |
| `internal/consul/grpc_discovery.go` | `ClientManager.newClient` | 把 Consul 健康地址通过 `UpdateState` 交给 ClientConn |

### 应用启动顺序

```text
main.go
→ app.Run
→ InitConfig
→ InitializeLogger
→ NewRedisClient / Ping
→ NewConsulRegistry / Ping
→ NewTracerProvider
→ NewServiceContext
→ NewClientManager
→ NewHTTPServer / NewGRPCServer
→ Consul RegisterHTTP / RegisterGRPC
→ 启动证书监控
→ 并发启动 HTTP 与 gRPC Server
→ 等待系统信号或 Server 错误
→ Consul 注销
→ Server 优雅关闭
→ ClientManager、Consul、Redis、Tracer、Logger 释放资源
```

测试文件使用与实现文件相同的包目录存放，不再单独设置一个不存在的 `test/` 目录；未来需要跨模块集成测试时再新增专门目录。

### Makefile 命令

在 `gateway` 目录执行：

| 命令 | 实际执行 | 用途 |
| --- | --- | --- |
| `make run` | `go run ./cmd/gateway` | 使用默认配置启动 Gateway |
| `make test` | `go test ./...` | 运行 Gateway 全部测试 |
| `make build` | `go build -o bin/gateway ./cmd/gateway` | 构建可执行文件 |
| `make fmt` | `go fmt ./...` | 格式化 Go 代码 |

## 新增内部服务时需要修改的地方

以下以新增 `UserService` 为例。不要直接复制 Admin 的所有代码，应根据接口是否公开、是否流式、是否上传文件选择需要的部分。

### 1. 定义并发布 Proto

在仓库的 `proto` Module 中新增或修改：

```text
proto/user/v1/user.proto
```

需要完成：

- 定义 Request、Response 和 `service UserService`。
- 给公开接口配置 `(common.v1.auth).public = true`。
- 给受保护接口配置允许访问的 `roles`；未配置时默认需要登录。
- 重新生成 `*.pb.go` 和 `*_grpc.pb.go`。
- 发布新的 `proto/vX.Y.Z` Git Tag。
- 更新 Gateway `go.mod` 中的 Proto 版本；本地 `go.work` 开发时也要用 `GOWORK=off` 验证远程版本。

生成代码以后，Gateway 才能使用：

```go
userv1.NewUserServiceClient(conn)
```

### 2. 给 ClientManager 添加类型化客户端

修改：

```text
internal/grpc/client.go
```

导入 User Proto，并增加：

```go
func (cm *ClientManager) UserClient(ctx context.Context) (userv1.UserServiceClient, error) {
    return CreateClient(ctx, cm, "user-service", userv1.NewUserServiceClient)
}
```

这里的 `"user-service"` 是逻辑服务名。`ClientManager` 会据此构造 Consul gRPC 目标；它必须与 User 服务注册到 Consul 时使用的名称规则一致。

通常不需要修改 `ClientManager.GetClient`、Consul resolver 或 `round_robin`，因为这些是所有服务共享的基础能力。

### 3. 添加业务 Forwarder

新增：

```text
internal/forwarder/user.go
```

普通 Unary RPC 可以参考 `admin.go`：

```go
type UserForwarder struct {
    userv1.UnimplementedUserServiceServer
    base *BaseForwarder[userv1.UserServiceClient]
}

func NewUserForwarder(svcCtx *svc.ServiceContext, manager *grpcclient.ClientManager) *UserForwarder {
    return &UserForwarder{
        base: NewBaseForwarder(
            svcCtx,
            userv1.NewUserServiceClient,
            manager,
            "user-service",
        ),
    }
}

func (f *UserForwarder) GetProfile(
    ctx context.Context,
    req *userv1.GetProfileRequest,
) (*userv1.GetProfileResponse, error) {
    client, err := f.base.GetClient(ctx)
    if err != nil {
        return nil, err
    }
    return client.GetProfile(ctx, req)
}
```

Forwarder 负责选择下游客户端并调用 RPC，不应该包含复杂业务规则或直接操作数据库。

### 4. 添加 HTTP 路由

新增：

```text
internal/server/routes_user.go
```

普通 JSON Unary 接口使用泛型 Handler：

```go
func (s *HTTPServer) registerUserRoutes() {
    userForwarder := forwarder.NewUserForwarder(s.svcCtx, s.clientManager)
    user := s.engine.Group("/user")

    // 公开接口直接注册。
    user.POST("/register", handler.NewGrpcHandler[
        userv1.RegisterRequest,
        userv1.RegisterResponse,
    ](userForwarder.Register).Handle)

    // 受保护接口统一挂 JWT 中间件。
    protected := user.Group("")
    protected.Use(s.jwtMiddleware.Handle)
    protected.POST("/profile", handler.NewGrpcHandler[
        userv1.GetProfileRequest,
        userv1.GetProfileResponse,
    ](userForwarder.GetProfile).Handle)
}
```

然后修改路由总入口：

```text
internal/server/routes.go
```

增加：

```go
func (s *HTTPServer) registerRoutes() {
    s.registerHealthRoutes()
    s.registerAdminRoutes()
    s.registerLLMRoutes()
    s.registerUserRoutes()
}
```

### 5. 传递用户身份 metadata

受 JWT 保护的路由在调用内部服务前，必须把 Gin Context 中的身份转为 gRPC outgoing metadata：

```text
x-user-id
x-user-role
x-session-id
x-token-id
```

否则内部服务的 Auth Interceptor 无法取得身份，会返回 `Unauthenticated`。新增服务应复用统一的 metadata 注入函数或客户端拦截器，不要在每个 Forwarder 中重复拼装。

公开接口不要求用户身份 metadata，但 Gateway 到内部服务的 mTLS 仍然应该保留。

### 6. 按接口类型选择 Handler

| 接口类型 | Gateway 实现方式 | 参考文件 |
| --- | --- | --- |
| 普通 Unary JSON | `NewGrpcHandler[Req, Resp]` | `routes_admin.go` |
| 服务端流式输出 | 专用 Handler：循环 `stream.Recv()` 并输出 SSE | `routes_llm.go`、`grpc_handler.go` |
| 小文件上传 | `NewUploadedFileHandler`，Multipart 转 protobuf bytes | `grpc_handler.go` |
| 客户端流/双向流 | 单独编写连接和收发循环，不套 Unary 泛型 Handler | 后续按具体业务实现 |

流式请求必须持续使用请求的 `context.Context`，客户端断开或超时后才能及时停止下游 RPC，避免 goroutine 泄漏。

### 7. 是否注册到 Gateway 的入站 gRPC Server

如果 Gateway 只提供 HTTP→gRPC 转换，不需要把 User Proto 注册到 Gateway 的入站 gRPC Server。

只有确实需要外部或其他服务以 gRPC 调用 Gateway，并由 Gateway 再转发时，才修改：

```text
internal/forwarder/register.go
```

加入：

```go
userForwarder := NewUserForwarder(svcCtx, clientManager)
userv1.RegisterUserServiceServer(grpcServer, userForwarder)
```

这一步与 HTTP 路由是两条不同入口，不要因为增加 HTTP 接口就默认注册入站 gRPC 服务。

### 8. 配置和 Consul

检查以下内容：

- User 服务以与 Gateway 客户端一致的逻辑名称注册到 Consul。
- 注册协议必须是 `grpc`，健康检查端口是 User gRPC Server 端口。
- User Server 已注册标准 `grpc.health.v1.Health` 服务。
- Gateway 不需要为每个新服务复制 resolver；共享 resolver 会按目标服务名发现实例。
- 如果新服务需要独立 TLS ServerName、超时或熔断策略，再扩展配置；没有差异时复用全局 gRPC 配置。

### 9. 最低测试清单

新增服务至少验证：

- Proto 生成代码可以编译。
- ClientManager 能创建并复用该服务的 ClientConn。
- 公开 HTTP 路由不要求 Access Token。
- 受保护路由缺少或携带无效 Token 时被 Gateway 拒绝。
- Gateway 能把用户身份 metadata 传到内部服务。
- 内部服务能按 Proto roles 放行或拒绝。
- 下游不可用、超时和取消能够正确转换成 HTTP 错误。
- 如果是流式请求，客户端断开后下游流和 goroutine 能退出。
- 执行 `go test ./...`，并使用 `GOWORK=off` 再验证一次远程 Proto 依赖。

### 新增服务修改清单速查

```text
[必须] proto/<service>/v1/*.proto
[必须] 重新生成并发布 Proto 版本
[必须] internal/grpc/client.go
[必须] internal/forwarder/<service>.go
[必须] internal/server/routes_<service>.go
[必须] internal/server/routes.go
[必须] JWT 身份到 gRPC metadata 的传递
[按需] internal/handler/ 下的流式或上传 Handler
[按需] internal/forwarder/register.go（仅入站 gRPC 转发）
[按需] internal/config/config.go 和 etc/gateway.yaml
[必须] 对应单元测试/集成测试
```

### 目录职责约束

- `app` 只负责创建依赖、启动服务和释放资源。
- `server` 只负责入站 Server 和路由注册，不创建基础设施依赖。
- `handler` 负责 HTTP 参数、响应、SSE 和协议转换。
- `forwarder` 负责具体 gRPC 业务调用以及可选的 gRPC→gRPC 转发。
- `grpc` 负责连接管理，不包含具体 HTTP 业务。
- `consul` 负责服务发现并把地址列表交给 grpc-go。

`server.registerRoutes()` 只负责调用各业务域的注册方法。新增服务时创建独立的
`routes_<service>.go`，避免所有路由堆积在一个函数中：

```go
func (s *Server) registerRoutes() {
    s.registerHealthRoutes()
    s.registerAdminRoutes()
    s.registerLLMRoutes()
}
```

依赖方向保持为：

```text
app
→ server
→ handler / forwarder
→ grpc ClientManager
→ consul resolver
```

## gRPC 服务发现和负载均衡

`ClientManager` 使用自定义 Consul resolver 创建连接目标：

```text
consul:///admin-service-grpc
consul:///llm-service-grpc
```

resolver 查询 Consul 健康实例，将地址转换为 `resolver.Address`，再通过
`UpdateState` 交给 `grpc.ClientConn`。grpc-go 为地址维护 SubConn，并由
`round_robin` 从处于 `READY` 状态的连接中轮流选择后端。
