package logger

import (
	"fmt"
	"log"
	"os"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

var (
	defaultLogger = log.New(os.Stdout, "", log.LstdFlags)
)

func SetOutput(w *os.File) {
	defaultLogger.SetOutput(w)
}

func SetFlags(flag int) {
	defaultLogger.SetFlags(flag)
}

func formatMessage(level LogLevel, format string, args ...interface{}) string {
	levelStr := ""
	switch level {
	case DEBUG:
		levelStr = "DEBUG"
	case INFO:
		levelStr = "INFO"
	case WARN:
		levelStr = "WARN"
	case ERROR:
		levelStr = "ERROR"
	case FATAL:
		levelStr = "FATAL"
	}

	msg := fmt.Sprintf(format, args...)
	return fmt.Sprintf("[%s] [WIREPORT] %s", levelStr, msg)
}

func Debug(format string, args ...interface{}) {
	defaultLogger.Println(formatMessage(DEBUG, format, args...))
}

func Info(format string, args ...interface{}) {
	defaultLogger.Println(formatMessage(INFO, format, args...))
}

func Warn(format string, args ...interface{}) {
	defaultLogger.Println(formatMessage(WARN, format, args...))
}

func Error(format string, args ...interface{}) {
	defaultLogger.Println(formatMessage(ERROR, format, args...))
}

func Fatal(format string, args ...interface{}) {
	defaultLogger.Fatal(formatMessage(FATAL, format, args...))
}

func Printf(format string, args ...interface{}) {
	defaultLogger.Printf(format, args...)
}

func Println(args ...interface{}) {
	defaultLogger.Println(args...)
}
