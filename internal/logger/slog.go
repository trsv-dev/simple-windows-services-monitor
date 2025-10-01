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
	slog   *slog.Logger
	output *os.File
}

// Close Если пишем лог в файл, этот метод позволяет его закрыть после работы
func (s *SlogAdapter) Close() error {
	if s.output != nil {
		return s.output.Close()
	}

	return nil
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

func Int64(key string, val int64) Field {
	return Field{
		Key:   key,
		Value: strconv.FormatInt(val, 10),
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
func InitLogger(logLevel string, output string) {

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

	switch output {
	case "stdout":
		once.Do(func() {
			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: programLevel}))
			Log = &SlogAdapter{slog: logger}
		})
	default:
		output, err := os.OpenFile(output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			os.Exit(1)
		}

		once.Do(func() {
			logger := slog.New(slog.NewTextHandler(output, &slog.HandlerOptions{Level: programLevel}))
			Log = &SlogAdapter{slog: logger}
		})
	}
}
