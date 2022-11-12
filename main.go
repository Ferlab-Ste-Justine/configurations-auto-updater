package main

import (
    "fmt"
    "os"

	"ferlab/configurations-auto-updater/cmd"
	"ferlab/configurations-auto-updater/configs"
	"ferlab/configurations-auto-updater/etcd"
	"ferlab/configurations-auto-updater/filesystem"
	"ferlab/configurations-auto-updater/model"

	clientv3 "go.etcd.io/etcd/client/v3"
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

	cli, connErr := etcd.Connect(
		confs.UserAuth.CertPath, 
		confs.UserAuth.KeyPath, 
		confs.UserAuth.Username,
		confs.UserAuth.Password,
		confs.CaCertPath, 
		confs.EtcdEndpoints, 
		confs.ConnectionTimeout, 
		confs.RequestTimeout, 
		confs.RequestRetries,
	)
	if connErr != nil {
		return connErr	
	}
	defer cli.Client.Close()

	etcdKeys, revision, rangeErr := cli.GetKeyRange(confs.EtcdKeyPrefix, clientv3.GetPrefixRangeEnd(confs.EtcdKeyPrefix))
	if rangeErr != nil {
		return rangeErr
	}

	dirKeys, dirErr := filesystem.GetDirectoryContent(confs.FilesystemPath)
	if dirErr != nil {
		return dirErr
	}

	diff := model.GetKeysDiff(etcdKeys, confs.EtcdKeyPrefix, dirKeys, confs.FilesystemPath)
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