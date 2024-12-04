# About

This tool will keep the content of a filesystem directory synchronized with a key prefix in etcd servers and is ideal for keeping configurations files synchronized with an autoritative etcd source.

It is a more generalised evolution of the following project: https://github.com/Ferlab-Ste-Justine/coredns-auto-updater

While in theory Windows is supported, the tool has only been validated on filesystems following the Unix convention so far.

Note that the tool watches for changes in the etcd prefix range as opposed to poll for changes and thus, is pretty responsive.

# Restrictions

Also note that the tool expects to be managed by a service manager like systemd to restart on error. It was designed with the philosophy that for most categories of errors outside the program's control, the best solution is to simply fail fast and get restarted with a fresh context.

Also, note that the tool expects to talk to etcd in a secure manner over a tls connection with either certificate auth or username/password auth.

# Notifications Support

The tool supports notifications whenever a change is applied.

It supports the following cases:
- Running a command with arguments AFTER files are updated with a change (and retry a certain number of time on error if the command returns a non-zero code)
- Push a notification to remote grpc server(s) with the following api contract: https://github.com/Ferlab-Ste-Justine/etcd-sdk/blob/main/keypb/api.proto#L42 . The push occurs BEFORE the files are updated and the files are only updated if the push succeeds. Note that because pushes to later servers (if you push to several servers) or even file update may fail, the same notification may be pushed more than once (and the servers should react to it in an idempotent way). However, assuming that this tool is restarted properly on failure, then the servers are guaranteed to eventually receive all file updates.

# Usage

The behavior of the binary is configured with a configuration file (it tries to look for a **config.yml** file in its running directory, but alternatively, you can specify another path for the configuration file with the **CONFS_AUTO_UPDATER_CONFIG_FILE** environment variable).

The **config.yml** file is as follows:

```
filesystem:
  path: "Path on the filesystem that should be synchronized"
  files_permission: "Permission that should be given to generated files in Unix base 8 format"
  directories_permission: "Permission that should be given to generated directories in Unix base 8 format"
etcd_client:
  prefix: "Etcd key prefix that the tool will synchronize the directory with"
  endpoints:
    - "List containing entries wwith the format: <ip>:<port>"
  connection_timeout: "Timeout to connect to etcd in golang duration format"
  request_timeout: "Timeout for etcd requests in golang duration format"
  retry_interval: "Interval of time to wait between retries in golang duration format"
  retries: "Maximum number of retries to make before giving up"
  auth:
    ca_cert: "Path to the CA certificate that signed the etcd servers' certificates"
    client_cert: "Path to a client certificate. If non-empty,should be accompanied by client_key and password_auth should be empty"
    client_key: "Path to a client key"
    client_cert_key: An alternative to the **client_cert** and **client_key** argument where both values are concatenated in the same file (cert first). Useful in situations where you want to update both values atomically or otherwise deal with tooling that expect to output secret values in a single file.
    password_auth: "Path to a yaml file containing the 'username' and 'password' keys if password authentication is used. If not-empty, client_cert and client_key should be empty"
notification_command:
  - "Notification command and its arguments to run whenever there is an update"
notification_command_retries: "Maximum number of time to retry the notification command if it returns a non-zero code"
grpc_notifications:
  - enpoint: "Endpoint to push notifications on a server to in the following format:  <url>:<port>"
    filter: "An optional regexp filter to apply on all file names being pushed. The remote server will be notified only of changes on files that pass the regexp"
    trim_key_path: "If set to true, the path of file names will be trimed out and the remote server will only receives the base of the file names in its notifications"
    max_chunk_size: "Maximum size to send per message in bytes. If the combined size of the updated files is larger, it will be broken down in several messages. Note that this is a best effort 'guarantee' as the message size may still be larger if a single file exceeds this value"
    auth:
      ca_cert: "Path to CA certificate that will validate the server's certificate for mTLS"
      client_cert: "Path to client public certificate that will authentication to the server for mTLS"
      client_key: "Path to client private key that will authentication to the server for mTLS"
  ..
log_level: "Minimum criticality of logs level displayed. Can be: debug, info, warn, error. Defaults to info"
```