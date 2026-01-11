package service_control_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	netutisMocks "github.com/trsv-dev/simple-windows-services-monitor/internal/netutils/mocks"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/service_control"
	serviceControlMocks "github.com/trsv-dev/simple-windows-services-monitor/internal/service_control/mocks"
)

func init() {
	logger.InitLogger("error", "stdout")
}

// TestNewWinRMFingerprinter Проверяет конструктор.
func TestNewWinRMFingerprinter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutisMocks.NewMockChecker(ctrl)
	mockWinRMPort := "5985"

	fp := service_control.NewWinRMFingerprinter(mockFactory, mockChecker, mockWinRMPort)

	assert.NotNil(t, fp)
}

// TestGetFingerprintSuccess Проверяет успешное получение fingerprint.
func TestGetFingerprintSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)
	mockChecker := netutisMocks.NewMockChecker(ctrl)
	mockWinRMPort := "5985"

	ctx := context.Background()

	expectedGUID := "550e8400-e29b-41d4-a716-446655440000"

	mockChecker.EXPECT().
		CheckWinRM(ctx, "192.168.1.100", mockWinRMPort, time.Duration(0)).
		Return(true)

	mockFactory.EXPECT().
		CreateClient("192.168.1.100", "admin", "password").
		Return(mockClient, nil)

	mockClient.EXPECT().
		RunCommand(gomock.Any(), gomock.Any()).
		Return(expectedGUID, nil)

	fp := service_control.NewWinRMFingerprinter(mockFactory, mockChecker, mockWinRMPort)

	fingerprint, err := fp.GetFingerprint(ctx, "192.168.1.100", "admin", "password")

	assert.NoError(t, err)
	assert.Equal(t, expectedGUID, fingerprint.String())
}

// TestGetFingerprintHostUnreachable Проверяет недоступный хост.
func TestGetFingerprintHostUnreachable(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutisMocks.NewMockChecker(ctrl)
	mockWinRMPort := "5985"

	ctx := context.Background()

	mockChecker.EXPECT().
		CheckWinRM(ctx, "192.168.1.100", mockWinRMPort, time.Duration(0)).
		Return(false)

	fp := service_control.NewWinRMFingerprinter(mockFactory, mockChecker, mockWinRMPort)

	fingerprint, err := fp.GetFingerprint(ctx, "192.168.1.100", "admin", "password")

	assert.Error(t, err)
	assert.Equal(t, uuid.Nil, fingerprint)
	assert.Contains(t, err.Error(), "недоступен")
}

// TestGetFingerprintClientFactoryError Проверяет ошибку при создании клиента.
func TestGetFingerprintClientFactoryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutisMocks.NewMockChecker(ctrl)
	mockWinRMPort := "5985"

	ctx := context.Background()

	mockChecker.EXPECT().
		CheckWinRM(ctx, "192.168.1.100", mockWinRMPort, time.Duration(0)).
		Return(true)

	mockFactory.EXPECT().
		CreateClient("192.168.1.100", "admin", "password").
		Return(nil, errors.New("authentication failed"))

	fp := service_control.NewWinRMFingerprinter(mockFactory, mockChecker, mockWinRMPort)

	fingerprint, err := fp.GetFingerprint(ctx, "192.168.1.100", "admin", "password")

	assert.Error(t, err)
	assert.Equal(t, uuid.Nil, fingerprint)
	assert.Contains(t, err.Error(), "ошибка создания WinRM клиента")
}

// TestGetFingerprintRunCommandError Проверяет ошибку при выполнении команды.
func TestGetFingerprintRunCommandError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)
	mockChecker := netutisMocks.NewMockChecker(ctrl)
	mockWinRMPort := "5985"

	ctx := context.Background()

	mockChecker.EXPECT().
		CheckWinRM(ctx, "192.168.1.100", mockWinRMPort, time.Duration(0)).
		Return(true)

	mockFactory.EXPECT().
		CreateClient("192.168.1.100", "admin", "password").
		Return(mockClient, nil)

	mockClient.EXPECT().
		RunCommand(gomock.Any(), gomock.Any()).
		Return("", errors.New("PowerShell error"))

	fp := service_control.NewWinRMFingerprinter(mockFactory, mockChecker, mockWinRMPort)

	fingerprint, err := fp.GetFingerprint(ctx, "192.168.1.100", "admin", "password")

	assert.Error(t, err)
	assert.Equal(t, uuid.Nil, fingerprint)
	assert.Contains(t, err.Error(), "не удалось получить уникальный идентификатор")
}

