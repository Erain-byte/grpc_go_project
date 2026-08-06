# Gateway

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
├── cmd/gateway/              程序入口
├── etc/                      Gateway 配置
├── internal/
│   ├── app/                  依赖组装与应用生命周期
│   ├── config/               配置结构和加载
│   ├── consul/               Consul 注册发现与 grpc-go resolver
│   ├── forwarder/            Admin、LLM gRPC 转发逻辑
│   ├── grpc/                 ClientManager、连接缓存与 TLS
│   ├── handler/              HTTP→gRPC、SSE、上传转换
│   ├── logger/               日志初始化
│   ├── middleware/           Gin 中间件与统一错误处理
│   ├── repository/           数据访问层预留
│   ├── server/               HTTP Server 与 Gin 路由注册
│   │   ├── http.go           HTTPServer 创建、启动与关闭
│   │   ├── grpc.go           GRPCServer 创建、注册、启动与关闭
│   │   ├── routes.go         路由注册总入口
│   │   ├── routes_health.go  健康检查路由
│   │   ├── routes_admin.go   Admin 路由
│   │   └── routes_llm.go     LLM 路由
│   └── svc/                  Redis、Registry 等共享服务上下文
├── pkg/apperror/             应用错误及 HTTP/gRPC 错误转换
└── test/                     集成测试目录
```

根仓库中的 `../proto` 是独立 Go Module，保存 Admin、LLM 的 protobuf 定义和生成代码。

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
