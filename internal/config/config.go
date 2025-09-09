package config

import (
	"flag"
	"os"
)

type Config struct {
	RunAddress   string
	DatabaseURI  string
	LogLevel     string
	JWTSecretKey string
	AESKey       string
}

// InitConfig Инициализация структуры, содержащей конфигурацию сервера, полученную из флагов или
// переменных окружения.
func InitConfig() *Config {
	config := &Config{}

	flag.StringVar(&config.RunAddress, "a", "127.0.0.1:8080", "HTTP server address and port")
	flag.StringVar(&config.DatabaseURI, "d", "", "Database URI (example: `postgres://username:password@localhost:5432/dbname?sslmode=disable`)")
	flag.StringVar(&config.LogLevel, "ll", "Debug", "Log level for logging (example: Debug, Info, Warn, Error)")
	flag.StringVar(&config.JWTSecretKey, "s", "", "Secret key used for signing and verifying JWT tokens (example: UIfuBqY1crEUgzIem9)")
	flag.StringVar(&config.AESKey, "k", "", "AES key for encrypting server passwords (hex-encoded, 32 bytes, for example: DjffxQxRnhvkB0CkxEiGbrFIoN8PTJc3TZqf/YNSVRI=)")
	flag.Parse()

	if value, ok := os.LookupEnv("RUN_ADDRESS"); ok {
		config.RunAddress = value
	}

	if value, ok := os.LookupEnv("DATABASE_URI"); ok {
		config.DatabaseURI = value
	}

	if value, ok := os.LookupEnv("LOG_LEVEL"); ok {
		config.LogLevel = value
	}

	if value, ok := os.LookupEnv("JWT_SECRET_KEY"); ok {
		config.JWTSecretKey = value
	}

	if value, ok := os.LookupEnv("AES_KEY"); ok {
		config.AESKey = value
	}

	return config
}