// TestGetFingerprintInvalidUUID Проверяет невалидный GUID.
func TestGetFingerprintInvalidUUID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)
	mockChecker := netutisMocks.NewMockChecker(ctrl)
	mockWinRMPort := "5985"

	ctx := context.Background()

	invalidGUID := "not-a-valid-guid"

	mockChecker.EXPECT().
		CheckWinRM(ctx, "192.168.1.100", mockWinRMPort, time.Duration(0)).
		Return(true)

	mockFactory.EXPECT().
		CreateClient("192.168.1.100", "admin", "password").
		Return(mockClient, nil)

	mockClient.EXPECT().
		RunCommand(gomock.Any(), gomock.Any()).
		Return(invalidGUID, nil)

	fp := service_control.NewWinRMFingerprinter(mockFactory, mockChecker, mockWinRMPort)

	fingerprint, err := fp.GetFingerprint(ctx, "192.168.1.100", "admin", "password")

	assert.Error(t, err)
	assert.Equal(t, uuid.Nil, fingerprint)
	assert.Contains(t, err.Error(), "не удалось распарсить")
}

// TestGetFingerprintEmptyResponse Проверяет пустой ответ.
func TestGetFingerprintEmptyResponse(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)
	mockChecker := netutisMocks.NewMockChecker(ctrl)
	mockWinRMPort := "5985"

	ctx := context.Background()

	mockChecker.EXPECT().
		CheckWinRM(ctx, "192.168.1.100", mockWinRMPort, time.Duration(0)).
		Return(true)

	mockFactory.EXPECT().
		CreateClient("192.168.1.100", "admin", "password").
		Return(mockClient, nil)

	mockClient.EXPECT().
		RunCommand(gomock.Any(), gomock.Any()).
		Return("", nil)

	fp := service_control.NewWinRMFingerprinter(mockFactory, mockChecker, mockWinRMPort)

	fingerprint, err := fp.GetFingerprint(ctx, "192.168.1.100", "admin", "password")

	assert.Error(t, err)
	assert.Equal(t, uuid.Nil, fingerprint)
}

// TestGetFingerprintDifferentGUIDs Проверяет разные GUID форматы.
func TestGetFingerprintDifferentGUIDs(t *testing.T) {
	tests := []struct {
		name  string
		guid  string
		valid bool
	}{
		{"стандартный GUID", "550e8400-e29b-41d4-a716-446655440000", true},
		{"GUID в верхнем регистре", "550E8400-E29B-41D4-A716-446655440000", true},
		{"GUID смешанный", "550E8400-e29b-41D4-A716-446655440000", true},
		{"пустой GUID (валидный, но нулевой)", "00000000-0000-0000-0000-000000000000", true}, // ✅ валидный!
		{"короткая строка", "550e8400", false},
		{"случайный текст", "abcdefgh-ijkl-mnop-qrst-uvwxyzabcdef", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockFactory := serviceControlMocks.NewMockClientFactory(ctrl)
			mockClient := serviceControlMocks.NewMockClient(ctrl)
			mockChecker := netutisMocks.NewMockChecker(ctrl)
			mockWinRMPort := "5985"

			ctx := context.Background()

			mockChecker.EXPECT().
				CheckWinRM(ctx, "192.168.1.100", mockWinRMPort, gomock.Any()).
				Return(true)

			mockFactory.EXPECT().
				CreateClient("192.168.1.100", "admin", "password").
				Return(mockClient, nil)

			mockClient.EXPECT().
				RunCommand(gomock.Any(), gomock.Any()).
				Return(tt.guid, nil)

			fp := service_control.NewWinRMFingerprinter(mockFactory, mockChecker, mockWinRMPort)

			fingerprint, err := fp.GetFingerprint(ctx, "192.168.1.100", "admin", "password")

			if tt.valid {
				assert.NoError(t, err)
				// просто проверяем что парсилось без ошибки
				// пустой GUID = uuid.UUID{0,0,0,...} это OK
				parsedUUID, _ := uuid.Parse(tt.guid)
				assert.Equal(t, parsedUUID, fingerprint)
			} else {
				assert.Error(t, err)
				assert.Equal(t, uuid.Nil, fingerprint)
			}
		})
	}
}

