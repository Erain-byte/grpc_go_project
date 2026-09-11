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

`gateway-runtime.json` 已接入 Gateway 的统一运行配置加载流程。Gateway 会从
`grpc-go/config/gateway/runtime` 一次读取 CORS、限流、熔断和防重放策略，完整解析、
校验成功后再整体合并到本地配置；KV 缺失、包含未知字段或配置非法时拒绝启动。

目前 Gateway 的 CORS 和限流通过同一个 Consul Blocking Query Watcher 热更新，
中间件从原子配置快照读取最新规则，不需要重启 Gateway。运行期间收到非法 JSON、
非法字段或 Consul 暂时不可用时，Gateway 保留最后一份有效快照并记录错误；启动时
无法取得首份有效配置则拒绝启动。

熔断和防重放虽然已进入统一配置对象，但对应运行组件尚未接入。密码、Token、JWT
Secret 与 TLS 私钥不得写入这些 JSON。
