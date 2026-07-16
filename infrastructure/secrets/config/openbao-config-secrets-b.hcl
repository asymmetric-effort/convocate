storage "file" {
  path = "/openbao/data"
}

listener "tcp" {
  address       = "0.0.0.0:8200"
  tls_cert_file = "/openbao/tls/cert.pem"
  tls_key_file  = "/openbao/tls/key.pem"
}

api_addr      = "https://192.168.3.161:8200"
cluster_addr  = "https://192.168.3.161:8201"
disable_mlock = true
ui            = true
