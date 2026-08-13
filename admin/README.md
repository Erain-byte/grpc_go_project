# Admin Service

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
