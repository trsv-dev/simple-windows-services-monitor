package logger

import (
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"

	"gopkg.in/natefinch/lumberjack.v2"
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

	once.Do(func() {
		var logOutput io.Writer

		switch output {
		case "stdout":
			logOutput = os.Stderr
		default:
			// используем lumberjack для ротации логов
			logOutput = &lumberjack.Logger{
				Filename:   output, // путь к файлу логов, например "swsm.log"
				MaxSize:    10,     // мегабайты
				MaxBackups: 10,     // сколько файлов хранить
				MaxAge:     30,     // дней
				Compress:   true,   // сжимать старые логи
			}
		}

		logger := slog.New(slog.NewTextHandler(logOutput, &slog.HandlerOptions{
			Level: programLevel,
		}))

		Log = &SlogAdapter{
			slog:   logger,
			output: nil,
		}
	})
}
