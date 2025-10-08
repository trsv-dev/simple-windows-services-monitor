package service_control

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/netutils"
)

// GetFingerprint Получение fingerprint (MachineGuid) с Windows сервера.
func GetFingerprint(ctx context.Context, address, username, password string) (uuid.UUID, error) {
	// проверяем доступность сервера, если недоступен - возвращаем ошибку
	if !netutils.IsHostReachable(address, 5985, 0) {
		logger.Log.Warn(fmt.Sprintf("Сервер %s недоступен", address))
		return uuid.Nil, fmt.Errorf(fmt.Sprintf("Сервер %s недоступен", address))
	}

	// создаём WinRM клиент для получения fingerprint (MachineGuid) с Windows сервера
	client, err := NewWinRMClient(address, username, password)

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
		//logger.Log.Warn(fmt.Sprintf("Не удалось получить уникальный идентификатор `%s` от сервера `%s`",
		//	username, address), logger.String("err", err.Error()))

		return uuid.Nil, fmt.Errorf("не удалось получить уникальный идентификатор сервера: %w", err)
	}

	// конвертируем строковый fingerprint в uuid.UUID
	fingerprint, err := uuid.Parse(fingerprintStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("не удалось распарсить уникальный идентификатор сервера: %w", err)
	}

	return fingerprint, nil
}
