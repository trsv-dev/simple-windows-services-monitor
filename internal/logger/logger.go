package logger

type Field struct {
	Key   string
	Value string
}

// Logger Интерфейс для "быстрой" замены логгера.
// Достаточно реализовать дополнительный адаптер для нового логгера.
type Logger interface {
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
}
