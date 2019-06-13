// Package log exports logging primitives that log to stderr
package log

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"
)

// Logger is the interface for logging messages.
type Logger interface {
	Print(v ...interface{})
	Println(v ...interface{})
	Printf(format string, v ...interface{})
	Fatal(v ...interface{})
	Fatalf(format string, v ...interface{})
}

// Level represents the level of logging.
type Level int

type logger struct {
	level Level
}

//levels of logging.
const (
	DebugLevel Level = iota
	InfoLevel
	ErrorLevel
	DisabledLevel
)

//The set of default loggers for each log level.
var (
	Debug = &logger{DebugLevel}
	Info  = &logger{InfoLevel}
	Error = &logger{ErrorLevel}
)

type globalState struct {
	currentLevel  Level
	defaultLogger Logger
}

var (
	mu    sync.RWMutex
	state = globalState{
		currentLevel:  InfoLevel,
		defaultLogger: newDefaultLogger(os.Stderr),
	}
)

func globals() globalState {
	mu.RLock()
	defer mu.RUnlock()
	return state
}

func newDefaultLogger(w io.Writer) Logger {
	return log.New(w, "", log.Ldate|log.Ltime|log.LUTC|log.Lmicroseconds)
}

//SetOutput set writer to default logger.
func SetOutput(w io.Writer) {
	mu.Lock()
	defer mu.Unlock()

	if w == nil {
		state.defaultLogger = nil
	} else {
		state.defaultLogger = newDefaultLogger(w)
	}
}

func toString(level Level) string {
	switch level {
	case InfoLevel:
		return "info"
	case DebugLevel:
		return "debug"
	case ErrorLevel:
		return "error"
	case DisabledLevel:
		return "disabled"
	}
	return "unknown"
}

func toLevel(level string) (Level, error) {
	switch level {
	case "info":
		return InfoLevel, nil
	case "debug":
		return DebugLevel, nil
	case "error":
		return ErrorLevel, nil
	case "disabled":
		return DisabledLevel, nil
	}
	return DisabledLevel, fmt.Errorf("invalid log level %q", level)
}

//GetLevel get current log level.
func GetLevel() string {
	g := globals()
	return toString(g.currentLevel)
}

//SetLevel sets given log level
func SetLevel(level string) error {
	l, err := toLevel(level)
	if err != nil {
		return err
	}
	mu.Lock()
	state.currentLevel = l
	mu.Unlock()
	return nil
}
func (l *logger) Printf(format string, v ...interface{}) {
	g := globals()

	if l.level < g.currentLevel {
		return
	}
	if g.defaultLogger != nil {
		g.defaultLogger.Printf(format, v...)
	}

}

func (l *logger) Print(v ...interface{}) {
	g := globals()

	if l.level < g.currentLevel {
		return // Don't log at lower levels.
	}
	if g.defaultLogger != nil {
		g.defaultLogger.Print(v...)
	}
}

func (l *logger) Println(v ...interface{}) {
	g := globals()

	if l.level < g.currentLevel {
		return
	}
	if g.defaultLogger != nil {
		g.defaultLogger.Println(v...)
	}
}

func (l *logger) Fatal(v ...interface{}) {
	g := globals()

	if g.defaultLogger != nil {
		g.defaultLogger.Fatal(v...)
	} else {
		log.Fatal(v...)
	}
}

func (l *logger) Fatalf(format string, v ...interface{}) {
	g := globals()

	if g.defaultLogger != nil {
		g.defaultLogger.Fatalf(format, v...)
	} else {
		log.Fatalf(format, v...)
	}
}

// Printf writes a formatted message to the log.
func Printf(format string, v ...interface{}) {
	Info.Printf(format, v...)
}

// Print writes a message to the log.
func Print(v ...interface{}) {
	Info.Print(v...)
}

// Println writes a line to the log.
func Println(v ...interface{}) {
	Info.Println(v...)
}

// Fatal writes a message to the log and aborts.
func Fatal(v ...interface{}) {
	Info.Fatal(v...)
}

// Fatalf writes a formatted message to the log and aborts.
func Fatalf(format string, v ...interface{}) {
	Info.Fatalf(format, v...)
}
