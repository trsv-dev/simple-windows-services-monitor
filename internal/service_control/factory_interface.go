package service_control

//go:generate mockgen -destination=mocks/mock_client_factory.go -package=mocks . ClientFactory

// ClientFactory Интерфейс для создания новых Client-объектов.
type ClientFactory interface {
	CreateClient(address, username, password string) (Client, error)
}
