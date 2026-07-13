# Gateway

使用 Go 编写的网关服务。

## 本地开发

```bash
go run ./cmd/gateway
go test ./...
```

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

- `cmd/gateway`：应用程序入口
- `internal/app`：应用组装、启动及生命周期管理
- `internal/config`：配置加载与管理
- `internal/handler`：HTTP、gRPC 等入站请求处理器
- `internal/middleware`：HTTP 中间件与 gRPC 拦截器
- `internal/service`：业务逻辑层
- `internal/repository`：数据访问层
- `pkg`：可供其他模块复用的公共组件
- `api/proto`：Protocol Buffer 接口定义
- `configs`：配置文件模板
- `deployments`：部署清单
- `test`：集成测试与端到端测试
