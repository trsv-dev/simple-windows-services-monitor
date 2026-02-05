package config

import (
	"flag"
	"os"
	"strings"
)

type Config struct {
	RunAddress            string
	DatabaseURI           string
	WinRMPort             string
	WinRMUseHTTPS         bool
	WinRMInsecureForHTTPS bool
	LogLevel              string
	LogOutput             string
	JWTSecretKey          string
	AESKey                string
	WebInterface          bool
	OpenRegistration      bool
	RegistrationKey       string
}

// InitConfig Инициализация структуры, содержащей конфигурацию сервера, полученную из флагов или
// переменных окружения.
func InitConfig() *Config {
	config := &Config{}

	flag.StringVar(&config.RunAddress, "a", "127.0.0.1:8080", "HTTP server address and port")
	flag.StringVar(&config.DatabaseURI, "d", "", "Database URI (example: `postgres://username:password@localhost:5432/dbname?sslmode=disable`)")
	flag.StringVar(&config.WinRMPort, "wp", "5985", "WinRM port (Default: 5985), alternative to https - 5986. Оr any custom port if your server uses a non-standard WinRM port ")
	flag.BoolVar(&config.WinRMUseHTTPS, "https", false, "Set the flag true for https connections. Default: false")
	flag.BoolVar(&config.WinRMInsecureForHTTPS, "ssl", false, "Set the flag true for skipping ssl verifications (useful for self-signed certificates)")
	flag.StringVar(&config.LogLevel, "ll", "Debug", "Log level for logging (example: Debug, Info, Warn, Error). Default level: Debug")
	flag.StringVar(&config.LogOutput, "lo", "./logs/swsm.log", "Log output destination: 'stdout' for console or relative path to logfile `./path/to/file.log` for log file. Default: './logs/swsm.log'")
	flag.StringVar(&config.JWTSecretKey, "s", "", "Secret key used for signing and verifying JWT tokens (example: UIfuBqY1crEUgzIem9)")
	flag.StringVar(&config.AESKey, "k", "", "AES key for encrypting server passwords (hex-encoded, 32 bytes, for example: DjffxQxRnhvkB0CkxEiGbrFIoN8PTJc3TZqf/YNSVRI=)")
	flag.BoolVar(&config.WebInterface, "w", true, "Enable the web interface (SSE and HTTP frontend). Set to false to run the server as API-only without frontend and SSE support. Default: true")
	flag.BoolVar(&config.OpenRegistration, "r", false, "Enable open registration. Default: false")
	flag.StringVar(&config.RegistrationKey, "rk", "your-secret-registration-key", "Secret key required for user registration")
	flag.Parse()

	if value, ok := os.LookupEnv("RUN_ADDRESS"); ok {
		config.RunAddress = value
	}

	if value, ok := os.LookupEnv("DATABASE_URI"); ok {
		config.DatabaseURI = value
	}

	if value, ok := os.LookupEnv("WINRM_PORT"); ok {
		config.WinRMPort = value
	}

	if value, ok := os.LookupEnv("WINRM_USE_HTTPS"); ok {
		switch strings.ToLower(value) {
		case "1", "true", "yes", "on":
			config.WinRMUseHTTPS = true
		case "0", "false", "no", "off":
			config.WinRMUseHTTPS = false
		}
	}

	if value, ok := os.LookupEnv("WINRM_INSECURE_FOR_HTTPS"); ok {
		switch strings.ToLower(value) {
		case "1", "true", "yes", "on":
			config.WinRMInsecureForHTTPS = true
		case "0", "false", "no", "off":
			config.WinRMInsecureForHTTPS = false
		}
	}

	if value, ok := os.LookupEnv("LOG_LEVEL"); ok {
		config.LogLevel = value
	}

	if value, ok := os.LookupEnv("LOG_OUTPUT"); ok {
		config.LogOutput = value
	}

	if value, ok := os.LookupEnv("JWT_SECRET_KEY"); ok {
		config.JWTSecretKey = value
	}

	if value, ok := os.LookupEnv("AES_KEY"); ok {
		config.AESKey = value
	}

	if value, ok := os.LookupEnv("WEB_INTERFACE"); ok {
		switch strings.ToLower(value) {
		case "1", "true", "yes", "on":
			config.WebInterface = true
		case "0", "false", "no", "off":
			config.WebInterface = false
		}
	}

	if value, ok := os.LookupEnv("OPEN_REGISTRATION"); ok {
		switch strings.ToLower(value) {
		case "1", "true", "yes", "on":
			config.OpenRegistration = true
		case "0", "false", "no", "off":
			config.OpenRegistration = false
		}
	}

	if value, ok := os.LookupEnv("REGISTRATION_KEY"); ok {
		config.RegistrationKey = value
	}

	return config
}
