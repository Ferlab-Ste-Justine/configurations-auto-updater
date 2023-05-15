package logger

import "log"

const (
	DEBUG int64 = 0
	INFO        = 1
	WARN        = 2
	ERROR       = 3
)

type Logger struct {
	LogLevel int64
}

func (logger Logger) Debugf(format string, args ...interface{}) {
	if logger.LogLevel <= DEBUG {
		log.Printf(format+"\n", args...)
	}
}

func (logger Logger) Infof(format string, args ...interface{}) {
	if logger.LogLevel <= INFO {
		log.Printf(format+"\n", args...)
	}
}

func (logger Logger) Warnf(format string, args ...interface{}) {
	if logger.LogLevel <= WARN {
		log.Printf(format+"\n", args...)
	}
}

func (logger Logger) Errorf(format string, args ...interface{}) {
	log.Printf(format+"\n", args...)
}
