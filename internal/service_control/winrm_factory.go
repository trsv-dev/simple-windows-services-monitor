package service_control

import "github.com/trsv-dev/simple-windows-services-monitor/internal/config"

// WinRMClientFactory Структура WinRMClientFactory фабрики.
type WinRMClientFactory struct {
	winRMConfig *config.WinRMConfig
}

// NewWinRMClientFactory Конструктор фабрики.
func NewWinRMClientFactory(winrmConfig *config.WinRMConfig) *WinRMClientFactory {
	return &WinRMClientFactory{
		winRMConfig: winrmConfig,
	}
}

// CreateClient Фабрика WinRMClient. Создаёт WinRMClient для указанных в сигнатуре параметров.
func (f *WinRMClientFactory) CreateClient(address, username, password string) (Client, error) {
	return NewWinRMClient(address, f.winRMConfig.Port, username, password, f.winRMConfig.UseHTTPS, f.winRMConfig.InsecureHTTPS)
}
