package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/service_control"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/service_control/utils"
)

// CheckServicesStatuses Получение с сервера статусов запрашиваемого слайса служб.
func CheckServicesStatuses(ctx context.Context, server *models.Server, services []*models.Service) ([]*models.Service, bool) {
	// создаём WinRM клиент
	client, err := service_control.NewWinRMClient(server.Address, server.Username, server.Password)

	if err != nil {
		logger.Log.Error("Ошибка создания WinRM клиента", logger.String("err", err.Error()))
		return nil, false
	}

	timeout := time.Duration(len(services)/10)*time.Second + 3*time.Second

	winRMCtx, winRMCtxCancel := context.WithTimeout(ctx, timeout)
	defer winRMCtxCancel()

	// формируем слайс имён служб для PowerShell
	serviceNames := make([]string, len(services))
	for i, svc := range services {
		// экранируем на всякий случай
		escaped := strings.ReplaceAll(svc.ServiceName, "'", "''")
		serviceNames[i] = fmt.Sprintf("'%s'", escaped)
		//serviceNames[i] = fmt.Sprintf(`'%s'`, svc.ServiceName)
	}

	// PowerShell-запрос для получения всех служб одним запросом
	psCmd := fmt.Sprintf(
		`powershell.exe -NoProfile -NonInteractive -Command "& {Get-Service -Name @(%s) -ErrorAction SilentlyContinue | Select-Object Name, @{Name='Status';Expression={$_.Status.ToString()}} | ConvertTo-Json -Compress}"`,
		strings.Join(serviceNames, ","),
	)

	// один PowerShell-запрос через WinRM для получения статусов нужных служб с сервера
	result, err := client.RunCommand(winRMCtx, psCmd)
	if err != nil {
		logger.Log.Error("Ошибка выполнения PowerShell-команды на получение статусов служб", logger.String("err", err.Error()))
		return nil, false
	}

	// парсим JSON результат и обновляем службы
	type powerShellService struct {
		Name string `json:"Name"`
		//Status int    `json:"Status"`
		Status string `json:"Status"`
	}

	var psService powerShellService
	var psServices []powerShellService

	// если вернулся один объект - PowerShell возвращает его не как массив, а как `{}`
	if strings.HasPrefix(strings.TrimSpace(result), "{") {
		if err := json.Unmarshal([]byte(result), &psService); err != nil {
			logger.Log.Error("Ошибка анмаршаллинга данных PowerShell-команды на получение статусов служб", logger.String("err", err.Error()))
			return nil, false
		}

		psServices = append(psServices, psService)

		// если вернулся массив объектов `[{}]`
	} else if strings.HasPrefix(strings.TrimSpace(result), "[") {
		if err := json.Unmarshal([]byte(result), &psServices); err != nil {
			logger.Log.Error("Ошибка анмаршаллинга данных PowerShell-команды на получение статусов служб", logger.String("err", err.Error()))
			return nil, false
		}
	}

	psMap := make(map[string]string, len(psServices))
	for _, ps := range psServices {
		psMap[ps.Name] = ps.Status
	}

	updates := make([]*models.Service, 0, len(psServices))

	// обновляем статусы в исходном слайсе
	updateTime := time.Now()
	for _, svc := range services {
		if status, ok := psMap[svc.ServiceName]; ok {
			// статус конвертируется для перевода на русский следующим образом:
			// `Running -> ServiceRunning (в int-представлении, 1) -> Работает`
			svc.Status = utils.GetStatusByINT(utils.GetStatus(status))
			svc.UpdatedAt = updateTime
			updates = append(updates, svc)
		}
	}

	return updates, true
}
