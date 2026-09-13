# HTTP/gRPC 健康检查使用 Agent 的 CA 和客户端证书。
# 这里只配置本地单节点探针身份，不开启 Consul HTTP API 的 HTTPS。
enable_agent_tls_for_checks = true
# 同时供 consul validate 独立校验使用，与 Compose 启动参数保持一致。
data_dir = "/consul/data"

tls {
  defaults {
    ca_file   = "/consul/certs/ca.crt"
    cert_file = "/consul/certs/client.crt"
    key_file  = "/consul/certs/client.key"
  }
}
