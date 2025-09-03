package postgres

import (
	"database/sql"
	"fmt"

	//"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/storage/postgres/utils"
)

// PgStorage Структура хранилища в PostgreSQL, удовлетворяющая интерфейсу Storage.
type PgStorage struct {
	DB *sql.DB
}

// InitStorage Инициализация хранилища.
func InitStorage(DatabaseURI string) (*PgStorage, error) {
	// открываем соединение с БД
	pg, err := sql.Open("pgx", DatabaseURI)
	if err != nil {
		logger.Log.Error("Ошибка подключения к БД PostgreSQL", logger.String("err", err.Error()))
		return nil, fmt.Errorf("ошибка подключения к БД PostgreSQL: %w", err)
	}

	// проверяем, "живое" ли соединение
	if err = pg.Ping(); err != nil {
		logger.Log.Error("Ошибка при попытке подключения к БД PostgreSQL", logger.String("err", err.Error()))
		return nil, fmt.Errorf("нет связи с БД PostgreSQL: %w", err)
	}

	// применяем миграции
	err = utils.ApplyMigrations(DatabaseURI)
	if err != nil {
		logger.Log.Error("Ошибка применения миграций к БД PostgreSQL", logger.String("err", err.Error()))
		_ = pg.Close()
		return nil, fmt.Errorf("ошибка применения миграций к БД PostgreSQL: %w", err)
	}

	pgStorage := &PgStorage{DB: pg}

	logger.Log.Info("В качестве хранилища используется БД PostgreSQL")
	return pgStorage, nil
}

func (S PgStorage) AddServer(server models.Server) error {
	//TODO implement me
	panic("implement me")
}

func (S PgStorage) DelServer(srvAddr string) error {
	//TODO implement me
	panic("implement me")
}

func (S PgStorage) GetServer(srvAddr string) (models.Server, error) {
	//TODO implement me
	panic("implement me")
}

func (S PgStorage) ListServers() ([]models.Server, error) {
	//TODO implement me
	panic("implement me")
}

func (S PgStorage) AddService(srvAddr string, service models.Service) error {
	//TODO implement me
	panic("implement me")
}

func (S PgStorage) DelService(srvAddr string, service models.Service) error {
	//TODO implement me
	panic("implement me")
}

func (S PgStorage) GetService(srvAddr string) (models.Service, error) {
	//TODO implement me
	panic("implement me")
}

func (S PgStorage) ListServices(srvAddr string) ([]models.Service, error) {
	//TODO implement me
	panic("implement me")
}

func (S PgStorage) Close() error {
	err := S.DB.Close()
	if err != nil {
		logger.Log.Error("Ошибка закрытия соединения с БД PostgreSQL", logger.String("err", err.Error()))
		return fmt.Errorf("ошибка закрытия БД PostgreSQL: %w", err)
	}

	return nil
}
