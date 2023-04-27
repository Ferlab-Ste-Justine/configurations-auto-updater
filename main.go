package main

import (
    "os"
	"os/signal"
	"syscall"

	"github.com/Ferlab-Ste-Justine/configurations-auto-updater/configs"
	"github.com/Ferlab-Ste-Justine/configurations-auto-updater/logger"

	//"google.golang.org/grpc"
	//"google.golang.org/grpc/credentials/insecure"
)

func getEnv(key string, fallback string) string {
    if value, ok := os.LookupEnv(key); ok {
        return value
    }
    return fallback
}

func main() {
	log := logger.Logger{LogLevel: logger.ERROR}

	confs, err := configs.GetConfigs(getEnv("CONFS_AUTO_UPDATER_CONFIG_FILE", "configs.yml"))
	if err != nil {
		log.Errorf(err.Error())
		os.Exit(1)
	}

	log.LogLevel = confs.GetLogLevel()

	var proceedCh chan struct{}
	var notifCli *GrpcNotifClient
	if len(confs.GrpcNotifications) > 0 {
		proceedCh = make(chan struct{})
		defer close(proceedCh)

		notifCli, err = ConnectToNotifEndpoints(confs.GrpcNotifications)
		if err != nil {
			log.Errorf(err.Error())
			os.Exit(1)
		}
	}

	syncCancel, syncFeedback := SyncFilesystem(confs, proceedCh, log)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-sigChan
		log.Warnf("[main] Caught signal %s. Terminating.", sig.String())
		syncCancel()
	}()

	for feedback := range syncFeedback {
		if feedback.Error != nil {
			syncCancel()
			log.Errorf(feedback.Error.Error())
			os.Exit(1)
		}

		log.Infof(
			"[main] Received Update: %d inserts, %d updates and %d deletions", 
			len(feedback.Diff.Inserts),
			len(feedback.Diff.Updates),
			len(feedback.Diff.Deletions),
		)

		if notifCli != nil {
			sendErr := notifCli.Send(feedback.Diff)
			if sendErr != nil {
				syncCancel()
				close(sigChan)
				log.Errorf(sendErr.Error())
				os.Exit(1)
			}
			proceedCh <- struct{}{}
		}
	}
}