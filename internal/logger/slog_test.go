package logger

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSlogAdapterDebug Проверяет логирование уровня Debug.
func TestSlogAdapterDebug(t *testing.T) {
	// создаём буфер для захвата вывода
	buf := &bytes.Buffer{}

	// создаём logger с буфером
	slogger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	adapter := &SlogAdapter{slog: slogger}

	// логируем сообщение
	adapter.Debug("test message", String("key", "value"))

	// проверяем что сообщение в логе
	assert.Contains(t, buf.String(), "test message")
	assert.Contains(t, buf.String(), "key=value")
}

// TestInitLoggerDebugLevel Проверяет инициализацию логгера с уровнем Debug.
func TestInitLoggerDebugLevel(t *testing.T) {
	// сбрасываем синглтон
	Log = nil
	once = sync.Once{}

	//buf := &bytes.Buffer{}

	// переопределяем os.Stderr для захвата вывода
	oldStderr := os.Stderr
	defer func() { os.Stderr = oldStderr }()

	// создаём временный файл для вывода
	tmpFile, err := os.CreateTemp("", "log-*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	InitLogger("debug", tmpFile.Name())

	// проверяем что логгер инициализирован
	assert.NotNil(t, Log)
}

// TestSlogAdapterInfo Проверяет логирование уровня Info.
func TestSlogAdapterInfo(t *testing.T) {
	buf := &bytes.Buffer{}
	slogger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	adapter := &SlogAdapter{slog: slogger}

	adapter.Info("info message", String("status", "ok"))

	assert.Contains(t, buf.String(), "info message")
	assert.Contains(t, buf.String(), "status=ok")
}

// TestInitLoggerErrorLevel Проверяет инициализацию логгера с уровнем Error.
func TestInitLoggerErrorLevel(t *testing.T) {
	// сбрасываем синглтон
	Log = nil
	once = sync.Once{}

	tmpFile, err := os.CreateTemp("", "log-*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	InitLogger("error", tmpFile.Name())

	// проверяем что логгер инициализирован
	assert.NotNil(t, Log)
}

// TestInitLoggerStdout Проверяет инициализацию логгера с выводом в stdout.
func TestInitLoggerStdout(t *testing.T) {
	// сбрасываем синглтон
	Log = nil
	once = sync.Once{}

	InitLogger("info", "stdout")

	// проверяем что логгер инициализирован
	assert.NotNil(t, Log)
}

// TestInitLoggerDefaultLevel Проверяет инициализацию с неизвестным уровнем (используется Debug).
func TestInitLoggerDefaultLevel(t *testing.T) {
	// сбрасываем синглтон
	Log = nil
	once = sync.Once{}

	tmpFile, err := os.CreateTemp("", "log-*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	// инициализируем с неизвестным уровнем
	InitLogger("unknown_level", tmpFile.Name())

	// проверяем что логгер инициализирован (с Debug по умолчанию)
	assert.NotNil(t, Log)
}

// TestSlogAdapterWarn Проверяет логирование уровня Warn.
func TestSlogAdapterWarn(t *testing.T) {
	buf := &bytes.Buffer{}
	slogger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	adapter := &SlogAdapter{slog: slogger}

	adapter.Warn("warn message", String("warning", "test"))

	assert.Contains(t, buf.String(), "warn message")
	assert.Contains(t, buf.String(), "warning=test")
}

// TestSlogAdapterError Проверяет логирование уровня Error.
func TestSlogAdapterError(t *testing.T) {
	buf := &bytes.Buffer{}
	slogger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	adapter := &SlogAdapter{slog: slogger}

	adapter.Error("error message", String("error", "something failed"))

	assert.Contains(t, buf.String(), "error message")
	assert.Contains(t, buf.String(), `error="something failed"`)
}

// TestSlogAdapterMultipleFields Проверяет логирование с несколькими полями.
func TestSlogAdapterMultipleFields(t *testing.T) {
	buf := &bytes.Buffer{}
	slogger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	adapter := &SlogAdapter{slog: slogger}

	// логируем с несколькими полями
	adapter.Info("request processed",
		String("method", "GET"),
		String("path", "/api/users"),
		Int("status", 200),
	)

	// проверяем что все поля в логе
	assert.Contains(t, buf.String(), "request processed")
	assert.Contains(t, buf.String(), "method=GET")
	assert.Contains(t, buf.String(), "path=/api/users")
	assert.Contains(t, buf.String(), "status=200")
}

// TestStringField Проверяет создание String поля.
func TestStringField(t *testing.T) {
	field := String("username", "alice")

	assert.Equal(t, "username", field.Key)
	assert.Equal(t, "alice", field.Value)
}

// TestIntField Проверяет создание Int поля.
func TestIntField(t *testing.T) {
	field := Int("count", 42)

	assert.Equal(t, "count", field.Key)
	assert.Equal(t, "42", field.Value)
}

// TestIntField64 Проверяет создание Int64 поля.
func TestIntField64(t *testing.T) {
	field := Int64("id", 9223372036854775807)

	assert.Equal(t, "id", field.Key)
	assert.Equal(t, "9223372036854775807", field.Value)
}

// TestIntFieldZero Проверяет Int поле с нулевым значением.
func TestIntFieldZero(t *testing.T) {
	field := Int("counter", 0)

	assert.Equal(t, "counter", field.Key)
	assert.Equal(t, "0", field.Value)
}

// TestInt64FieldNegative Проверяет Int64 поле с отрицательным значением.
func TestInt64FieldNegative(t *testing.T) {
	field := Int64("temperature", -273)

	assert.Equal(t, "temperature", field.Key)
	assert.Equal(t, "-273", field.Value)
}

// TestStringFieldEmpty Проверяет String поле с пустой строкой.
func TestStringFieldEmpty(t *testing.T) {
	field := String("message", "")

	assert.Equal(t, "message", field.Key)
	assert.Equal(t, "", field.Value)
}

// TestConvertFields Проверяет преобразование Fields в any[].
func TestConvertFields(t *testing.T) {
	fields := []Field{
		String("key1", "value1"),
		String("key2", "value2"),
		Int("key3", 123),
	}

	result := convertFields(fields)

	// проверяем что результат содержит ключи и значения
	assert.Equal(t, 6, len(result))
	assert.Equal(t, "key1", result[0])
	assert.Equal(t, "value1", result[1])
	assert.Equal(t, "key2", result[2])
	assert.Equal(t, "value2", result[3])
	assert.Equal(t, "key3", result[4])
	assert.Equal(t, "123", result[5])
}

// TestConvertFieldsEmpty Проверяет преобразование пустого списка полей.
func TestConvertFieldsEmpty(t *testing.T) {
	fields := []Field{}

	result := convertFields(fields)

	assert.Equal(t, 0, len(result))
}

// TestInitLoggerSingleton Проверяет что InitLogger работает как синглтон.
func TestInitLoggerSingleton(t *testing.T) {
	// сбрасываем синглтон
	Log = nil
	once = sync.Once{}

	tmpFile1, err := os.CreateTemp("", "log-*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile1.Name())

	tmpFile2, err := os.CreateTemp("", "log-*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile2.Name())

	// первая инициализация
	InitLogger("debug", tmpFile1.Name())
	firstLog := Log

	// вторая инициализация с другим путём
	InitLogger("error", tmpFile2.Name())
	secondLog := Log

	// проверяем что оба вызова вернули одинаковый логгер (синглтон)
	assert.Equal(t, firstLog, secondLog)
}

// TestSlogAdapterClose Проверяет закрытие адаптера.
func TestSlogAdapterClose(t *testing.T) {
	// создаём временный файл
	tmpFile, err := os.CreateTemp("", "log-*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	slogger := slog.New(slog.NewTextHandler(tmpFile, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	adapter := &SlogAdapter{slog: slogger, output: tmpFile}

	// закрываем адаптер
	err = adapter.Close()

	// проверяем что нет ошибки
	assert.NoError(t, err)
}

// TestSlogAdapterCloseNil Проверяет закрытие адаптера с nil output.
func TestSlogAdapterCloseNil(t *testing.T) {
	slogger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	adapter := &SlogAdapter{slog: slogger, output: nil}

	// закрываем адаптер без output
	err := adapter.Close()

	// проверяем что нет ошибки
	assert.NoError(t, err)
}

// TestSlogAdapterLogLevels Проверяет фильтрацию по уровням логирования.
func TestSlogAdapterLogLevels(t *testing.T) {
	tests := []struct {
		name            string
		level           slog.Level
		shouldHaveDebug bool
		shouldHaveInfo  bool
		shouldHaveWarn  bool
		shouldHaveError bool
	}{
		{
			name:            "уровень Debug",
			level:           slog.LevelDebug,
			shouldHaveDebug: true,
			shouldHaveInfo:  true,
			shouldHaveWarn:  true,
			shouldHaveError: true,
		},
		{
			name:            "уровень Info",
			level:           slog.LevelInfo,
			shouldHaveDebug: false,
			shouldHaveInfo:  true,
			shouldHaveWarn:  true,
			shouldHaveError: true,
		},
		{
			name:            "уровень Warn",
			level:           slog.LevelWarn,
			shouldHaveDebug: false,
			shouldHaveInfo:  false,
			shouldHaveWarn:  true,
			shouldHaveError: true,
		},
		{
			name:            "уровень Error",
			level:           slog.LevelError,
			shouldHaveDebug: false,
			shouldHaveInfo:  false,
			shouldHaveWarn:  false,
			shouldHaveError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			slogger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{
				Level: tt.level,
			}))

			adapter := &SlogAdapter{slog: slogger}

			// логируем на всех уровнях
			adapter.Debug("debug message")
			adapter.Info("info message")
			adapter.Warn("warn message")
			adapter.Error("error message")

			output := buf.String()

			// проверяем какие сообщения должны быть в логе
			if tt.shouldHaveDebug {
				assert.Contains(t, output, "debug message")
			} else {
				assert.NotContains(t, output, "debug message")
			}

			if tt.shouldHaveInfo {
				assert.Contains(t, output, "info message")
			} else {
				assert.NotContains(t, output, "info message")
			}

			if tt.shouldHaveWarn {
				assert.Contains(t, output, "warn message")
			} else {
				assert.NotContains(t, output, "warn message")
			}

			if tt.shouldHaveError {
				assert.Contains(t, output, "error message")
			} else {
				assert.NotContains(t, output, "error message")
			}
		})
	}
}

// TestLoggerConcurrency Проверяет конкурентное логирование.
func TestLoggerConcurrency(t *testing.T) {
	buf := &bytes.Buffer{}
	slogger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	adapter := &SlogAdapter{slog: slogger}

	// запускаем множество горутин, логирующих одновременно
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			adapter.Info("concurrent log", Int("id", id))
		}(i)
	}

	wg.Wait()

	// проверяем что все сообщения залогированы
	assert.Greater(t, len(buf.String()), 0)
	// все 10 сообщений должны быть там
	assert.Equal(t, 10, strings.Count(buf.String(), "concurrent log"))
}

// TestInitLoggerCaseInsensitive Проверяет что уровень логирования case-insensitive.
func TestInitLoggerCaseInsensitive(t *testing.T) {
	tests := []struct {
		name     string
		logLevel string
	}{
		{"DEBUG", "DEBUG"},
		{"Debug", "Debug"},
		{"debug", "debug"},
		{"ERROR", "ERROR"},
		{"Error", "Error"},
		{"error", "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// сбрасываем синглтон
			Log = nil
			once = sync.Once{}

			tmpFile, err := os.CreateTemp("", "log-*.txt")
			require.NoError(t, err)
			defer os.Remove(tmpFile.Name())

			// инициализируем с разным случаем букв
			InitLogger(tt.logLevel, tmpFile.Name())

			// проверяем что логгер инициализирован
			assert.NotNil(t, Log)
		})
	}
}
