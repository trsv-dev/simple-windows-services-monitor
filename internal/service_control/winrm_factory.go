package service_control

// WinRMClientFactory Структура WinRMClientFactory фабрики.
type WinRMClientFactory struct{}

// NewWinRMClientFactory Конструктор фабрики.
func NewWinRMClientFactory() *WinRMClientFactory {
	return &WinRMClientFactory{}
}

// CreateClient Фабрика WinRMClient. Создаёт WinRMClient для указанных в сигнатуре параметров.
func (f *WinRMClientFactory) CreateClient(address, username, password string) (Client, error) {
	return NewWinRMClient(address, username, password)
}
