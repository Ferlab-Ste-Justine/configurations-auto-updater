resource "etcd_synchronized_directory" "conf" {
    directory = "${path.module}/upload"
    key_prefix = "/upload/"
    source = "directory"
    recurrence = "onchange"
}