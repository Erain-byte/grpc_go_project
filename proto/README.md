# Protocol Buffers 协议规范

`proto` 是独立的 Go Module，也是所有服务共享的 API 契约。业务服务只能依赖这里生成的代码，不应手工修改 `*.pb.go` 或 `*_grpc.pb.go`。

## 目录模板

```text
proto/
├── common/v1/              # 跨领域且稳定的公共协议、Method Option
├── <domain>/v1/            # 一个业务领域的第一个兼容版本
│   ├── <domain>.proto      # Service、Request、Response 和领域消息
│   ├── <domain>.pb.go      # protoc-gen-go 生成
│   └── <domain>_grpc.pb.go # protoc-gen-go-grpc 生成
├── go.mod
└── go.sum
```

当前项目规模下，一个领域先使用一个 `.proto` 文件更容易查找。当单个文件明显过大时，再按职责拆成 `service.proto`、`types.proto`，不要提前拆成大量小文件。

## 新增服务模板

```proto
syntax = "proto3";

package order.v1;

import "common/v1/security.proto";

option go_package = "github.com/Erain-byte/grpc_go_project/proto/order/v1;orderv1";

// OrderService 提供订单领域能力。
service OrderService {
  rpc GetOrder(GetOrderRequest) returns (GetOrderResponse) {
    option (common.v1.auth) = {
      roles: "user"
      roles: "admin"
    };
  }
}

message GetOrderRequest {
  string order_id = 1;
}

message GetOrderResponse {
  Order order = 1;
}

message Order {
  string id = 1;
}
```

## 必须遵守的规则

1. package 使用 `<domain>.v1`；Go 包使用 `<domain>v1`。
2. Service 以 `Service` 结尾；RPC 使用动词开头；消息使用 PascalCase；字段使用 snake_case。
3. 每个 RPC 使用独立的 `XxxRequest` 和 `XxxResponse`，不要复用请求/响应消息。
4. 字段编号一旦发布永不复用。删除字段时同时 `reserved` 原编号和名称。
5. 枚举的第一个值必须为 `0`，名称采用 `<ENUM>_UNSPECIFIED`。
6. 时间点使用 `google.protobuf.Timestamp`，时间长度使用 `google.protobuf.Duration`。
7. 业务失败优先使用 gRPC status code 和 error details；新的响应不再重复添加 `success`、`message`。
8. 身份、Trace、Request ID 等调用上下文通过受信任的 gRPC metadata 传递，不放进每个业务 Request。
9. 默认需要认证；公开 RPC 必须显式设置 `(common.v1.auth).public = true`，权限通过 `roles` 声明。
10. `common` 只存放真正跨领域且语义稳定的类型，避免形成所有服务耦合的“大杂烩”。

## 生成与检查

推荐安装 [Buf CLI](https://buf.build/docs/cli/installation/)，然后进入独立的 `proto` 模块执行：

```powershell
cd proto
buf format -w
buf lint
buf generate
```

检查相对于远程主分支是否存在破坏性变更：

```powershell
buf breaking --against "https://github.com/Erain-byte/grpc_go_project.git#branch=main,subdir=proto"
```

生成后执行：

```powershell
go test ./proto/...
go test ./gateway/...
go test ./admin/...
```

如果本机尚未安装 Buf，可在 `proto` 目录使用 `protoc`：

```powershell
protoc -I . --go_out=paths=source_relative:. --go-grpc_out=paths=source_relative:. admin/v1/admin.proto auth/v1/auth.proto common/v1/security.proto gateway/v1/gateway.proto llm/v1/llm.proto user/v1/user.proto
```

## 版本发布

兼容修改继续发布 `v1`：新增 RPC、新增消息、在末尾使用新编号增加字段。删除或改名 RPC、改变字段类型、复用字段编号等破坏性修改必须创建新的协议版本目录，例如 `admin/v2`。

仓库使用子模块标签发布 proto，例如：

```powershell
git tag proto/v0.3.0
git push origin proto/v0.3.0
```

本地联调可以使用 `go.work`；远程构建不依赖 `go.work`，而是依赖已经推送的 proto tag。

## 当前 v1 的兼容性说明

现有 `admin.v1`、`user.v1` 中部分 Response 带有 `success/message`，部分旧 Request 携带 Token。这些属于已经被调用方依赖的 v1 契约，不能直接删除。后续修改对应业务时应先停止继续复制这种设计；需要彻底清理时建立 `v2`，并给 gateway 和服务端保留迁移窗口。
