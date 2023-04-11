package main

import (
	"context"
    "fmt"
    "os"

	"ferlab/configurations-auto-updater/cmd"
	"ferlab/configurations-auto-updater/configs"
	"ferlab/configurations-auto-updater/filesystem"

	"github.com/Ferlab-Ste-Justine/etcd-sdk/client"
)

func WatchInfoToKeyDiffs(w client.WatchInfo) client.KeysDiff {
	diff := client.KeysDiff{
		Upserts: make(map[string]string),
		Deletions: w.Deletions,
	}

	for key, val := range w.Upserts {
		diff.Upserts[key] = val.Value
	}

	return diff
}

func syncFilesystem() error {
	confs, err := configs.GetConfigs()
	if err != nil {
		return err
	}

	fsErr := filesystem.EnsureFilesystemDir(confs.Filesystem.Path, filesystem.ConvertFileMode(confs.Filesystem.DirectoriesPermission))
	if fsErr != nil {
		return fsErr
	}

	cli, cliErr := client.Connect(context.Background(), client.EtcdClientOptions{
		ClientCertPath:    confs.EtcdClient.Auth.ClientCert,
		ClientKeyPath:     confs.EtcdClient.Auth.ClientKey,
		CaCertPath:        confs.EtcdClient.Auth.CaCert,
		Username:          confs.EtcdClient.Auth.Username,
		Password:		   confs.EtcdClient.Auth.Password,
		EtcdEndpoints:     confs.EtcdClient.Endpoints,
		ConnectionTimeout: confs.EtcdClient.ConnectionTimeout,
		RequestTimeout:    confs.EtcdClient.RequestTimeout,
		RetryInterval:     confs.EtcdClient.RetryInterval,
		Retries:           confs.EtcdClient.Retries,
	})

	if cliErr != nil {
		return cliErr	
	}
	defer cli.Client.Close()

	etcdKeys, revision, prefixErr := cli.GetPrefix(confs.EtcdClient.Prefix)
	if prefixErr != nil {
		return prefixErr
	}

	dirKeys, dirErr := filesystem.GetDirectoryContent(confs.Filesystem.Path)
	if dirErr != nil {
		return dirErr
	}

	diff := client.GetKeysDiff(etcdKeys, confs.EtcdClient.Prefix, dirKeys, confs.Filesystem.Path)
	applyErr := filesystem.ApplyDiffToDirectory(confs.Filesystem.Path, diff, filesystem.ConvertFileMode(confs.Filesystem.FilesPermission), filesystem.ConvertFileMode(confs.Filesystem.DirectoriesPermission))
	if applyErr != nil {
		return applyErr
	}

	if len(confs.NotificationCommand) > 0 {
		cmdErr := cmd.ExecCommand(confs.NotificationCommand, confs.NotificationCommandRetries)
		if cmdErr != nil {
			return cmdErr
		}
	}

	changeChan := cli.WatchPrefixChanges(confs.EtcdClient.Prefix, revision + 1, true)
	for res := range changeChan {
		if res.Error != nil {
			return res.Error
		}

		applyErr := filesystem.ApplyDiffToDirectory(confs.Filesystem.Path, WatchInfoToKeyDiffs(res.Changes), filesystem.ConvertFileMode(confs.Filesystem.FilesPermission), filesystem.ConvertFileMode(confs.Filesystem.DirectoriesPermission))
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