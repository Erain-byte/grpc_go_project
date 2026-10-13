# 内部服务通信认证与授权设计

更新时间：2026-10-13

## 1. 目标

项目中的内部 gRPC 通信需要同时解决三个问题：

1. **用户身份认证**：请求代表哪个 Keycloak 用户。
2. **服务身份认证**：请求实际由 Gateway、Admin、User 中的哪个服务发出。
3. **服务调用授权**：该服务是否允许调用当前 gRPC 方法。

三者不能互相替代。Consul 只负责发现健康实例，不负责证明请求来源，也不负责 RPC 授权。

```text
浏览器
  │ Keycloak Access Token
  ▼
Gateway
  │ mTLS + 用户身份
  ▼
Admin / User
  │ mTLS + 用户身份
  ▼
User / LLM
```

## 2. 当前采用的安全边界

### 2.0 当前实现状态（2026-10-13）

| 能力 | 当前状态 |
| --- | --- |
| Gateway 校验 Keycloak JWT | 已实现基本链路 |
| Gateway 将原始 Bearer Token 透传到 gRPC metadata | 已实现 |
| Admin Unary 独立校验 Keycloak Token 与 Proto 角色 | 已实现 |
| Admin Stream 独立校验 Keycloak Token 与 Proto 角色 | 已实现 |
| Admin mTLS 客户端证书 CA 验证 | 已实现，可配置 |
| 客户端证书 SAN 服务身份校验 | 未实现 |
| 按 gRPC FullMethod 配置调用服务白名单 | 未实现 |
| Admin Stream RPC 服务身份授权 | 未实现 |
| 授权策略 Consul KV 热更新 | 规划中，当前不阻塞 User 基本流程 |

因此，当前已经具备外部认证和内部传输安全基础，但内部服务认证、服务授权及流式接口保护仍需完善。

### 2.1 外部请求

Gateway 校验 Keycloak Access Token：

- RS256 签名和 `kid`
- `iss`
- `aud` 必须包含 `grpc-api`
- `exp`、`nbf`
- `sub`、`preferred_username`
- `realm_access.roles`

### 2.2 内部请求

内部 gRPC 使用 mTLS：

- 客户端验证服务端证书，防止连接到伪造服务。
- 服务端验证客户端证书，确认调用方持有受信 CA 签发的证书。
- 从已验证客户端证书的 SAN 提取服务身份。
- 根据“服务身份 + gRPC FullMethod”执行白名单授权。

不要使用来源 IP、Consul 服务名或客户端自行填写的 `x-service-name` 作为可信身份。这些值可以被伪造或错误配置。

## 3. 服务证书身份约定

每个服务使用独立证书，并在 DNS SAN 中保存稳定的服务身份：

| 服务 | DNS SAN |
| --- | --- |
| Gateway | `gateway-service` |
| Admin | `admin-service` |
| User | `user-service` |
| LLM | `llm-service` |

服务端只能从已经通过 CA 验证的证书链提取身份。生产环境禁止使用 `InsecureSkipVerify`。

## 4. RPC 授权策略

策略采用默认拒绝：没有配置的方法拒绝访问；方法已配置但调用服务不在白名单中也拒绝访问。

开发阶段先在服务 YAML 中保存静态规则：

```yaml
grpc_authorization:
  rules:
    "/user.v1.UserService/GetUserInfo":
      - gateway-service
      - admin-service

    "/user.v1.UserService/GetUserList":
      - admin-service

    "/user.v1.UserService/UpdateUserInfo":
      - gateway-service

    "/user.v1.UserService/Chat":
      - gateway-service

    "/user.v1.UserService/StreamChat":
      - gateway-service
```

LLM 服务建议采用：

```yaml
grpc_authorization:
  rules:
    "/llm.v1.LlmService/Chat":
      - user-service
      - admin-service

    "/llm.v1.LlmService/StreamChat":
      - user-service
      - admin-service
```

具体规则应以最终 Proto 的 service package 和方法名为准，不能使用 HTTP 路径代替 gRPC FullMethod。

## 5. 拦截器判断流程

Unary 和 Stream RPC 都必须配置授权拦截器。流程如下：

```text
获取 peer.AuthInfo
  ↓
确认连接使用 TLS
  ↓
确认客户端证书链已经验证
  ↓
从客户端证书 DNS SAN 提取服务身份
  ↓
读取当前 RPC 的 FullMethod
  ↓
查询内存授权快照
  ↓
白名单匹配则放行，否则返回 PermissionDenied
```

身份缺失、没有使用 TLS 或证书未验证时返回 `Unauthenticated`；身份有效但无权调用方法时返回 `PermissionDenied`。

伪代码：

```go
func authorize(ctx context.Context, fullMethod string, rules Rules) error {
    caller, err := serviceIdentityFromVerifiedCertificate(ctx)
    if err != nil {
        return status.Error(codes.Unauthenticated, "missing trusted service identity")
    }

    allowed, exists := rules[fullMethod]
    if !exists || !slices.Contains(allowed, caller) {
        return status.Error(codes.PermissionDenied, "calling service is not authorized")
    }
    return nil
}
```

流式接口不能只保护建立流的 HTTP 入口。`StreamServerInterceptor` 必须在 gRPC 服务端建立流时完成相同检查。

