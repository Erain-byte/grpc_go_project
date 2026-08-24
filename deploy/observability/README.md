# 本地基础设施

Compose 当前管理：

- Consul：服务注册、发现和健康检查。
- Jaeger All-in-One：OTLP Trace 接收、临时存储和查询页面。

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
