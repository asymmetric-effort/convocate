storage "file" {
  path = "/openbao/data"
}

listener "tcp" {
  address     = "0.0.0.0:8200"
  tls_disable = true
}

audit {
  type        = "file"
  path        = "file"
  description = "Audit log"
  options = {
    file_path = "/openbao/audit/audit.log"
  }
}

telemetry {
  prometheus_retention_time      = "30s"
  disable_hostname               = true
  unauthenticated_metrics_access = true
}

api_addr      = "OPENBAO_API_ADDR"
cluster_addr  = "OPENBAO_CLUSTER_ADDR"
disable_mlock = true
ui            = true
