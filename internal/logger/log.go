package logger

import "log/slog"

func Info(msg string, args ...interface{}){
	slog.Info(msg, args...)
}

func Warn(msg string, args ...interface{}){
	slog.Warn(msg, args...)
}

func Error(msg string, args ...interface{}){
	slog.Error(msg, args...)
}
