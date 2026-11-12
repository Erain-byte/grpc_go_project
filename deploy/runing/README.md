# 公网 IP 测试部署

本环境使用服务器 `81.71.19.90` 直接联调，不依赖域名、Nginx、CDN 或 HTTPS。
它复用 `deploy/production/compose.yaml` 的容器定义，仅通过独立环境文件切换公开地址和端口监听方式。

## 1. 创建服务器私密配置

```bash
sudo mkdir -p /opt/grpc-go/env /opt/grpc-go/secrets/mtls
sudo cp deploy/runing/.env.example /opt/grpc-go/env/runing.env
sudo chmod 600 /opt/grpc-go/env/runing.env
sudo nano /opt/grpc-go/env/runing.env
```

必须替换所有 `replace-with-...`。真实密码、Cookie Secret、Client Secret 和 API Key 不得提交到 Git。

## 2. 测试地址

| 功能 | 地址 |
|---|---|
| User Web | `http://81.71.19.90:8081` |
| Admin Web | `http://81.71.19.90:3000` |
| Gateway API | `http://81.71.19.90:9080` |
| Keycloak | `http://81.71.19.90:8180` |
| Admin OAuth2 Proxy | `http://81.71.19.90:4181` |

PostgreSQL、Redis、RabbitMQ AMQP、Consul、Jaeger和各内部 gRPC 端口不应开放公网。

## 3. Keycloak

新数据库首次导入 Realm 时，`admin-web` 会使用以下测试配置：

```text
Valid redirect URI:             http://81.71.19.90:4181/oauth2/callback
Valid post logout redirect URI: http://81.71.19.90:4181/*
Web origin:                     http://81.71.19.90:3000
```

如果服务器已经存在 `grpc-project` Realm，Keycloak 不会用导入文件覆盖它，必须在管理台手动修改上述三项，并确认 Client Secret 与 `runing.env` 一致。

用户端 A 包当前无需登录。以后启用 `user-app` 登录时，再增加：

```text
Valid redirect URI: http://81.71.19.90:8081/oauth/callback
Web origin:         http://81.71.19.90:8081
```

## 4. 校验并构建

```bash
docker compose \
  --env-file /opt/grpc-go/env/runing.env \
  -f deploy/production/compose.yaml \
  config --quiet

docker compose \
  --env-file /opt/grpc-go/env/runing.env \
  -f deploy/production/compose.yaml \
  build gateway admin user web3
```

配置没有校验通过时不要启动容器或执行迁移。

## 5. 安全组

测试期间仅向固定测试 IP 开放 `8081`、`3000`、`9080`、`8180`、`4181`。不要对全网长期开放。
HTTP 测试环境只能使用临时测试账号和测试数据，不应传输正式密钥或真实用户数据。

正式上线时切回 `deploy/production/.env.example` 对应的服务器私密配置，并启用域名、HTTPS 和 Nginx。
