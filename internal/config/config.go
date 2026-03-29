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
	KeycloakBaseURL       string
	SkipIssuerCheck       bool
	KeycloakRealmName     string
	KeycloakClientID      string
	AESKey                string
	WebInterface          bool
}

// InitConfig Инициализация структуры, содержащей конфигурацию сервера, полученную из флагов или
// переменных окружения.
func InitConfig() *Config {
	config := &Config{}

	//flag.StringVar(&config.RunAddress, "address", "127.0.0.1:8080", "HTTP server address and port")
	flag.StringVar(&config.RunAddress, "address", "127.0.0.1:9090", "HTTP server address and port")
	flag.StringVar(&config.DatabaseURI, "database", "", "Database URI (example: `postgres://username:password@localhost:5432/dbname?sslmode=disable`)")
	flag.StringVar(&config.WinRMPort, "winrm-port", "5985",
		"WinRM port (Default: 5985), alternative to https - 5986. Оr any custom port if your server uses a non-standard WinRM port ")
	flag.BoolVar(&config.WinRMUseHTTPS, "winrm-use-https", false, "Set the flag true for https connections. Default: false")
	flag.BoolVar(&config.WinRMInsecureForHTTPS, "ssl", false, "Set the flag true for skipping ssl verifications (useful for self-signed certificates)")
	flag.StringVar(&config.LogLevel, "log-level", "Debug", "Log level for logging (example: Debug, Info, Warn, Error). Default level: Debug")
	flag.StringVar(&config.LogOutput, "log-output", "./logs/swsm.log",
		"Log output destination: 'stdout' for console or relative path to logfile `./path/to/file.log` for log file. Default: './logs/swsm.log'")
	flag.StringVar(&config.AESKey, "aes-key", "",
		"AES key for encrypting server passwords (hex-encoded, 32 bytes, for example: DjffxQxRnhvkB0CkxEiGbrFIoN8PTJc3TZqf/YNSVRI=)")
	flag.StringVar(&config.KeycloakBaseURL, "keycloak-url-address", "http://127.0.0.1:8081",
		"Keycloak URL address (example: `http(s)://<host>:<port>`). Default: http://127.0.0.1:8081")
	flag.BoolVar(&config.SkipIssuerCheck, "skip-issuer-check", false, "Disables issuer verification for local development in Docker containers. Default: false")
	flag.StringVar(&config.KeycloakRealmName, "realm-name", "swsm", "Keycloak realm name (example: `swsm`). Default: swsm")
	flag.StringVar(&config.KeycloakClientID, "keycloak-client-id", "swsm", "Keycloak client ID. Must match client in Keycloak (example: `swsm`). Default: swsm")
	flag.BoolVar(&config.WebInterface, "web-interface", true,
		"Enable the web interface (SSE and HTTP frontend). Set to false to run the server as API-only without frontend and SSE support. Default: true")
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

	if value, ok := os.LookupEnv("KEYCLOAK_BASE_URL"); ok {
		config.KeycloakBaseURL = value
	}

	if value, ok := os.LookupEnv("KEYCLOAK_SKIP_ISSUER_CHECK"); ok {
		switch strings.ToLower(value) {
		case "1", "true", "yes", "on":
			config.SkipIssuerCheck = true
		case "0", "false", "no", "off":
			config.SkipIssuerCheck = false
		}
	}

	if value, ok := os.LookupEnv("KEYCLOAK_REALM_NAME"); ok {
		config.KeycloakRealmName = value
	}

	if value, ok := os.LookupEnv("KEYCLOAK_CLIENT_ID"); ok {
		config.KeycloakClientID = value
	}

	return config
}
