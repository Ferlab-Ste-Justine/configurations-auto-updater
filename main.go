package main

import (
	"context"
    "os"
	"os/signal"
	"syscall"

	"github.com/Ferlab-Ste-Justine/configurations-auto-updater/cmd"
	"github.com/Ferlab-Ste-Justine/configurations-auto-updater/configs"
	"github.com/Ferlab-Ste-Justine/configurations-auto-updater/filesystem"
	"github.com/Ferlab-Ste-Justine/configurations-auto-updater/logger"

	//"google.golang.org/grpc"
	//"google.golang.org/grpc/credentials/insecure"
	"github.com/Ferlab-Ste-Justine/etcd-sdk/client"
)

func SyncFilesystem(confs configs.Configs, log logger.Logger) (context.CancelFunc, <-chan error) {
	errChan := make(chan error)
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		defer func() {
			close(errChan)
			cancel()
		}()

		fsErr := filesystem.EnsureFilesystemDir(confs.Filesystem.Path, filesystem.ConvertFileMode(confs.Filesystem.DirectoriesPermission))
		if fsErr != nil {
			errChan <- fsErr
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
			errChan <- cliErr
			return
		}
		defer cli.Client.Close()

		prefixInfo, prefixErr := cli.GetPrefix(confs.EtcdClient.Prefix)
		if prefixErr != nil {
			errChan <- prefixErr
			return
		}

		dirKeys, dirErr := filesystem.GetDirectoryContent(confs.Filesystem.Path)
		if dirErr != nil {
			errChan <- dirErr
			return
		}

		diff := client.GetKeyDiff(
			prefixInfo.Keys.ToValueMap(confs.EtcdClient.Prefix), 
			dirKeys.ToValueMap(confs.Filesystem.Path),
		)

		if !diff.IsEmpty() {
			applyErr := filesystem.ApplyDiffToDirectory(confs.Filesystem.Path, diff, filesystem.ConvertFileMode(confs.Filesystem.FilesPermission), filesystem.ConvertFileMode(confs.Filesystem.DirectoriesPermission))
			if applyErr != nil {
				errChan <- applyErr
				return
			}
	
			if len(confs.NotificationCommand) > 0 {
				cmdErr := cmd.ExecCommand(confs.NotificationCommand, confs.NotificationCommandRetries)
				if cmdErr != nil {
					errChan <- cmdErr
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
				errChan <- res.Error
				return
			}

			diff, diffErr := filesystem.WatchInfoToKeyDiffs(confs.Filesystem.Path, res.Changes)
			if diffErr != nil {
				errChan <- diffErr
				return
			}

			applyErr := filesystem.ApplyDiffToDirectory(confs.Filesystem.Path, diff, filesystem.ConvertFileMode(confs.Filesystem.FilesPermission), filesystem.ConvertFileMode(confs.Filesystem.DirectoriesPermission))
			if applyErr != nil {
				errChan <- applyErr
				return
			}

			if len(confs.NotificationCommand) > 0 {
				cmdErr := cmd.ExecCommand(confs.NotificationCommand, confs.NotificationCommandRetries)
				if cmdErr != nil {
					errChan <- cmdErr
					return
				}
			}
		}
		log.Infof("[Etcd] Etcd watch stopped")
	}()

	return cancel, errChan
}

func main() {
	log := logger.Logger{LogLevel: logger.INFO}

	confs, err := configs.GetConfigs()
	if err != nil {
		log.Errorf(err.Error())
		os.Exit(1)
	}

	syncCancel, syncErrChan := SyncFilesystem(confs, log)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-sigChan
		log.Warnf("[main] Caught signal %s. Terminating.", sig.String())
		syncCancel()
	}()

	syncErr := <- syncErrChan
	if syncErr != nil {
		log.Errorf(err.Error())
		os.Exit(1)
	}
}