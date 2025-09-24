package utils

import (
	"strings"
)

const (
	Unknown                = 0
	ServiceRunning         = 1
	ServiceStopped         = 2
	ServiceStartPending    = 3
	ServiceStopPending     = 4
	ServiceContinuePending = 5
	ServicePausePending    = 6
	ServicePaused          = 7
)

// GetStatus Получение цифрового статуса службы по ее строковому описанию.
func GetStatus(query string) int {
	status := Unknown

	switch {
	case strings.Contains(query, "RUNNING"):
		status = ServiceRunning
	case strings.Contains(query, "STOPPED"):
		status = ServiceStopped
	case strings.Contains(query, "START_PENDING"):
		status = ServiceStartPending
	case strings.Contains(query, "STOP_PENDING"):
		status = ServiceStopPending
	case strings.Contains(query, "PAUSE_PENDING"):
		status = ServicePausePending
	case strings.Contains(query, "CONTINUE_PENDING"):
		status = ServiceContinuePending
	case strings.Contains(query, "PAUSED"):
		status = ServicePaused
	}

	return status
}

// GetStatusByINT Получение строкового описания службы по ее цифровому идентификатору.
func GetStatusByINT(status int) string {
	switch {
	case status == ServiceRunning:
		return "Запущена"
	case status == ServiceStopped:
		return "Остановлена"
	case status == ServiceStartPending:
		return "Запускается"
	case status == ServiceStopPending:
		return "Останавливается"
	case status == ServicePausePending:
		return "Приостанавливается"
	case status == ServiceContinuePending:
		return "Возобновляется"
	case status == ServicePaused:
		return "Приостановлена"
	default:
		return "Неизвестно"
	}
}
