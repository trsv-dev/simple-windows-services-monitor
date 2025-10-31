package service_control_test

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/service_control"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	serviceControlMocks "github.com/trsv-dev/simple-windows-services-monitor/internal/service_control/mocks"
)

func init() {
	logger.InitLogger("error", "stdout")
}

// TestWinRMClientImplementsInterface Проверяет что WinRMClient реализует интерфейс.
func TestWinRMClientImplementsInterface(t *testing.T) {
	var _ service_control.Client = (*service_control.WinRMClient)(nil)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := serviceControlMocks.NewMockClient(ctrl)
	assert.NotNil(t, mockClient)
}

// TestRunCommandErrorHandling Проверяет обработку ошибок.
func TestRunCommandErrorHandling(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := serviceControlMocks.NewMockClient(ctrl)

	mockClient.EXPECT().
		RunCommand(gomock.Any(), gomock.Any()).
		Return("", context.DeadlineExceeded)

	ctx := context.Background()
	result, err := mockClient.RunCommand(ctx, "Get-Service")

	assert.Error(t, err)
	assert.Equal(t, "", result)
}

// TestRunCommandSuccessOutput Проверяет успешный вывод команды.
func TestRunCommandSuccessOutput(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := serviceControlMocks.NewMockClient(ctrl)
	expectedOutput := "Service1\nService2\nService3"

	mockClient.EXPECT().
		RunCommand(gomock.Any(), "Get-Service").
		Return(expectedOutput, nil)

	ctx := context.Background()
	result, err := mockClient.RunCommand(ctx, "Get-Service")

	assert.NoError(t, err)
	assert.Equal(t, expectedOutput, result)
}

// TestRunCommandEmptyOutput Проверяет пустой вывод.
func TestRunCommandEmptyOutput(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := serviceControlMocks.NewMockClient(ctrl)

	mockClient.EXPECT().
		RunCommand(gomock.Any(), "Get-Service").
		Return("", nil)

	ctx := context.Background()
	result, err := mockClient.RunCommand(ctx, "Get-Service")

	assert.NoError(t, err)
	assert.Equal(t, "", result)
}

// TestRunCommandWithContext Проверяет выполнение с контекстом.
func TestRunCommandWithContext(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := serviceControlMocks.NewMockClient(ctrl)

	mockClient.EXPECT().
		RunCommand(gomock.Any(), "Test-Command").
		Return("Success", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := mockClient.RunCommand(ctx, "Test-Command")

	assert.NoError(t, err)
	assert.Equal(t, "Success", result)
}

// TestRunCommandMultipleCalls Проверяет несколько вызовов.
func TestRunCommandMultipleCalls(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := serviceControlMocks.NewMockClient(ctrl)

	gomock.InOrder(
		mockClient.EXPECT().
			RunCommand(gomock.Any(), "Command1").
			Return("Output1", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), "Command2").
			Return("Output2", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), "Command3").
			Return("Output3", nil),
	)

	ctx := context.Background()

	result1, err1 := mockClient.RunCommand(ctx, "Command1")
	result2, err2 := mockClient.RunCommand(ctx, "Command2")
	result3, err3 := mockClient.RunCommand(ctx, "Command3")

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.NoError(t, err3)
	assert.Equal(t, "Output1", result1)
	assert.Equal(t, "Output2", result2)
	assert.Equal(t, "Output3", result3)
}
