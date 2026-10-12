# RabbitMQ 本地部署

RabbitMQ 已独立管理，继续使用原有外部数据卷 `observability_rabbitmq_data`，不会丢失现有队列、交换机和持久消息。

```powershell
cd D:\grpc_go_project\deploy\rabbitmq
Copy-Item .env.example .env
# 编辑 .env 后启动
docker compose up -d
docker compose ps
```

- AMQP：`127.0.0.1:5672`
- 管理页面：`http://127.0.0.1:15672`
- 用户名：`grpc_app`
- vhost：`/grpc-go`

Gateway、Admin、User 使用的 RabbitMQ 密码必须与本目录 `.env` 一致。普通 `docker compose down` 不删除外部卷；不要手动删除 `observability_rabbitmq_data`。
