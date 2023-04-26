package main

import (
	"context"

	"github.com/Ferlab-Ste-Justine/configurations-auto-updater/cmd"
	"github.com/Ferlab-Ste-Justine/configurations-auto-updater/configs"
	"github.com/Ferlab-Ste-Justine/configurations-auto-updater/filesystem"
	"github.com/Ferlab-Ste-Justine/configurations-auto-updater/logger"

	"github.com/Ferlab-Ste-Justine/etcd-sdk/client"
)

type SyncFsFeedback struct {
	Diff  client.KeyDiff
	Error error
}

func SyncFilesystem(confs configs.Configs, proceedChan <-chan struct{}, log logger.Logger) (context.CancelFunc, <-chan SyncFsFeedback) {
	feedbackChan := make(chan SyncFsFeedback)
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		defer func() {
			close(feedbackChan)
			cancel()
		}()

		fsErr := filesystem.EnsureFilesystemDir(confs.Filesystem.Path, filesystem.ConvertFileMode(confs.Filesystem.DirectoriesPermission))
		if fsErr != nil {
			feedbackChan <- SyncFsFeedback{Error: fsErr}
			return
		}

		cli, cliErr := client.Connect(ctx, client.EtcdClientOptions{
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
			feedbackChan <- SyncFsFeedback{Error: cliErr}
			return
		}
		defer cli.Client.Close()

		prefixInfo, prefixErr := cli.GetPrefix(confs.EtcdClient.Prefix)
		if prefixErr != nil {
			feedbackChan <- SyncFsFeedback{Error: prefixErr}
			return
		}

		dirKeys, dirErr := filesystem.GetDirectoryContent(confs.Filesystem.Path)
		if dirErr != nil {
			feedbackChan <- SyncFsFeedback{Error: dirErr}
			return
		}

		diff := client.GetKeyDiff(
			prefixInfo.Keys.ToValueMap(confs.EtcdClient.Prefix), 
			dirKeys.ToValueMap(confs.Filesystem.SlashPath),
		)

		if !diff.IsEmpty() {
			feedbackChan <- SyncFsFeedback{Diff: diff}
			if proceedChan != nil {
				_, ok := <- proceedChan
				if !ok {
					return
				}
			}

			applyErr := filesystem.ApplyDiffToDirectory(confs.Filesystem.Path, diff, filesystem.ConvertFileMode(confs.Filesystem.FilesPermission), filesystem.ConvertFileMode(confs.Filesystem.DirectoriesPermission))
			if applyErr != nil {
				feedbackChan <- SyncFsFeedback{Error: applyErr}
				return
			}
	
			if len(confs.NotificationCommand) > 0 {
				cmdErr := cmd.ExecCommand(confs.NotificationCommand, confs.NotificationCommandRetries)
				if cmdErr != nil {
					feedbackChan <- SyncFsFeedback{Error: cmdErr}
					return
				}
			}
		}

		wOpts := client.WatchOptions{
			Revision: prefixInfo.Revision + 1,
			IsPrefix: true,
			TrimPrefix: true,
		}
		changeChan := cli.Watch(confs.EtcdClient.Prefix, wOpts)
		for res := range changeChan {
			if res.Error != nil {
				feedbackChan <- SyncFsFeedback{Error: res.Error}
				return
			}

			diff, diffErr := filesystem.WatchInfoToKeyDiffs(confs.Filesystem.Path, res.Changes)
			if diffErr != nil {
				feedbackChan <- SyncFsFeedback{Error: diffErr}
				return
			}

			feedbackChan <- SyncFsFeedback{Diff: diff}
			if proceedChan != nil {
				_, ok := <- proceedChan
				if !ok {
					return
				}
			}

			applyErr := filesystem.ApplyDiffToDirectory(confs.Filesystem.Path, diff, filesystem.ConvertFileMode(confs.Filesystem.FilesPermission), filesystem.ConvertFileMode(confs.Filesystem.DirectoriesPermission))
			if applyErr != nil {
				feedbackChan <- SyncFsFeedback{Error: applyErr}
				return
			}

			if len(confs.NotificationCommand) > 0 {
				cmdErr := cmd.ExecCommand(confs.NotificationCommand, confs.NotificationCommandRetries)
				if cmdErr != nil {
					feedbackChan <- SyncFsFeedback{Error: cmdErr}
					return
				}
			}
		}
		log.Infof("[Etcd] Etcd watch stopped")
	}()

	return cancel, feedbackChan
}