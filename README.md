# About

This tool will keep the content of a filesystem directory synchronized with a key prefix in etcd servers and is ideal for keeping configurations files synchronized with an autoritative etcd source.

It is a more generalised evolution of the following project: https://github.com/Ferlab-Ste-Justine/coredns-auto-updater

Note that presently, only filesystems following the Unix convention are supported.

Note that the tool watches for changes in the etcd prefix range as opposed to poll for changes and thus, is pretty responsive.

# Restrictions

Also note that the tool expects to be managed by a service manager like systemd to restart on error. The tool was designed with the philosophy that for most categories of erros outside the program's control, the best solution is to simply crash and get restarted with a fresh context.

Also, note that the tool expects to talk to etcd in a secure manner over a tls connection with either certificate auth or username/password auth.

# Usage

The tool is a binary that can be configured either with a configuration file or environment variables (it tries to look for a **configs.json** file in its running directory (alternatively, you can specify another path for the configuration file with the **CONFS_AUTO_UPDATER_CONFIG_FILE** environment variable) and if the file is absent, it fallsback to reading the expected environment variables).

The **configs.json** file is as follows:

```
{
    "FilesystemPath": "Path on the filesystem that should be synchronized",
    "FilesPermission": "Permission that should be given to generated files in Unix base 8 format",
    "DirectoriesPermission": "Permission that should be given to generated directories in Unix base 8 format",
    "EtcdKeyPrefix": "Etcd key prefix that the tool will synchronize the directory with",
    "EtcdEndpoints": "Comma separated list containing entries wwith the format: <ip>:<port>",
    "CaCertPath": "Path to the CA certificate that signed the etcd servers' certificates",
    "UserAuth": {
        "CertPath": "Path to a client certificate. If non-empty,should be accompanied by KeyPath and Username/Password should be empty",
        "KeyPath": "Path to a client key",
        "Username": "Client username. If non-empty, should be accompanied by by Password and CertPath/KeyPath should be empty",
        "Password": "Client password"
    },
    "ConnectionTimeout": "Connection timeout (number of seconds as integer)",
    "RequestTimeout": "Request timeout (number of seconds as integer)",
    "RequestRetries": "Number of times a failing request should be attempted before exiting on failure",
    "NotificationCommand": ["Command to execute after a successful startup or an update to provide a hook to notify another program", "and its arguments"],
    "NotificationCommandRetries": "Number of retries to give to the notification command before giving up"
}
```

The environment variables are:

- **CONNECTION_TIMEOUT**: Same parameter as **ConnectionTimeout** in **configs.json**
- **REQUEST_TIMEOUT**: Same parameter as **RequestTimeout** in **configs.json**
- **REQUEST_RETRIES**: Same parameter as **RequestRetries** in **configs.json**
- **USER_CERT_PATH**: Same parameter as **UserAuth.CertPath** in **configs.json**
- **USER_KEY_PATH**: Same parameter as **UserAuth.KeyPath** in **configs.json**
- **USER_NAME**: Same parameter as **UserAuth.Username** in **configs.json**
- **USER_PASSWORD**: Same parameter as **UserAuth.Password** in **configs.json**
- **FILESYSTEM_PATH**: Same parameter as **FilesystemPath** in **configs.json**
- **ETCD_ENDPOINTS**: Same parameter as **EtcdEndpoints** in **configs.json**
- **CA_CERT_PATH**: Same parameter as **CaCertPath** in **configs.json**
- **ETCD_KEY_PREFIX**: Same parameter as **EtcdKeyPrefix** in **configs.json**
- **NOTIFICATION_COMMAND**: Same parameter as **NotificationCommand** in **configs.json**, but doesn't accept additional arguments
- **NOTIFICATION_COMMAND_RETRIES**: Same parameter as **NotificationCommandRetries** in **configs.json**