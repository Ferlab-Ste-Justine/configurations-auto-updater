package main

import (
    "os"
	"os/signal"
	"syscall"

	"github.com/Ferlab-Ste-Justine/configurations-auto-updater/config"
	"github.com/Ferlab-Ste-Justine/configurations-auto-updater/logger"
)

func getEnv(key string, fallback string) string {
    if value, ok := os.LookupEnv(key); ok {
        return value
    }
    return fallback
}

func main() {
	log := logger.Logger{LogLevel: logger.ERROR}

	conf, err := config.GetConfig(getEnv("CONFS_AUTO_UPDATER_CONFIG_FILE", "config.yml"))
	if err != nil {
		log.Errorf(err.Error())
		os.Exit(1)
	}

	log.LogLevel = conf.GetLogLevel()

	var proceedCh chan struct{}
	var notifCli *GrpcNotifClient
	if len(conf.GrpcNotifications) > 0 {
		proceedCh = make(chan struct{})
		defer close(proceedCh)

		notifCli, err = ConnectToNotifEndpoints(conf.GrpcNotifications)
		if err != nil {
			log.Errorf(err.Error())
			os.Exit(1)
		}
	}

	syncCancel, syncFeedback := SyncFilesystem(conf, proceedCh, log)

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