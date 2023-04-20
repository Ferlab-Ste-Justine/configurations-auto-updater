package configs

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
	yaml "gopkg.in/yaml.v2"
)

type EtcdPasswordAuth struct {
	Username string
	Password string
}

type ConfigsEtcdAuth struct {
	CaCert       string `yaml:"ca_cert"`
	ClientCert   string `yaml:"client_cert"`
	ClientKey    string `yaml:"client_key"`
	PasswordAuth string `yaml:"password_auth"`
	Username     string `yaml:"-"`
	Password     string `yaml:"-"`
}

type ConfigsEtcd struct {
	Prefix            string
	Endpoints         []string
	ConnectionTimeout time.Duration        `yaml:"connection_timeout"`
	RequestTimeout    time.Duration        `yaml:"request_timeout"`
	RetryInterval     time.Duration        `yaml:"retry_interval"`
	Retries           uint64
	Auth              ConfigsEtcdAuth
}

type ConfigsFilesystem struct {
	Path                  string
	FilesPermission       string `yaml:"files_permission"`
	DirectoriesPermission string `yaml:"directories_permission"`
}

type ConfigsGrpcAuth struct {
	CaCert            string `yaml:"ca_cert"`
	ClientCert        string `yaml:"client_cert"`
	ClientKey         string `yaml:"client_key"`
}

type ConfigsGrpcNotifEndpoint struct {
	Endpoint    string
	Filter      string
	FilterRegex *regexp.Regexp `yaml:"-"`
}

type ConfigsGrpcNotifications struct {
	Endpoint          string
	Filter            string
	FilterRegex       *regexp.Regexp `yaml:"-"`
	ConnectionTimeout time.Duration               `yaml:"connection_timeout"`
	RequestTimeout    time.Duration               `yaml:"request_timeout"`
	RetryInterval     time.Duration               `yaml:"retry_interval"`
	Retries           uint64
	Auth              ConfigsGrpcAuth
}

type Configs struct {
	Filesystem                 ConfigsFilesystem
	EtcdClient                 ConfigsEtcd                `yaml:"etcd_client"`
	GrpcNotifications          []ConfigsGrpcNotifications `yaml:"grpc_notifications"`
	NotificationCommand        []string                   `yaml:"notification_command"`
	NotificationCommandRetries uint64                     `yaml:"notification_command_retries"`
}

func getEnv(key string, fallback string) string {
    if value, ok := os.LookupEnv(key); ok {
        return value
    }
    return fallback
}

func checkConfigsIntegrity(c Configs) error {
	if c.Filesystem.Path == "" {
		return errors.New("Configuration error: Filesystem path cannot be empty")
	}

	if len(c.EtcdClient.Endpoints) == 0 {
		return errors.New("Configuration error: Etcd endpoints cannot be empty")
	}

	if c.EtcdClient.Auth.CaCert == "" {
		return errors.New("Configuration error: CA certificate path cannot be empty")
	}

	noValidAuth := (c.EtcdClient.Auth.ClientCert == "" || c.EtcdClient.Auth.ClientKey == "") && (c.EtcdClient.Auth.Username == "" || c.EtcdClient.Auth.Password == "")
	ambiguousAuthMethod := (c.EtcdClient.Auth.ClientCert != "" || c.EtcdClient.Auth.ClientKey  != "") && (c.EtcdClient.Auth.Username != "" || c.EtcdClient.Auth.Password != "")

	if noValidAuth || ambiguousAuthMethod {
		return errors.New("Configuration error: Either user certificate AND key path should not be empty XOR user name AND password should not be empty")
	}

	if c.EtcdClient.Prefix == "" {
		return errors.New("Configuration error: Etcd key prefix cannot be empty")
	}

	parsedPermission, err := strconv.ParseInt(c.Filesystem.FilesPermission, 8, 32)
	if err != nil || parsedPermission < 0 || parsedPermission > 511 {
		return errors.New("Configuration error: Files permission must constitute a valid unix value for file permissions")
	}

	parsedPermission, err = strconv.ParseInt(c.Filesystem.DirectoriesPermission, 8, 32)
	if err != nil || parsedPermission < 0 || parsedPermission > 511 {
		return errors.New("Configuration error: Directories permission must constitute a valid unix value for file permissions")
	}

	return nil
}

func getPasswordAuth(path string) (EtcdPasswordAuth, error) {
	var a EtcdPasswordAuth

	b, err := ioutil.ReadFile(path)
	if err != nil {
		return a, errors.New(fmt.Sprintf("Error reading the password auth file: %s", err.Error()))
	}

	err = yaml.Unmarshal(b, &a)
	if err != nil {
		return a, errors.New(fmt.Sprintf("Error parsing the password auth file: %s", err.Error()))
	}

	return a, nil
}

func setGrpcEndpointsRegex(c *Configs) error {
	for idx, notif := range c.GrpcNotifications {
		if notif.Filter != "" {
			exp, expErr := regexp.Compile(notif.Filter)
			if expErr != nil {
				return expErr
			}
			notif.FilterRegex = exp
			c.GrpcNotifications[idx] = notif
		}
	}

	return nil
}

func GetConfigs() (Configs, error) {
	var c Configs
	conf_file_path := getEnv("CONFS_AUTO_UPDATER_CONFIG_FILE", "./configs.yml")

	bs, err := ioutil.ReadFile(conf_file_path)
	if err != nil {
		return Configs{}, errors.New(fmt.Sprintf("Error reading configuration file: %s", err.Error()))
	}

	err = yaml.Unmarshal(bs, &c)
	if err != nil {
		return Configs{}, errors.New(fmt.Sprintf("Error reading configuration file: %s", err.Error()))
	}

	if c.EtcdClient.Auth.PasswordAuth != "" {
		pAuth, pAuthErr := getPasswordAuth(c.EtcdClient.Auth.PasswordAuth)
		if pAuthErr != nil {
			return c, pAuthErr
		}
		c.EtcdClient.Auth.Username = pAuth.Username
		c.EtcdClient.Auth.Password = pAuth.Password
	}

	if c.Filesystem.FilesPermission == "" {
		c.Filesystem.FilesPermission = "0660"
	}

	if c.Filesystem.DirectoriesPermission == "" {
		c.Filesystem.DirectoriesPermission = "0770"
	}

	absPath, absPathErr := filepath.Abs(c.Filesystem.Path)
	if absPathErr != nil {
		return Configs{}, errors.New(fmt.Sprintf("Error conversion filesystem path to absolute path: %s", absPathErr.Error()))
	}

	c.Filesystem.Path = absPath
	if c.Filesystem.Path[len(c.Filesystem.Path)-1:] != "/" {
		c.Filesystem.Path = c.Filesystem.Path + "/"
	}

	expErr := setGrpcEndpointsRegex(&c)
	if expErr != nil {
		return c, expErr
	}

	err = checkConfigsIntegrity(c)
	if err != nil {
		return Configs{}, err
	}

	return c, nil
}