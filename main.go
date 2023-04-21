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

func main() {
	log := logger.Logger{LogLevel: logger.INFO}

	confs, err := configs.GetConfigs()
	if err != nil {
		log.Errorf(err.Error())
		os.Exit(1)
	}

	syncCancel, syncFeedback := SyncFilesystem(confs, nil, log)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-sigChan
		log.Warnf("[main] Caught signal %s. Terminating.", sig.String())
		syncCancel()
	}()

	for feedback := range syncFeedback {
		if feedback.Error != nil {
			log.Errorf(feedback.Error.Error())
			os.Exit(1)
		}

		log.Infof(
			"[main] Received Update: %d inserts, %d updates and %d deletions", 
			len(feedback.Diff.Inserts),
			len(feedback.Diff.Updates),
			len(feedback.Diff.Deletions),
		)
	}
}