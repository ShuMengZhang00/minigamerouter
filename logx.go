package minigamerouter

import "log"

const (
	DebugLevel = iota
	InfoLevel
	ErrorLevel
)

const logLevel = ErrorLevel

func shallLog(level int) bool {
	return logLevel >= level
}

func Errorf(format string, v ...any) {
	if shallLog(ErrorLevel) {
		log.Printf(format, v)
	}
}
