package main

import (
    "fmt"
    "os"
	"time"

	"ferlab/configurations-auto-updater/cmd"
	"ferlab/configurations-auto-updater/configs"
	"ferlab/configurations-auto-updater/filesystem"

	"github.com/Ferlab-Ste-Justine/etcd-sdk/client"
	"github.com/Ferlab-Ste-Justine/etcd-sdk/keymodels"
)

func syncFilesystem() error {
	confs, err := configs.GetConfigs()
	if err != nil {
		return err
	}

	fsErr := filesystem.EnsureFilesystemDir(confs.FilesystemPath, filesystem.ConvertFileMode(confs.DirectoriesPermission))
	if fsErr != nil {
		return fsErr
	}

	connTimeout, _ := time.ParseDuration(confs.ConnectionTimeout)
	reqTimeout, _ := time.ParseDuration(confs.RequestTimeout)
	cli, cliErr := client.Connect(client.EtcdClientOptions{
		ClientCertPath:    confs.UserAuth.CertPath,
		ClientKeyPath:     confs.UserAuth.KeyPath,
		CaCertPath:        confs.CaCertPath,
		Username:          confs.UserAuth.Username,
		Password:		   confs.UserAuth.Password,
		EtcdEndpoints:     confs.EtcdEndpoints,
		ConnectionTimeout: connTimeout,
		RequestTimeout:    reqTimeout,
		Retries:           confs.RequestRetries,
	})

	if cliErr != nil {
		return cliErr	
	}
	defer cli.Client.Close()

	etcdKeys, revision, prefixErr := cli.GetPrefix(confs.EtcdKeyPrefix)
	if prefixErr != nil {
		return prefixErr
	}

	dirKeys, dirErr := filesystem.GetDirectoryContent(confs.FilesystemPath)
	if dirErr != nil {
		return dirErr
	}

	diff := keymodels.GetKeysDiff(etcdKeys, confs.EtcdKeyPrefix, dirKeys, confs.FilesystemPath)
	applyErr := filesystem.ApplyDiffToDirectory(confs.FilesystemPath, diff, filesystem.ConvertFileMode(confs.FilesPermission), filesystem.ConvertFileMode(confs.DirectoriesPermission))
	if applyErr != nil {
		return applyErr
	}

	if len(confs.NotificationCommand) > 0 {
		cmdErr := cmd.ExecCommand(confs.NotificationCommand, confs.NotificationCommandRetries)
		if cmdErr != nil {
			return cmdErr
		}
	}

	changeChan := cli.WatchPrefixChanges(confs.EtcdKeyPrefix, revision + 1)
	for res := range changeChan {
		if res.Error != nil {
			return res.Error
		}

		applyErr := filesystem.ApplyDiffToDirectory(confs.FilesystemPath, res.Changes, filesystem.ConvertFileMode(confs.FilesPermission), filesystem.ConvertFileMode(confs.DirectoriesPermission))
		if applyErr != nil {
			return applyErr
		}

		if len(confs.NotificationCommand) > 0 {
			cmdErr := cmd.ExecCommand(confs.NotificationCommand, confs.NotificationCommandRetries)
			if cmdErr != nil {
				return cmdErr
			}
		}
	}

	return nil
}

func main() {
	err := syncFilesystem()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}