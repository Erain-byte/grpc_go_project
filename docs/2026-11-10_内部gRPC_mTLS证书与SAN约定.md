# 内部 gRPC mTLS 证书与 SAN 约定

## 1. 核心约定

本项目内部 gRPC 统一遵守：

```text
Consul serviceName = 服务端证书公共 DNS SAN
```

例如：

| 服务 | Consul serviceName | 服务端证书必须包含的 DNS SAN |
| --- | --- | --- |
| Admin | `admin-service` | `admin-service` |
| User | `user-service` | `user-service` |
| Web3 | `web3-service` | `web3-service` |

客户端创建连接时，直接使用目标服务名校验证书：

```go
tlsConfig := &tls.Config{
    MinVersion: tls.VersionTLS12,
    ServerName: strings.TrimSpace(serviceName),
}
```

因此不需要维护 `serviceName -> ServerName` 映射。只有未来 Consul 服务名与证书 SAN 无法保持一致时，才需要单独设计映射。

## 2. ServerName、SAN 和证书路径的区别

三者含义不同：

```text
CertFile / KeyFile = 当前调用方自己的证书和私钥
CAFile             = 用于验证对方证书签名的公共 CA
ServerName         = 客户端期望连接的服务身份
DNS SAN            = 服务端证书声明自己能够代表的服务身份
```

`ServerName` 不用于查找证书目录，客户端也不会读取目标服务保存在本机的证书文件。

以 `User -> Web3` 为例：

```text
1. User 从本地读取 user/service.crt 和 user/service.key。
2. User 通过 Consul 发现 web3-service 实例。
3. Web3 在 TLS 握手中返回自己的 web3/server.crt。
4. User 使用 ca/ca.crt 验证 Web3 证书签名。
5. User 检查 Web3 证书 DNS SAN 是否包含 web3-service。
```

只要签名可信且 SAN 匹配，TLS 身份验证才通过。

## 3. 多实例约定

同一逻辑服务可以有多个实例：

```text
user-service
├─ user-1：10.0.0.11:9083
└─ user-2：10.0.0.12:9083
```

每个实例可以使用独立证书，但每张证书都必须包含公共服务 SAN：

```text
user-1 证书 DNS SAN：
- user-service
- user-1.internal

user-2 证书 DNS SAN：
- user-service
- user-2.internal
```

客户端统一使用：

```go
ServerName: "user-service"
```

这样 Consul 无论选择哪个健康实例，证书都能通过 `user-service` 身份校验。

普通 TLS 校验确认的是“对方是不是该逻辑服务的合法实例”，不会确认具体选中了 `user-1` 还是 `user-2`。如果未来必须验证具体工作负载身份，应使用独立实例证书配合 SPIFFE/SPIRE、Service Mesh 或 URI SAN 校验，而不是增加普通字符串映射。

## 4. 当前证书目录

本地开发证书统一放在：

```text
certs/mtls/
├─ ca/
│  ├─ ca.crt
│  └─ ca.key
├─ gateway/
│  ├─ service.crt
│  └─ service.key
├─ admin/
│  ├─ service.crt
│  └─ service.key
├─ user/
│  ├─ service.crt
│  └─ service.key
├─ web3/
│  ├─ server.crt
│  └─ server.key
└─ consul/
   ├─ client.crt
   └─ client.key
```

整个 `certs/` 目录已被 Git 忽略，不得提交证书私钥或 CA 私钥。

## 5. 当前调用链

```text
Gateway --mTLS--> Admin
Gateway --mTLS--> User
Admin   --mTLS--> User
User    --mTLS--> Web3
Consul  --mTLS--> Gateway / Admin / User / Web3 健康检查
```

LLM 尚未接入 mTLS，目前通过 `plaintext_services` 作为迁移期例外。LLM 完成证书配置后，应删除该明文例外。

## 6. 证书生成与检查

本地重新生成证书：

```powershell
$env:GOWORK = "off"
go run scripts/generate-mtls/main.go
```

生成脚本必须保证服务证书 SAN 与 Consul 服务名一致。修改服务名时，需要同步检查：

1. 服务配置中的 `name`；
2. Consul 注册使用的 `serviceName`；
3. 证书生成脚本中的 DNS SAN；
4. 调用方传入 `ClientManager.GetClient()` 的服务名。

只修改其中一处会导致类似错误：

```text
x509: certificate is valid for user-service, not another-service
```

Gateway、Admin、User、Web3 在启动时还会读取本地服务端证书，并执行：

```go
if err := leaf.VerifyHostname(cfg.TLSServerName); err != nil {
    return fmt.Errorf(
        "server TLS certificate SAN does not match service name %q: %w",
        cfg.TLSServerName,
        err,
    )
}
```

因此 SAN 与 `tls_server_name` 不一致时，服务会直接拒绝启动，不会等到第一个业务请求到达后才暴露错误。客户端在 TLS 握手阶段还会使用目标 `serviceName` 再验证一次远程证书。

## 7. 生产环境注意事项

1. 不要使用仓库绝对路径，容器中应统一挂载到受控目录，例如 `/etc/project-certs`。
2. CA 私钥不应放在业务服务器，应交给离线 CA、KMS、Vault 或企业证书系统保管。
3. 服务证书应设置合理有效期，并配置到期监控和自动轮换。
4. 严禁在生产环境启用 `InsecureSkipVerify`。
5. 严禁为了让多个服务共用固定 `ServerName`，给无关服务证书加入相同 SAN。
6. 如果实例共享一张服务证书，任一实例私钥泄露都会影响整个服务；更严格的生产方案应为实例签发独立证书，同时保留公共服务 SAN。