## 6. 服务权限与用户权限

mTLS 只证明调用服务身份。例如 `admin-service` 证书不能证明当前操作用户拥有管理员角色。

敏感操作需要同时满足：

```text
可信服务身份
AND
可信用户身份与角色
```

例如 `User.GetUserList`：

```text
调用服务必须是 admin-service
AND
最终用户角色必须包含 admin 或 super-admin
```

用户身份来自 Keycloak Access Token。Gateway 先验证 Token，然后通过标准 `authorization: Bearer <token>` gRPC metadata 透传原始 Token；Admin、User 和 LLM 分别使用 JWKS 独立验证签名、Issuer、Audience、有效期和角色。内部服务不得使用 `x-user-role` 等普通 metadata 作为最终授权依据。

Gateway 接收外部 HTTP 请求时，必须删除客户端提供的内部身份 Header，再根据已经验证的 Keycloak Token 重新生成 gRPC metadata。禁止直接复制客户端提交的 `x-user-id`、`x-user-role`、`x-service-name` 等字段。

Admin 向 User 或 LLM 转发请求时，继续透传原始 `authorization` metadata。下游服务必须再次验证 Token，不能直接信任 Admin 解析出的角色，也不能从普通请求字段重新构造用户身份。

## 7. 内部端口与网络隔离

Admin、User 和 LLM 的 gRPC 端口只能对内部网络或明确的调用方开放，不应直接暴露到公网。生产部署至少需要满足：

- 防火墙或安全组限制内部 gRPC 端口来源。
- 容器环境只加入必要的内部网络。
- 对外仅暴露 Gateway 或受控反向代理入口。
- 网络隔离不能替代 mTLS 和 RPC 授权，两者必须同时存在。

## 8. Consul 的职责边界

Consul 服务注册与健康检查用于回答：

```text
user-service 当前有哪些健康地址？
```

Consul 不能回答：

```text
本次请求是否真的来自 admin-service？
admin-service 是否允许调用 GetUserList？
```

即使恶意实例注册了 `admin-service` 名称，只要它没有受信 CA 签发且 SAN 正确的客户端证书，mTLS 握手或服务授权仍应拒绝它。

## 9. 从 YAML 迁移到 Consul KV

当前阶段先使用 YAML，完成 User、Admin 和 LLM 调用闭环。静态规则修改后重启服务生效。

需要动态更新时，复用已有 Consul KV 和运行时配置快照设计，不新增配置中心。建议 Key：

```text
grpc-go/security/grpc-authorization/user-service
grpc-go/security/grpc-authorization/llm-service
grpc-go/security/grpc-authorization/admin-service
```

示例值：

```json
{
  "rules": {
    "/user.v1.UserService/GetUserInfo": [
      "gateway-service",
      "admin-service"
    ],
    "/user.v1.UserService/GetUserList": [
      "admin-service"
    ]
  }
}
```

运行时流程：

```text
启动时读取 Consul KV
  ↓
严格解析并校验全部规则
  ↓
生成不可变内存快照
  ↓
每次 RPC 只查询内存快照
  ↓
Blocking Query Watcher 接收更新
  ↓
新配置校验成功后原子替换快照
```

失败策略：

- 启动时无法获得首份有效策略：拒绝启动。
- 运行期间 Consul 暂时不可用：继续使用最后一份有效快照。
- 收到非法 JSON、未知字段或非法服务身份：拒绝更新并记录错误。
- 每次 RPC 不直接查询 Consul，避免增加延迟和放大故障。

授权策略可以存入 Consul KV，但证书私钥、Consul Token、数据库密码和其他 Secret 不得写入普通 KV JSON。

## 10. 实施顺序

为避免阻塞 User 服务和 LLM 基本流程，按以下顺序实施：

1. 确认各服务证书具有唯一 DNS SAN。
2. 在 User 服务实现 Unary 和 Stream 服务身份拦截器。
3. 使用 YAML 配置 User RPC 白名单。
4. 验证 Gateway → User、Admin → User 的允许与拒绝场景。
5. 在 LLM 服务实现相同拦截规则。
6. 验证 User/Admin → LLM 的普通和流式请求。
7. 基本链路稳定后，再将授权策略迁移至 Consul KV。
8. 最后实现 Consul Watch、原子快照和故障回退。

## 11. 必须验证的场景

- 合法 Admin 证书可以调用 `User.GetUserList`。
- Gateway 证书调用 `User.GetUserList` 返回 `PermissionDenied`。
- 未提供客户端证书时 mTLS 握手失败，或者拦截器返回 `Unauthenticated`。
- 使用不受信 CA 签发的证书时连接失败。
- 合法 User 证书可以调用 LLM `Chat` 和 `StreamChat`。
- User 角色不能通过 Admin 链路执行管理员操作。
- 外部客户端伪造 `x-user-id` 或 `x-user-role` 时，Gateway 必须覆盖或删除这些值。
- 直接访问内部 gRPC 端口时，网络策略或 mTLS 必须拒绝未授权来源。
- Consul 中存在同名恶意服务注册时，证书验证仍能阻止调用。
- 动态策略更新失败时继续使用最后一份有效快照。
