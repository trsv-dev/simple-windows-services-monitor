package config

import (
	"flag"
	"os"
)

type Config struct {
	runAddress  string
	databaseURI string
	logLevel    string
}

// InitConfig Инициализация структуры, содержащей конфигурацию сервера, полученную из флагов или
// переменных окружения.
func InitConfig() *Config {
	config := &Config{}

	flag.StringVar(&config.runAddress, "a", "127.0.0.1:8080", "HTTP server address and port")
	flag.StringVar(&config.databaseURI, "d", "", "Database URI (example: `postgres://username:password@localhost:5432/dbname?sslmode=disable`)")
	flag.StringVar(&config.logLevel, "ll", "Debug", "Log level for logging (example: Debug, Info, Warn, Error, DPanic, Panic, Fatal)")
	flag.Parse()

	if value, ok := os.LookupEnv("RUN_ADDRESS"); ok {
		config.runAddress = value
	}

	if value, ok := os.LookupEnv("DATABASE_URI"); ok {
		config.databaseURI = value
	}

	if value, ok := os.LookupEnv("LOG_LEVEL"); ok {
		config.logLevel = value
	}

	return config
}
