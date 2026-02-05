package di_containers

import (
	"time"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/api/app_handler"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/api/authorization_handler"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/api/control_handler"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/api/health_handler"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/api/registration_handler"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/api/server_handler"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/api/service_handler"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/auth"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/broadcast"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/config"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/health_storage"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/netutils"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/service_control"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/storage"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/worker"
)

// HandlersContainer Контейнер со всеми хендлерами приложения (и их зависимостями).
type HandlersContainer struct {
	ServerHandler        *server_handler.ServerHandler
	ServiceHandler       *service_handler.ServiceHandler
	ControlHandler       *control_handler.ControlHandler
	RegistrationHandler  *registration_handler.RegistrationHandler
	AuthorizationHandler *authorization_handler.AuthorizationHandler
	HealthHandler        *health_handler.HealthHandler
	AppHandler           *app_handler.AppHandler
}

// NewHandlersContainer Конструктор контейнера с зависимостями для хендлеров.
func NewHandlersContainer(storage storage.Storage, statusCache health_storage.StatusCacheStorage, srvConfig *config.Config, broadcaster broadcast.Broadcaster, tokenBuilder auth.TokenBuilder, netChecker netutils.Checker) *HandlersContainer {
	winRMConfig := config.NewWinRMConfig(srvConfig, 10*time.Second)
	clientFactory := service_control.NewWinRMClientFactory(winRMConfig)
	fingerprinter := service_control.NewWinRMFingerprinter(clientFactory, netChecker, winRMConfig.Port)
	serviceStatusesChecker := worker.NewServiceStatusesChecker(clientFactory)

	serverHandler := server_handler.NewServerHandler(storage, fingerprinter)
	serviceHandler := service_handler.NewServiceHandler(storage, clientFactory, netChecker, serviceStatusesChecker, winRMConfig.Port)
	controlHandler := control_handler.NewControlHandler(storage, clientFactory, netChecker, winRMConfig.Port)
	registrationHandler := registration_handler.NewRegistrationHandler(storage, tokenBuilder,
		srvConfig.JWTSecretKey, srvConfig.RegistrationKey, srvConfig.OpenRegistration)
	authorizationHandler := authorization_handler.NewAuthorizationHandler(storage, tokenBuilder, srvConfig.JWTSecretKey)
	//healthHandler := health_handler.NewHealthHandler(storage, statusCache, netChecker, srvConfig.WinRMPort)
	healthHandler := health_handler.NewHealthHandler(storage, statusCache, netChecker)
	appHandler := app_handler.NewAppHandler(srvConfig.JWTSecretKey, tokenBuilder, broadcaster)

	return &HandlersContainer{
		ServerHandler:        serverHandler,
		ServiceHandler:       serviceHandler,
		ControlHandler:       controlHandler,
		RegistrationHandler:  registrationHandler,
		AuthorizationHandler: authorizationHandler,
		HealthHandler:        healthHandler,
		AppHandler:           appHandler,
	}
}
