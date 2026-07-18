storage "file" {
  path = "/openbao/data"
}

listener "tcp" {
  address       = "192.168.3.160:443"
  tls_cert_file = "/openbao/tls/cert.pem"
  tls_key_file  = "/openbao/tls/key.pem"
}

api_addr      = "https://192.168.3.160:443"
cluster_addr  = "https://192.168.3.160:8201"
disable_mlock = true
ui            = true
