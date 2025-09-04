package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/errs"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/storage/postgres/utils"
	"golang.org/x/crypto/bcrypt"
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

// AddServer Добавление нового сервера в БД.
func (pg *PgStorage) AddServer(ctx context.Context, server models.Server, userID int) error {
	// хэшируем пароль для передачи в БД
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(server.Password), bcrypt.DefaultCost)
	if err != nil {
		logger.Log.Error("Не удалось хэшировать пароль", logger.String("err", err.Error()))
		return err
	}

	query := `INSERT INTO servers (user_id, name, address, username, password) VALUES ($1, $2, $3, $4, $5)`

	_, err = pg.DB.ExecContext(ctx, query, userID, server.Name, server.Address, server.Username, hashedPassword)

	var pgErr *pgconn.PgError
	if err != nil {
		switch {
		// если ошибка говорит о дубликате сервера - выходим из функции и возвращаем ошибку
		case errors.As(err, &pgErr) && pgErr.Code == "23505":
			return errs.NewErrDuplicatedServer(server.Address, err)
		default:
			return fmt.Errorf("ошибка при добавлении сервера: %w", err)
		}
	}

	return nil
}

func (pg *PgStorage) DelServer(ctx context.Context, srvAddr string, login string) error {
	query := `DELETE FROM servers 
       		  WHERE address = $1 AND user_id = (SELECT id FROM users WHERE login = $2)`

	Result, err := pg.DB.ExecContext(ctx, query, srvAddr, login)

	if err != nil {
		logger.Log.Error("Ошибка запроса", logger.String("err", err.Error()))
		return err
	}

	affectedRows, err := Result.RowsAffected()
	if err != nil {
		return fmt.Errorf("ошибка при выполнении запроса %w", err)
	}

	if affectedRows == 0 {
		return errs.NewErrServerNotFound(srvAddr, login)
	}

	return nil
}

func (pg *PgStorage) GetServer(ctx context.Context, srvAddr string) (models.Server, error) {
	//TODO implement me
	panic("implement me")
}

func (pg *PgStorage) ListServers(ctx context.Context) ([]models.Server, error) {
	//TODO implement me
	panic("implement me")
}

func (pg *PgStorage) AddService(ctx context.Context, srvAddr string, service models.Service) error {
	//TODO implement me
	panic("implement me")
}

func (pg *PgStorage) DelService(ctx context.Context, srvAddr string, service models.Service) error {
	//TODO implement me
	panic("implement me")
}

func (pg *PgStorage) GetService(ctx context.Context, srvAddr string) (models.Service, error) {
	//TODO implement me
	panic("implement me")
}

func (pg *PgStorage) ListServices(ctx context.Context, srvAddr string) ([]models.Service, error) {
	//TODO implement me
	panic("implement me")
}

// CreateUser Создание пользователя.
func (pg *PgStorage) CreateUser(ctx context.Context, user *models.User) error {
	// хэшируем пароль для передачи в БД
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		logger.Log.Error("Не удалось хэшировать пароль", logger.String("err", err.Error()))
		return err
	}

	query := `INSERT INTO users (login, password) VALUES ($1, $2)`

	_, err = pg.DB.Exec(query, user.Login, string(hashedPassword))
	var pgErr *pgconn.PgError
	if err != nil {
		switch {
		// если ошибка говорит о дубликате логина - выходим из функции и возвращаем ошибку
		case errors.As(err, &pgErr) && pgErr.Code == "23505":
			err = errs.NewErrLoginIsTaken(user.Login, err)
			logger.Log.Error("Пользователь существует", logger.String("err", err.Error()))
			return err
		default:
			logger.Log.Error("Ошибка при создании пользователя", logger.String("err", err.Error()))
			return fmt.Errorf("ошибка создания пользователя: %w", err)
		}
	}

	return nil
}

// GetUser Возвращает пользователя если он зарегистрирован.
func (pg *PgStorage) GetUser(ctx context.Context, user *models.User) (*models.User, error) {
	var userFromDB models.User

	query := `SELECT login, password FROM users WHERE login = $1`
	err := pg.DB.QueryRowContext(ctx, query, user.Login).Scan(&userFromDB.Login, &userFromDB.Password)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		logger.Log.Error("Пользователь с таким логином не найден", logger.String("err", err.Error()))
		return nil, errs.NewErrWrongLoginOrPassword(err)
	case err != nil:
		logger.Log.Error("Ошибка запроса", logger.String("err", err.Error()))
		return nil, err
	}

	if err = bcrypt.CompareHashAndPassword([]byte(userFromDB.Password), []byte(user.Password)); err != nil {
		logger.Log.Error("Неверная пара логин/пароль", logger.String("err", err.Error()))
		return nil, errs.NewErrWrongLoginOrPassword(err)
	}

	return &userFromDB, nil
}

// GetUserIDByLogin Возвращает userID пользователя если он зарегистрирован.
func (pg *PgStorage) GetUserIDByLogin(ctx context.Context, login string) (int, error) {
	var userID int

	query := `SELECT id FROM users WHERE login = $1`

	err := pg.DB.QueryRowContext(ctx, query, login).Scan(&userID)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		return 0, errs.NewErrLoginNotFound(err)
	case err != nil:
		logger.Log.Error("Ошибка запроса", logger.String("err", err.Error()))
		return 0, err
	}

	return userID, nil
}

func (pg *PgStorage) Close() error {
	err := pg.DB.Close()
	if err != nil {
		logger.Log.Error("Ошибка закрытия соединения с БД PostgreSQL", logger.String("err", err.Error()))
		return fmt.Errorf("ошибка закрытия БД PostgreSQL: %w", err)
	}

	return nil
}
