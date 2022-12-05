provider "etcd" {
  endpoints = "127.0.0.1:32381"
  ca_cert = "../etcd-server/certs/ca.pem"
  cert = "../etcd-server/certs/root.pem"
  key = "../etcd-server/certs/root.key"
}