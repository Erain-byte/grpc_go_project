# 本地基础设施

Compose 当前管理：

- Consul：服务注册、发现和健康检查。
- Jaeger All-in-One：OTLP Trace 接收、临时存储和查询页面。
- RabbitMQ：异步消息、操作日志队列和后续事件驱动功能。

当前不需要额外启动 OpenTelemetry Collector、Tempo 或 Grafana，也不再需要单独运行 `D:\consul\consul.exe`。

在项目根目录执行：

```powershell
docker compose -f deploy/observability/compose.yaml up -d
```

检查容器：

```powershell
docker compose -f deploy/observability/compose.yaml ps
Test-NetConnection 127.0.0.1 -Port 8500
Test-NetConnection 127.0.0.1 -Port 4317
```

Consul 管理页面：

```text
http://127.0.0.1:8500
```

启动 Gateway 和 Admin 并调用一次接口后，访问：

```text
http://127.0.0.1:16686
```

在 Jaeger 的 Service 下拉框中选择 `gateway-service` 或 `admin-service` 查询调用链。

停止本地 Consul 和 Jaeger：

```powershell
docker compose -f deploy/observability/compose.yaml down
```

## 日常启动与配置更新

Docker Desktop 启动后，`restart: unless-stopped` 会自动恢复已经创建过的容器。首次创建容器，或者修改了 `compose.yaml` 后，在项目根目录执行：

```powershell
cd D:\grpc_go_project
docker compose -f deploy\observability\compose.yaml up -d
docker compose -f deploy\observability\compose.yaml ps
```

修改 `gateway/etc/gateway.yaml` 或 `admin/etc/admin.yaml` 不需要重新创建 Docker 容器，只需要重启对应的 Go 服务。`docker restart` 只重启旧容器，不会应用 Compose 文件的新配置。

常用排查命令：

```powershell
docker ps -a
docker logs -f grpc-go-consul
docker logs -f grpc-go-jaeger
Test-NetConnection 127.0.0.1 -Port 8500
Test-NetConnection 127.0.0.1 -Port 4317
```

当前端口：

| 组件 | 端口 | 用途 |
| --- | ---: | --- |
| Consul | 8500 | HTTP API 和管理页面 |
| Consul | 8600 | DNS 接口，当前业务暂未使用 |
| Jaeger | 4317 | Gateway/Admin 上报 OTLP gRPC Trace |
| Jaeger | 4318 | OTLP HTTP，当前业务暂未使用 |
| Jaeger | 16686 | Trace 查询页面 |
| RabbitMQ | 5672 | Go 服务使用的 AMQP 连接端口 |
| RabbitMQ | 15672 | Management 管理页面 |

Consul 运行在容器中，容器内的 `127.0.0.1` 指向容器自身。Gateway 和 Admin 运行在 Windows 主机上，因此应用配置使用：

```yaml
consul:
  host: "127.0.0.1"
  check_host: "host.docker.internal"
```

`host` 是 Go 程序连接 Consul API 的地址；`check_host` 是 Consul 容器访问 Windows Go 服务的地址。两者不能混为一个字段。

## RabbitMQ

RabbitMQ 使用独立 vhost `/grpc-go` 和本地用户 `grpc_app`。密码从当前基础设施目录 `deploy/observability/.env` 中的 `RABBITMQ_DEFAULT_PASS` 读取，该文件已被 Git 忽略；同目录只保留不含真实密码的 `.env.example`。

镜像固定为 `rabbitmq:4.3.5-management`，避免浮动的 `4-management` 标签在后续更新时无意改变本地运行版本。

首次创建或修改 RabbitMQ Compose 配置后执行：

```powershell
cd D:\grpc_go_project
docker compose -f deploy\observability\compose.yaml up -d rabbitmq
docker compose -f deploy\observability\compose.yaml ps
```

管理页面：

```text
http://127.0.0.1:15672
```

本地用户名为 `grpc_app`，密码读取 `.env` 中的 `RABBITMQ_DEFAULT_PASS`。检查日志和端口：

```powershell
docker logs -f grpc-go-rabbitmq
Test-NetConnection 127.0.0.1 -Port 5672
Test-NetConnection 127.0.0.1 -Port 15672
```
# Consul 的 mTLS 健康检查

Consul 使用 `consul/health-tls.hcl` 中的 Agent TLS 配置执行 HTTP/gRPC 检查；这不是业务 KV 热更新配置。Compose 只读挂载 `D:/local-pki/ca.crt` 与 `D:/local-pki/consul/client.crt`、`client.key`，不挂载 CA 私钥。启动前必须准备这些文件。

Admin 的 `grpc.tls_server_name` 为 `admin-service`，注册时传给 `TLSServerName`，保持 `TLSSkipVerify=false`。后续 User 服务填写自己的证书 SAN。Gateway 的入站明文 HTTP/gRPC 检查不需要套用 Admin 的 TLS 服务名。

在项目根目录应用配置（仅重建 Consul，不删除数据卷）：

```powershell
& "C:/Program Files/Docker/Docker/resources/bin/docker.exe" compose -f deploy/observability/compose.yaml up -d consul
```

随后重启 Admin 以重新注册检查；Gateway 调用 Admin 时也必须设置 `grpc.use_tls: true`。到 Consul UI 查看 Admin 检查是否为 `passing`，再测试登录。这里未实现业务 TLS 证书热更新；更换证书后需重新加载或重建 Consul，并重启使用固定 TLS 凭证的业务服务。
