package config

import (
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	yaml "gopkg.in/yaml.v2"

	"github.com/Ferlab-Ste-Justine/configurations-auto-updater/logger"
)

type EtcdPasswordAuth struct {
	Username string
	Password string
}

type ConfigEtcdAuth struct {
	CaCert       string `yaml:"ca_cert"`
	ClientCert   string `yaml:"client_cert"`
	ClientKey    string `yaml:"client_key"`
	PasswordAuth string `yaml:"password_auth"`
	Username     string `yaml:"-"`
	Password     string `yaml:"-"`
}

type ConfigEtcd struct {
	Prefix            string
	Endpoints         []string
	ConnectionTimeout time.Duration        `yaml:"connection_timeout"`
	RequestTimeout    time.Duration        `yaml:"request_timeout"`
	RetryInterval     time.Duration        `yaml:"retry_interval"`
	Retries           uint64
	Auth              ConfigEtcdAuth
}

type ConfigFilesystem struct {
	Path                  string
	SlashPath             string `yaml:"-"`
	FilesPermission       string `yaml:"files_permission"`
	DirectoriesPermission string `yaml:"directories_permission"`
}

type ConfigGrpcAuth struct {
	CaCert            string `yaml:"ca_cert"`
	ClientCert        string `yaml:"client_cert"`
	ClientKey         string `yaml:"client_key"`
}

type ConfigGrpcNotifications struct {
	Endpoint          string
	Filter            string
	FilterRegex       *regexp.Regexp  `yaml:"-"`
	TrimKeyPath       bool            `yaml:"trim_key_path"`
	MaxChunkSize      uint64          `yaml:"max_chunk_size"`
	ConnectionTimeout time.Duration   `yaml:"connection_timeout"`
	RequestTimeout    time.Duration   `yaml:"request_timeout"`            
	RetryInterval     time.Duration   `yaml:"retry_interval"`
	Retries           uint64
	Auth              ConfigGrpcAuth
}

type Config struct {
	Filesystem                 ConfigFilesystem
	EtcdClient                 ConfigEtcd                `yaml:"etcd_client"`
	GrpcNotifications          []ConfigGrpcNotifications `yaml:"grpc_notifications"`
	NotificationCommand        []string                   `yaml:"notification_command"`
	NotificationCommandRetries uint64                     `yaml:"notification_command_retries"`
    LogLevel                   string                     `yaml:"log_level"`
}

func (c *Config) GetLogLevel() int64 {
	logLevel := strings.ToLower(c.LogLevel)
	switch logLevel {
	case "error":
		return logger.ERROR
	case "warning":
		return logger.WARN
	case "debug":
		return logger.DEBUG
	default:
		return logger.INFO
	}
}

func checkConfigIntegrity(c Config) error {
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

func setGrpcEndpointsRegex(c *Config) error {
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

func GetConfig(confFilePath string) (Config, error) {
	var c Config

	bs, err := ioutil.ReadFile(confFilePath)
	if err != nil {
		return Config{}, errors.New(fmt.Sprintf("Error reading configuration file: %s", err.Error()))
	}

	err = yaml.Unmarshal(bs, &c)
	if err != nil {
		return Config{}, errors.New(fmt.Sprintf("Error reading configuration file: %s", err.Error()))
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
		return Config{}, errors.New(fmt.Sprintf("Error conversion filesystem path to absolute path: %s", absPathErr.Error()))
	}

	c.Filesystem.Path = absPath
	c.Filesystem.SlashPath = filepath.ToSlash(absPath)
	if c.Filesystem.SlashPath [len(c.Filesystem.SlashPath )-1:] != "/" {
		c.Filesystem.SlashPath  = c.Filesystem.SlashPath + "/"
	}

	expErr := setGrpcEndpointsRegex(&c)
	if expErr != nil {
		return c, expErr
	}

	err = checkConfigIntegrity(c)
	if err != nil {
		return Config{}, err
	}

	return c, nil
}