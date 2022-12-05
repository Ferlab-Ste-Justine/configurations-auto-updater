terraform {
  required_providers {
    etcd = {
      source = "Ferlab-Ste-Justine/etcd"
      version = "= 0.7.0"
    }
  }
  required_version = ">= 1.0.0"
}