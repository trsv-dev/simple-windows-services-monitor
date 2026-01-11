package service_control

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/netutils"
)

// WinRMFingerprinter Реальная реализация получения fingerprint через WinRM.
type WinRMFingerprinter struct {
	clientFactory ClientFactory
	netChecker    netutils.Checker
	winrmPort     string
}

// NewWinRMFingerprinter Конструктор.
func NewWinRMFingerprinter(clientFactory ClientFactory, netChecker netutils.Checker, winrmPort string) *WinRMFingerprinter {
	return &WinRMFingerprinter{
		clientFactory: clientFactory,
		netChecker:    netChecker,
		winrmPort:     winrmPort,
	}
}

// GetFingerprint Получение fingerprint (MachineGuid) с Windows сервера.
func (wf *WinRMFingerprinter) GetFingerprint(ctx context.Context, address, username, password string) (uuid.UUID, error) {
	// проверяем доступность сервера, если недоступен - возвращаем ошибку
	if !wf.netChecker.CheckWinRM(ctx, address, wf.winrmPort, 0) {
		logger.Log.Warn(fmt.Sprintf("Сервер %s недоступен", address))
		return uuid.Nil, fmt.Errorf("сервер %s недоступен", address)
	}

	// создаём WinRM клиент для получения fingerprint (MachineGuid) с Windows сервера
	client, err := wf.clientFactory.CreateClient(address, username, password)

	if err != nil {
		logger.Log.Error("Ошибка создания WinRM клиента", logger.String("err", err.Error()))
		return uuid.Nil, fmt.Errorf("ошибка создания WinRM клиента: %w", err)
	}

	// команда для получения fingerprint сервера
	fingerprintCmd := `powershell -NoProfile -NonInteractive -ExecutionPolicy Bypass -Command "[Console]::Out.Write((Get-ItemProperty 'HKLM:\SOFTWARE\Microsoft\Cryptography').MachineGuid)"`

	// контекст для получения fingerprint
	fingerprintCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// получение fingerprint сервера
	fingerprintStr, err := client.RunCommand(fingerprintCtx, fingerprintCmd)
	if err != nil {
		return uuid.Nil, fmt.Errorf("не удалось получить уникальный идентификатор сервера: %w", err)
	}

	// конвертируем строковый fingerprint в uuid.UUID
	fingerprint, err := uuid.Parse(fingerprintStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("не удалось распарсить уникальный идентификатор сервера: %w", err)
	}

	return fingerprint, nil
}
