# Consul KV 初始化配置

这里保存 Consul KV 的初始化源文件，不包含密码、Token、JWT Secret 或 TLS 私钥。

## KV 路径

| 文件 | Consul KV Key |
| --- | --- |
| `global-runtime.json` | `grpc-go/config/global/runtime` |
| `gateway-runtime.json` | `grpc-go/config/gateway/runtime` |
| `admin-runtime.json` | `grpc-go/config/admin/runtime` |

## 写入配置

在项目根目录执行：

```powershell
Get-Content .\deploy\consul\kv\global-runtime.json -Raw |
    docker exec -i grpc-go-consul consul kv put grpc-go/config/global/runtime -

Get-Content .\deploy\consul\kv\gateway-runtime.json -Raw |
    docker exec -i grpc-go-consul consul kv put grpc-go/config/gateway/runtime -

Get-Content .\deploy\consul\kv\admin-runtime.json -Raw |
    docker exec -i grpc-go-consul consul kv put grpc-go/config/admin/runtime -
```

## 验证配置

```powershell
docker exec grpc-go-consul consul kv get grpc-go/config/global/runtime
docker exec grpc-go-consul consul kv get grpc-go/config/gateway/runtime
docker exec grpc-go-consul consul kv get grpc-go/config/admin/runtime
```

这些 JSON 目前只是初始化源。Gateway 和 Admin 还需要实现 KV 加载、字段校验、Blocking Query Watch 和最后有效配置快照，配置变化才会自动作用到运行中的服务。
