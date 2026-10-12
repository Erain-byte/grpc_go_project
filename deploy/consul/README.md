# Consul 本地部署

Consul 已独立管理，保存服务注册、发现和 KV 配置。现有数据继续使用外部卷 `observability_consul_data`。

```powershell
docker compose -f deploy/consul/compose.yaml config --quiet
docker compose -f deploy/consul/compose.yaml up -d
docker compose -f deploy/consul/compose.yaml ps
```

管理页面为 `http://127.0.0.1:8500`，DNS 端口为 `8600`。KV 初始化文件保存在 `deploy/consul/kv`。

健康检查需要 `D:\local-pki\ca.crt`、`D:\local-pki\consul\client.crt` 和 `D:\local-pki\consul\client.key`。

普通 `docker compose down` 不删除外部数据卷。不要手动删除 `observability_consul_data`，除非已完成备份并明确要重置 Consul。
