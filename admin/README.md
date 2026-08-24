# Admin Service

## 本地启动

Admin 是内部 gRPC 服务，不启动 Gin/HTTP 业务端口。当前监听：

```text
gRPC: 127.0.0.1:9082
```

### 初始化数据库表结构

数据库本身需要提前创建，当前本地配置使用数据库和用户 `admindb`。密码只通过环境变量提供，不写入 YAML：

```powershell
cd D:\grpc_go_project\admin
$env:ADMIN_DB_PASSWORD = "本地 MySQL 密码"
go run ./cmd/migrate -f etc/admin.yaml
```

迁移命令根据当前 GORM 模型创建或更新：

```text
admins
role
permissions
admin_sessions
operation_log
admin_roles
role_permission
```

出现下面输出表示迁移成功：

```text
admin database migration completed
```

该命令当前使用 GORM `AutoMigrate`，适合本地开发和集成测试。它不会创建初始管理员，也不会删除已有字段或数据；生产环境后续应改用经过评审、可回滚的版本化 SQL。

迁移命令包含一个明确的旧结构兼容步骤：如果检测到旧版 `admins.password` 列，会先把非空值复制到 `password_hash`，再删除旧列。这样新版代码插入管理员时不会再触发 `Field 'password' doesn't have a default value`。该清理只针对这个已经确认废弃的字段。

### 创建本地初始管理员

迁移完成后，通过环境变量提供账号和明文密码。Seed 命令会生成 bcrypt Hash，并在一个事务中创建管理员、`super_admin` 角色及关联关系：

```powershell
cd D:\grpc_go_project\admin
$env:ADMIN_INITIAL_USERNAME = "admintest"
$env:ADMIN_INITIAL_PASSWORD = "本地测试密码"
go run ./cmd/seed-admin -f etc/admin.yaml
Remove-Item Env:ADMIN_INITIAL_PASSWORD
```

该命令拒绝覆盖已经存在的同名管理员，避免误重置密码。明文密码不会写入数据库、配置文件或 Git。

启动前需要准备 MySQL、Redis、Consul 和 Jaeger。Consul 与 Jaeger 由项目 Compose 管理：

```powershell
cd D:\grpc_go_project
docker compose -f deploy\observability\compose.yaml up -d
```

在新的 PowerShell 中设置本地密钥并启动 Admin：

```powershell
cd D:\grpc_go_project\admin
$env:ADMIN_DB_PASSWORD = "请替换为本地 MySQL 密码"
$env:ADMIN_REDIS_PASSWORD = "123123"
$env:ADMIN_ACCESS_TOKEN_SECRET = "请替换为本地测试密钥"
go run ./cmd/admin
```

如果 Consul 使用 ACL，还需要设置：

```powershell
$env:ADMIN_CONSUL_TOKEN = "请替换为 Consul Token"
```

当前开发模式 Consul没有启用 ACL，可以不设置该变量。Admin 与 Gateway 的 Access Token Secret 必须相同。

Admin 日志默认位于：

```text
D:\grpc_go_project\admin\logs\admin.log
```

持续查看日志：

```powershell
Get-Content D:\grpc_go_project\admin\logs\admin.log -Tail 100 -Wait
```

### Consul 注册与健康检查

Admin 启动后注册：

```text
admin-service-grpc
```

Consul 调用标准的 `grpc.health.v1.Health/Check` 判断实例状态。由于 Consul 在 Docker 中、Admin 在 Windows 中，本地配置为：

```yaml
consul:
  host: "127.0.0.1"
  port: 8500
  check_host: "host.docker.internal"
  check_interval: "10s"
  check_timeout: "5s"
  deregister_critical_after: "90s"
```

Admin gRPC Server 必须已经监听 `9082`，并注册标准 gRPC Health Server，检查才会变为健康。修改配置后重启 Admin，使其重新注册到 Consul。

### Jaeger 链路追踪

Admin 将 gRPC Server Span 上报到 `127.0.0.1:4317`。调用链正常传播时，Jaeger 中可以看到：

```text
Gateway HTTP Span
→ Gateway gRPC Client Span
→ Admin gRPC Server Span
→ Admin 业务处理
```

浏览器访问 `http://127.0.0.1:16686`，选择 `admin-service` 查询。Jaeger 只负责链路数据，不参与 Admin 业务请求和身份认证。

Admin 是项目的后台管理 gRPC 服务。外部 HTTP 请求由 Gateway 接收并转换为 Admin gRPC 调用。

## 目录结构

```text
admin/
├── cmd/admin/             程序入口
├── etc/                   本地配置文件
├── internal/
│   ├── app/               依赖组装、启动和优雅退出
│   ├── config/            配置结构与加载
│   ├── database/          GORM 连接、连接池配置和底层 DB 获取
│   ├── handler/           protobuf gRPC Server 接口实现
│   ├── middleware/        gRPC unary/stream 拦截器
│   ├── model/             领域模型和持久化模型
│   ├── redis/             Redis Client 创建与基础操作封装
│   ├── repository/        数据访问接口与实现
│   ├── server/            gRPC Server 创建、注册和生命周期
│   ├── service/           业务逻辑
│   └── svc/               共享业务依赖
├── migrations/            数据库迁移
└── test/integration/      集成测试
```

## 依赖方向

```text
cmd/admin
→ internal/app
→ internal/server
→ internal/handler
→ internal/service
→ internal/repository
→ internal/model
```

`app` 是组合根，负责创建长期依赖并按相反顺序关闭。`handler` 只负责 gRPC 参数转换，业务规则放在 `service`，数据库访问放在 `repository`。

GORM 相关职责拆分如下：

```text
database   → 创建 *gorm.DB、设置连接池
model      → 定义表模型
repository → 使用 *gorm.DB 执行查询和事务
app        → 创建并关闭数据库资源
```

## 当前阶段

当前只建立可编译骨架。下一步依次实现配置加载、数据库连接、gRPC Server、健康检查和 AdminService 最小接口。
