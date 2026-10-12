# Jaeger 本地部署

Jaeger All-in-One 用于本地 Trace 接收和查询，不保存长期生产数据。

```powershell
docker compose -f deploy/jaeger/compose.yaml up -d
docker compose -f deploy/jaeger/compose.yaml ps
```

- OTLP gRPC：`127.0.0.1:4317`
- OTLP HTTP：`127.0.0.1:4318`
- 查询页面：`http://127.0.0.1:16686`

当前镜像仍沿用既有 `latest`。后续准备生产环境时应固定经过验证的版本，并采用适合生产的存储后端。
