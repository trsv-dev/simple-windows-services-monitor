package config

import "time"

// WinRMConfig Структура конфигурации для создания WinRM клиента.
type WinRMConfig struct {
	Port          string
	UseHTTPS      bool
	InsecureHTTPS bool
	Timeout       time.Duration
}

// NewWinRMConfig Конструктор, возвращающий конфиг с параметрами для создания WinRM клиента.
func NewWinRMConfig(srvConfig *Config, timeout time.Duration) *WinRMConfig {
	return &WinRMConfig{
		Port:          srvConfig.WinRMPort,
		UseHTTPS:      srvConfig.WinRMUseHTTPS,
		InsecureHTTPS: srvConfig.WinRMInsecureForHTTPS,
		Timeout:       timeout,
	}
}
