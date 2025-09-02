package logger

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
)

// SlogAdapter Адаптер для логгера slog.
type SlogAdapter struct {
	slog *slog.Logger
}

func (s *SlogAdapter) Debug(msg string, fields ...Field) {
	s.slog.Debug(msg, convertFields(fields)...)
}

func (s *SlogAdapter) Info(msg string, fields ...Field) {
	s.slog.Info(msg, convertFields(fields)...)
}

func (s *SlogAdapter) Error(msg string, fields ...Field) {
	s.slog.Error(msg, convertFields(fields)...)
}

func (s *SlogAdapter) Warn(msg string, fields ...Field) {
	s.slog.Warn(msg, convertFields(fields)...)
}

func String(key string, val string) Field {
	return Field{
		Key:   key,
		Value: val,
	}
}

func Int(key string, val int) Field {
	return Field{
		Key:   key,
		Value: strconv.Itoa(val),
	}
}

// Конвертация Fields в any[].
func convertFields(fields []Field) []any {
	args := make([]any, 0, len(fields)*2)
	for _, f := range fields {
		args = append(args, f.Key, f.Value)
	}
	return args
}

var (
	Log  Logger
	once sync.Once
)

// InitLogger Синглтон инициализации логгера.
func InitLogger(logLevel string) {

	var programLevel slog.Level

	switch strings.ToLower(logLevel) {
	case "debug":
		programLevel = slog.LevelDebug
	case "info":
		programLevel = slog.LevelInfo
	case "warn":
		programLevel = slog.LevelWarn
	case "error":
		programLevel = slog.LevelError
	default:
		programLevel = slog.LevelDebug
	}

	once.Do(func() {
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: programLevel}))
		Log = &SlogAdapter{slog: logger}
	})
}
