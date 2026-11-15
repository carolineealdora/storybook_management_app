package logrus

import (
	"io"
	"os"

	logruslib "github.com/sirupsen/logrus"

	"codebase/pkg/logger"
)

type Level string

const JSONFormatter = "json"

type Option struct {
	Level       Level
	LogFilePath string
	Formatter   string
	MaxSize     int
	MaxBackups  int
	MaxAge      int
	Compress    bool
}

type logrusLogger struct {
	log *logruslib.Logger
}

type logrusEntry struct {
	entry *logruslib.Entry
}

func New(opt *Option) (logger.Logger, error) {
	log := logruslib.New()

	if opt.Formatter == JSONFormatter {
		log.SetFormatter(&logruslib.JSONFormatter{})
	} else {
		log.SetFormatter(&logruslib.TextFormatter{})
	}

	level, err := logruslib.ParseLevel(string(opt.Level))
	if err != nil {
		level = logruslib.InfoLevel
	}
	log.SetLevel(level)

	writers := []io.Writer{os.Stdout}
	if opt.LogFilePath != "" {
		if err := os.MkdirAll(opt.LogFilePath, 0755); err == nil {
			f, err := os.OpenFile(opt.LogFilePath+"app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
			if err == nil {
				writers = append(writers, f)
			}
		}
	}
	if len(writers) > 1 {
		log.SetOutput(io.MultiWriter(writers...))
	}

	return &logrusLogger{log: log}, nil
}

func (l *logrusLogger) WithFields(fields map[string]interface{}) logger.Logger {
	return &logrusEntry{entry: l.log.WithFields(logruslib.Fields(fields))}
}

func (l *logrusLogger) Info(args ...interface{})                  { l.log.Info(args...) }
func (l *logrusLogger) Error(args ...interface{})                 { l.log.Error(args...) }
func (l *logrusLogger) Debug(args ...interface{})                 { l.log.Debug(args...) }
func (l *logrusLogger) Fatal(args ...interface{})                 { l.log.Fatal(args...) }
func (l *logrusLogger) Fatalf(format string, args ...interface{}) { l.log.Fatalf(format, args...) }
func (l *logrusLogger) Errorf(format string, v ...interface{})    { l.log.Errorf(format, v...) }
func (l *logrusLogger) Warnf(format string, v ...interface{})     { l.log.Warnf(format, v...) }
func (l *logrusLogger) Debugf(format string, v ...interface{})    { l.log.Debugf(format, v...) }

func (e *logrusEntry) WithFields(fields map[string]interface{}) logger.Logger {
	return &logrusEntry{entry: e.entry.WithFields(logruslib.Fields(fields))}
}

func (e *logrusEntry) Info(args ...interface{})                  { e.entry.Info(args...) }
func (e *logrusEntry) Error(args ...interface{})                 { e.entry.Error(args...) }
func (e *logrusEntry) Debug(args ...interface{})                 { e.entry.Debug(args...) }
func (e *logrusEntry) Fatal(args ...interface{})                 { e.entry.Fatal(args...) }
func (e *logrusEntry) Fatalf(format string, args ...interface{}) { e.entry.Fatalf(format, args...) }
func (e *logrusEntry) Errorf(format string, v ...interface{})    { e.entry.Errorf(format, v...) }
func (e *logrusEntry) Warnf(format string, v ...interface{})     { e.entry.Warnf(format, v...) }
func (e *logrusEntry) Debugf(format string, v ...interface{})    { e.entry.Debugf(format, v...) }
