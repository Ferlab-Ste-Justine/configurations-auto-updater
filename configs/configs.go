package configs

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
)

type UserAuth struct {
	CertPath      string
	KeyPath       string
	Username      string
	Password      string
}

type Configs struct {
	FilesystemPath             string
	FilesPermission            string
	DirectoriesPermission      string
	EtcdKeyPrefix              string
	EtcdEndpoints              string
	CaCertPath                 string
	UserAuth                   UserAuth
	ConnectionTimeout          uint64
	RequestTimeout             uint64
	RequestRetries             uint64
	NotificationCommand        []string
	NotificationCommandRetries uint64
}

func getEnv(key string, fallback string) string {
    if value, ok := os.LookupEnv(key); ok {
        return value
    }
    return fallback
}

func checkConfigsIntegrity(c Configs) error {
	if c.FilesystemPath == "" {
		return errors.New("Configuration error: Filesystem path cannot be empty")
	}

	if c.EtcdEndpoints == "" {
		return errors.New("Configuration error: Etcd endpoints cannot be empty")
	}

	if c.CaCertPath == "" {
		return errors.New("Configuration error: CA certificate path cannot be empty")
	}

	noValidAuth := (c.UserAuth.CertPath == "" || c.UserAuth.KeyPath == "") && (c.UserAuth.Username == "" || c.UserAuth.Password == "")
	ambiguousAuthMethod := (c.UserAuth.CertPath != "" || c.UserAuth.KeyPath != "") && (c.UserAuth.Username != "" || c.UserAuth.Password != "")

	if noValidAuth || ambiguousAuthMethod {
		return errors.New("Configuration error: Either user certificate AND key path should not be empty XOR user name AND password should not be empty")
	}

	if c.EtcdKeyPrefix == "" {
		return errors.New("Configuration error: Etcd key prefix cannot be empty")
	}

	parsedPermission, err := strconv.ParseInt(c.FilesPermission, 8, 32)
	if err != nil || parsedPermission < 0 || parsedPermission > 511 {
		return errors.New("Configuration error: Files permission must constitute a valid unix value for file permissions")
	}

	parsedPermission, err = strconv.ParseInt(c.DirectoriesPermission, 8, 32)
	if err != nil || parsedPermission < 0 || parsedPermission > 511 {
		return errors.New("Configuration error: Directories permission must constitute a valid unix value for file permissions")
	}

	return nil
}

func GetConfigs() (Configs, error) {
	var c Configs
	_, err := os.Stat("./configs.json")

	if err == nil {
		bs, err := ioutil.ReadFile("./configs.json")
		if err != nil {
			return Configs{}, errors.New(fmt.Sprintf("Error reading configuration file: %s", err.Error()))
		}
	
		err = json.Unmarshal(bs, &c)
		if err != nil {
			return Configs{}, errors.New(fmt.Sprintf("Error reading configuration file: %s", err.Error()))
		}

		if c.FilesPermission == "" {
			c.FilesPermission = "0660"
		}

		if c.DirectoriesPermission == "" {
			c.DirectoriesPermission = "0770"
		}
	} else if errors.Is(err, os.ErrNotExist) {
		var connectionTimeout, requestTimeout, requestRetries uint64
		var err error
		connectionTimeout, err = strconv.ParseUint(getEnv("CONNECTION_TIMEOUT", "0"), 10, 64)
		if err != nil {
			return Configs{}, errors.New("Error fetching configuration environment variables: CONNECTION_TIMEOUT must be an unsigned integer")
		}
		requestTimeout, err = strconv.ParseUint(getEnv("REQUEST_TIMEOUT", "0"), 10, 64)
		if err != nil {
			return Configs{}, errors.New("Error fetching configuration environment variables: REQUEST_TIMEOUT must be an unsigned integer")
		}
		requestRetries, err = strconv.ParseUint(getEnv("REQUEST_RETRIES", "0"), 10, 64)
		if err != nil {
			return Configs{}, errors.New("Error fetching configuration environment variables: REQUEST_RETRIES must be an unsigned integer")
		}
		c.ConnectionTimeout = connectionTimeout
		c.RequestTimeout = requestTimeout
		c.RequestRetries = requestRetries

		c.FilesPermission = getEnv("FILES_PERMISSION", "0660")
		c.DirectoriesPermission = getEnv("DIRECTORIES_PERMISSION", "0770")

		userAuth := UserAuth{
			CertPath: getEnv("USER_CERT_PATH", ""),
			KeyPath: getEnv("USER_KEY_PATH", ""),
			Username: getEnv("USER_NAME", ""),
			Password: getEnv("USER_PASSWORD", ""),
		}
		c.UserAuth = userAuth

		c.FilesystemPath = os.Getenv("FILESYSTEM_PATH")
		c.EtcdEndpoints = os.Getenv("ETCD_ENDPOINTS")
		c.CaCertPath = os.Getenv("CA_CERT_PATH")
		c.EtcdKeyPrefix = os.Getenv("ETCD_KEY_PREFIX")

		notificationCommand := getEnv("NOTIFICATION_COMMAND", "")
		c.NotificationCommand = []string{}
		if notificationCommand != "" {
			c.NotificationCommand = append(c.NotificationCommand, notificationCommand)
		}

		var notificationCommandRetries uint64
		notificationCommandRetries, err = strconv.ParseUint(getEnv("NOTIFICATION_COMMAND_RETRIES", "0"), 10, 64)
		if err != nil {
			return Configs{}, errors.New("Error fetching configuration environment variables: NOTIFICATION_COMMAND_RETRIES must be an unsigned integer")
		}
		c.NotificationCommandRetries = notificationCommandRetries
	} else {
		return Configs{}, errors.New(fmt.Sprintf("Error reading configuration file: %s", err.Error()))
	}

	absPath, absPathErr := filepath.Abs(c.FilesystemPath)
	if absPathErr != nil {
		return Configs{}, errors.New(fmt.Sprintf("Error conversion filesystem path to absolute path: %s", absPathErr.Error()))
	}

	c.FilesystemPath = absPath
	if c.FilesystemPath[len(c.FilesystemPath)-1:] != "/" {
		c.FilesystemPath = c.FilesystemPath + "/"
	}

	err = checkConfigsIntegrity(c)
	if err != nil {
		return Configs{}, err
	}

	return c, nil
}