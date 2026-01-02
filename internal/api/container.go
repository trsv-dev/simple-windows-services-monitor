package api

import (
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
	"github.com/trsv-dev/simple-windows-services-monitor/internal/netutils"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/service_control"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/storage"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/worker"
)

// HandlersContainer Контейнер со всеми handlers приложения (и их зависимостями).
type HandlersContainer struct {
	ServerHandler        *server_handler.ServerHandler
	ServiceHandler       *service_handler.ServiceHandler
	ControlHandler       *control_handler.ControlHandler
	RegistrationHandler  *registration_handler.RegistrationHandler
	AuthorizationHandler *authorization_handler.AuthorizationHandler
	HealthHandler        *health_handler.HealthHandler
	AppHandler           *app_handler.AppHandler
}

// NewHandlersContainer Конструктор контейнера с зависимостями.
func NewHandlersContainer(storage storage.Storage, srvConfig *config.Config, broadcaster broadcast.Broadcaster, tokenBuilder auth.TokenBuilder) *HandlersContainer {
	clientFactory := service_control.NewWinRMClientFactory()
	netChecker := netutils.NewNetworkChecker()
	fingerprinter := service_control.NewWinRMFingerprinter(clientFactory, netChecker)
	servicesStatusesWorker := worker.NewServicesStatusesWorker(clientFactory)

	serverHandler := server_handler.NewServerHandler(storage, fingerprinter)
	serviceHandler := service_handler.NewServiceHandler(storage, clientFactory, netChecker, servicesStatusesWorker)
	controlHandler := control_handler.NewControlHandler(storage, clientFactory, netChecker)
	registrationHandler := registration_handler.NewRegistrationHandler(storage, tokenBuilder,
		srvConfig.JWTSecretKey, srvConfig.RegistrationKey, srvConfig.OpenRegistration)
	authorizationHandler := authorization_handler.NewAuthorizationHandler(storage, tokenBuilder, srvConfig.JWTSecretKey)
	healthHandler := health_handler.NewHealthHandler(storage)
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