// TestGetFingerprintContextTimeout Проверяет таймаут контекста.
func TestGetFingerprintContextTimeout(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)
	mockChecker := netutisMocks.NewMockChecker(ctrl)
	mockWinRMPort := "5985"

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	mockChecker.EXPECT().
		CheckWinRM(ctx, "192.168.1.100", mockWinRMPort, time.Duration(0)).
		Return(true)

	mockFactory.EXPECT().
		CreateClient("192.168.1.100", "admin", "password").
		Return(mockClient, nil)

	mockClient.EXPECT().
		RunCommand(gomock.Any(), gomock.Any()).
		Return("", context.DeadlineExceeded)

	fp := service_control.NewWinRMFingerprinter(mockFactory, mockChecker, mockWinRMPort)

	fingerprint, err := fp.GetFingerprint(ctx, "192.168.1.100", "admin", "password")

	assert.Error(t, err)
	assert.Equal(t, uuid.Nil, fingerprint)
}

// TestGetFingerprintMultipleServers Проверяет работу с разными серверами.
func TestGetFingerprintMultipleServers(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient1 := serviceControlMocks.NewMockClient(ctrl)
	mockClient2 := serviceControlMocks.NewMockClient(ctrl)
	mockChecker := netutisMocks.NewMockChecker(ctrl)
	mockWinRMPort := "5985"

	ctx := context.Background()

	guid1 := "550e8400-e29b-41d4-a716-446655440001"
	guid2 := "550e8400-e29b-41d4-a716-446655440002"

	// Первый сервер
	mockChecker.EXPECT().
		CheckWinRM(ctx, "192.168.1.100", mockWinRMPort, time.Duration(0)).
		Return(true)

	mockFactory.EXPECT().
		CreateClient("192.168.1.100", "admin", "password").
		Return(mockClient1, nil)

	mockClient1.EXPECT().
		RunCommand(gomock.Any(), gomock.Any()).
		Return(guid1, nil)

	// Второй сервер
	mockChecker.EXPECT().
		CheckWinRM(ctx, "192.168.1.101", mockWinRMPort, time.Duration(0)).
		Return(true)

	mockFactory.EXPECT().
		CreateClient("192.168.1.101", "admin", "password").
		Return(mockClient2, nil)

	mockClient2.EXPECT().
		RunCommand(gomock.Any(), gomock.Any()).
		Return(guid2, nil)

	fp := service_control.NewWinRMFingerprinter(mockFactory, mockChecker, mockWinRMPort)

	fingerprint1, err1 := fp.GetFingerprint(ctx, "192.168.1.100", "admin", "password")
	fingerprint2, err2 := fp.GetFingerprint(ctx, "192.168.1.101", "admin", "password")

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.NotEqual(t, fingerprint1, fingerprint2)
	assert.Equal(t, guid1, fingerprint1.String())
	assert.Equal(t, guid2, fingerprint2.String())
}

// TestGetFingerprintCredentials Проверяет что правильные креденшалы передаются.
func TestGetFingerprintCredentials(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)
	mockChecker := netutisMocks.NewMockChecker(ctrl)
	mockWinRMPort := "5985"

	expectedAddress := "192.168.1.100"
	expectedUsername := "domain\\admin"
	expectedPassword := "P@ssw0rd!"

	ctx := context.Background()

	mockChecker.EXPECT().
		CheckWinRM(ctx, expectedAddress, mockWinRMPort, time.Duration(0)).
		Return(true)

	mockFactory.EXPECT().
		CreateClient(expectedAddress, expectedUsername, expectedPassword).
		Return(mockClient, nil)

	mockClient.EXPECT().
		RunCommand(gomock.Any(), gomock.Any()).
		Return("550e8400-e29b-41d4-a716-446655440000", nil)

	fp := service_control.NewWinRMFingerprinter(mockFactory, mockChecker, mockWinRMPort)

	_, err := fp.GetFingerprint(ctx, expectedAddress, expectedUsername, expectedPassword)

	assert.NoError(t, err)
}
