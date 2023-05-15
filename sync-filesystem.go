package main

import (
	"context"

	"github.com/Ferlab-Ste-Justine/configurations-auto-updater/cmd"
	"github.com/Ferlab-Ste-Justine/configurations-auto-updater/config"
	"github.com/Ferlab-Ste-Justine/configurations-auto-updater/filesystem"
	"github.com/Ferlab-Ste-Justine/configurations-auto-updater/logger"

	"github.com/Ferlab-Ste-Justine/etcd-sdk/client"
)

type SyncFsFeedback struct {
	Diff  client.KeyDiff
	Error error
}

func SyncFilesystem(conf config.Config, proceedChan <-chan struct{}, log logger.Logger) (context.CancelFunc, <-chan SyncFsFeedback) {
	feedbackChan := make(chan SyncFsFeedback)
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		defer func() {
			close(feedbackChan)
			cancel()
		}()

		fsErr := filesystem.EnsureFilesystemDir(conf.Filesystem.Path, filesystem.ConvertFileMode(conf.Filesystem.DirectoriesPermission))
		if fsErr != nil {
			feedbackChan <- SyncFsFeedback{Error: fsErr}
			return
		}

		cli, cliErr := client.Connect(ctx, client.EtcdClientOptions{
			ClientCertPath:    conf.EtcdClient.Auth.ClientCert,
			ClientKeyPath:     conf.EtcdClient.Auth.ClientKey,
			CaCertPath:        conf.EtcdClient.Auth.CaCert,
			Username:          conf.EtcdClient.Auth.Username,
			Password:          conf.EtcdClient.Auth.Password,
			EtcdEndpoints:     conf.EtcdClient.Endpoints,
			ConnectionTimeout: conf.EtcdClient.ConnectionTimeout,
			RequestTimeout:    conf.EtcdClient.RequestTimeout,
			RetryInterval:     conf.EtcdClient.RetryInterval,
			Retries:           conf.EtcdClient.Retries,
		})

		if cliErr != nil {
			feedbackChan <- SyncFsFeedback{Error: cliErr}
			return
		}
		defer cli.Client.Close()

		prefixInfo, prefixErr := cli.GetPrefix(conf.EtcdClient.Prefix)
		if prefixErr != nil {
			feedbackChan <- SyncFsFeedback{Error: prefixErr}
			return
		}

		dirKeys, dirErr := filesystem.GetDirectoryContent(conf.Filesystem.Path)
		if dirErr != nil {
			feedbackChan <- SyncFsFeedback{Error: dirErr}
			return
		}

		diff := client.GetKeyDiff(
			prefixInfo.Keys.ToValueMap(conf.EtcdClient.Prefix),
			dirKeys.ToValueMap(conf.Filesystem.SlashPath),
		)

		if !diff.IsEmpty() {
			feedbackChan <- SyncFsFeedback{Diff: diff}
			if proceedChan != nil {
				_, ok := <-proceedChan
				if !ok {
					return
				}
			}

			applyErr := filesystem.ApplyDiffToDirectory(conf.Filesystem.Path, diff, filesystem.ConvertFileMode(conf.Filesystem.FilesPermission), filesystem.ConvertFileMode(conf.Filesystem.DirectoriesPermission))
			if applyErr != nil {
				feedbackChan <- SyncFsFeedback{Error: applyErr}
				return
			}

			if len(conf.NotificationCommand) > 0 {
				cmdErr := cmd.ExecCommand(conf.NotificationCommand, conf.NotificationCommandRetries)
				if cmdErr != nil {
					feedbackChan <- SyncFsFeedback{Error: cmdErr}
					return
				}
			}
		}

		wOpts := client.WatchOptions{
			Revision:   prefixInfo.Revision + 1,
			IsPrefix:   true,
			TrimPrefix: true,
		}
		changeChan := cli.Watch(conf.EtcdClient.Prefix, wOpts)
		for res := range changeChan {
			if res.Error != nil {
				feedbackChan <- SyncFsFeedback{Error: res.Error}
				return
			}

			diff, diffErr := filesystem.WatchInfoToKeyDiffs(conf.Filesystem.Path, res.Changes)
			if diffErr != nil {
				feedbackChan <- SyncFsFeedback{Error: diffErr}
				return
			}

			feedbackChan <- SyncFsFeedback{Diff: diff}
			if proceedChan != nil {
				_, ok := <-proceedChan
				if !ok {
					return
				}
			}

			applyErr := filesystem.ApplyDiffToDirectory(conf.Filesystem.Path, diff, filesystem.ConvertFileMode(conf.Filesystem.FilesPermission), filesystem.ConvertFileMode(conf.Filesystem.DirectoriesPermission))
			if applyErr != nil {
				feedbackChan <- SyncFsFeedback{Error: applyErr}
				return
			}

			if len(conf.NotificationCommand) > 0 {
				cmdErr := cmd.ExecCommand(conf.NotificationCommand, conf.NotificationCommandRetries)
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
