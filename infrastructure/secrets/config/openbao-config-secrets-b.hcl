storage "file" {
  path = "/openbao/data"
}

listener "tcp" {
  address       = "192.168.3.161:443"
  tls_cert_file = "/openbao/tls/cert.pem"
  tls_key_file  = "/openbao/tls/key.pem"
}

telemetry {
  prometheus_retention_time      = "30s"
  disable_hostname               = true
  unauthenticated_metrics_access = true
}

api_addr      = "https://192.168.3.161:443"
cluster_addr  = "https://192.168.3.161:8201"
disable_mlock = true
ui            = true
