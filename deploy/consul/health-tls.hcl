# Use the Consul agent client certificate for HTTP and gRPC health checks.
# This does not enable HTTPS on the local Consul HTTP API.
enable_agent_tls_for_checks = true
data_dir = "/consul/data"
# Local single-node data may be reopened after a long development pause.
server_rejoin_age_max = "87600h"

tls {
  defaults {
    ca_file   = "/consul/certs/ca.crt"
    cert_file = "/consul/certs/client.crt"
    key_file  = "/consul/certs/client.key"
  }
}
