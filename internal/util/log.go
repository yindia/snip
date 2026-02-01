package util

import (
	"io"
	"log"
	"os"
)

var debugEnabled bool

// SetDebug toggles debug logging.
func SetDebug(enabled bool) {
	debugEnabled = enabled
	if enabled {
		log.SetOutput(os.Stderr)
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	} else {
		log.SetOutput(io.Discard)
	}
}

// Debugf logs a formatted debug line.
func Debugf(format string, args ...interface{}) {
	if debugEnabled {
		log.Printf(format, args...)
	}
}
