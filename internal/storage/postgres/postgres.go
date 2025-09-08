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

	var newPassword []byte

	if server.Password != "" {
		// хэшируем пароль для передачи в БД
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(server.Password), bcrypt.DefaultCost)
		if err != nil {
			logger.Log.Error("Не удалось хэшировать пароль", logger.String("err", err.Error()))
			return err
		}

		newPassword = hashedPassword
	} else {
		newPassword = []byte(server.Password)
	}

	query := `INSERT INTO servers (user_id, name, address, username, password) VALUES ($1, $2, $3, $4, $5)`

	_, err := pg.DB.ExecContext(ctx, query, userID, server.Name, server.Address, server.Username, newPassword)

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

// EditServer Редактирование сервера, принадлежащего пользователю.
func (pg *PgStorage) EditServer(ctx context.Context, editedServer *models.Server, serverID int, login string) error {
	var password []byte

	// если был передан новый пароль - хэшируем его для передачи в БД
	if editedServer.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(editedServer.Password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("не удалось хэшировать пароль: %w", err)
		}

		password = hashedPassword
	} else {
		// Если пароль не был передан, получаем текущий из БД
		var currentPassword []byte
		getCurrentPasswordQuery := `SELECT password FROM servers WHERE id = $1 AND user_id = (SELECT id FROM users WHERE login = $2)`
		err := pg.DB.QueryRowContext(ctx, getCurrentPasswordQuery, serverID, login).Scan(&currentPassword)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errs.NewErrServerNotFound(serverID, login, err)
			}
			return fmt.Errorf("ошибка при получении текущего пароля: %w", err)
		}
		password = currentPassword
	}

	// обновляем сервер собранными данными
	updateQuery := `UPDATE servers SET name = $1, username = $2, address = $3, password = $4 
              WHERE id = $5 AND user_id = (SELECT id FROM users WHERE login = $6)`

	Result, err := pg.DB.ExecContext(ctx, updateQuery, editedServer.Name, editedServer.Username, editedServer.Address, password, serverID, login)
	if err != nil {
		return fmt.Errorf("ошибка при обновлении информации о сервере: %w", err)
	}

	affectedRows, err := Result.RowsAffected()
	if err != nil {
		return fmt.Errorf("ошибка при выполнении запроса %w", err)
	}

	if affectedRows == 0 {
		return errs.NewErrServerNotFound(serverID, login, fmt.Errorf("%w: затронутых строк %d", sql.ErrNoRows, affectedRows))
	}

	return nil
}

// DelServer Удаление сервера, принадлежащего пользователю.
func (pg *PgStorage) DelServer(ctx context.Context, serverID int, login string) error {
	query := `DELETE FROM servers 
       		  WHERE id = $1 AND user_id = (SELECT id FROM users WHERE login = $2)`

	Result, err := pg.DB.ExecContext(ctx, query, serverID, login)

	if err != nil {
		logger.Log.Error("Ошибка запроса", logger.String("err", err.Error()))
		return err
	}

	affectedRows, err := Result.RowsAffected()
	if err != nil {
		return fmt.Errorf("ошибка при выполнении запроса %w", err)
	}

	if affectedRows == 0 {
		return errs.NewErrServerNotFound(serverID, login, fmt.Errorf("%w: затронутых строк %d", sql.ErrNoRows, affectedRows))
	}

	return nil
}

// GetServer Получение информации о сервере, принадлежащем пользователю.
func (pg *PgStorage) GetServer(ctx context.Context, serverID int, login string) (*models.Server, error) {
	var server models.Server

	query := `SELECT name, address, username, created_at FROM servers 
              WHERE id = $1 
                AND user_id = (SELECT id FROM users WHERE login = $2)`

	err := pg.DB.QueryRowContext(ctx, query, serverID, login).Scan(&server.Name, &server.Address, &server.Username, &server.CreatedAt)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, errs.NewErrServerNotFound(serverID, login, err)
		default:
			return nil, err
		}
	}

	return &server, nil
}

// ListServers Отображение списка серверов, принадлежащих пользователю.
func (pg *PgStorage) ListServers(ctx context.Context, login string) ([]models.Server, error) {
	query := `SELECT name, address, username, created_at 
			  FROM servers WHERE user_id = (SELECT id FROM users WHERE login = $1)`

	rows, err := pg.DB.QueryContext(ctx, query, login)
	if err != nil {
		logger.Log.Error("Ошибка при получении списка серверов пользователя", logger.String("err", err.Error()))
		return nil, fmt.Errorf("ошибка при получении серверов пользователя: %w", err)
	}
	defer rows.Close()

	var servers []models.Server

	for rows.Next() {
		var server models.Server
		err = rows.Scan(&server.Name, &server.Address, &server.Username, &server.CreatedAt)
		if err != nil {
			logger.Log.Error("ошибка парсинга запроса на получение серверов пользователя", logger.String("err", err.Error()))
			return nil, err
		}

		servers = append(servers, server)
	}

	err = rows.Err()
	if err != nil {
		logger.Log.Error("Ошибка при обработке строк на получение информации о серверах пользователя", logger.String("err", err.Error()))
		return nil, err
	}

	return servers, nil
}

// AddService Добавление службы на сервер пользователя.
func (pg *PgStorage) AddService(ctx context.Context, serverID int, login string, service models.Service) error {
	query := `INSERT INTO services (server_id, displayed_name, service_name, status)
			  SELECT s.id, $2, $3, $4
			  FROM servers s
			  WHERE s.id = $1 AND user_id = (SELECT id FROM users WHERE login = $5)`

	Result, err := pg.DB.ExecContext(ctx, query, serverID, service.DisplayedName, service.ServiceName, service.Status, login)

	var pgErr *pgconn.PgError
	if err != nil {
		switch {
		// если ошибка говорит о дубликате службы - выходим из функции и возвращаем ошибку
		case errors.As(err, &pgErr) && pgErr.Code == "23505":
			return errs.NewErrDuplicatedService(service.ServiceName, err)
		default:
			return fmt.Errorf("ошибка при добавлении службы: %w", err)
		}
	}

	affectedRows, err := Result.RowsAffected()
	if err != nil {
		return fmt.Errorf("ошибка при выполнении запроса %w", err)
	}

	if affectedRows == 0 {
		return errs.NewErrServerNotFound(serverID, login, fmt.Errorf("%w: затронутых строк %d", sql.ErrNoRows, affectedRows))
	}

	return nil
}

// DelService Удаление службы с сервера пользователя.
func (pg *PgStorage) DelService(ctx context.Context, serverID int, serviceID int, login string) error {
	query := `DELETE FROM services 
              WHERE id = $1 
                AND server_id = $2
                AND server_id IN (
                	SELECT id FROM servers 
                	WHERE user_id = (SELECT id FROM users WHERE login = $3))`

	Result, err := pg.DB.ExecContext(ctx, query, serviceID, serverID, login)
	if err != nil {
		logger.Log.Error("Ошибка запроса", logger.String("err", err.Error()))
		return err
	}

	affectedRows, err := Result.RowsAffected()
	if err != nil {
		return fmt.Errorf("ошибка при выполнении запроса %w", err)
	}

	if affectedRows == 0 {
		return errs.NewErrServiceNotFound(login, serverID, serviceID, fmt.Errorf("%w: затронутых строк %d", sql.ErrNoRows, affectedRows))
	}

	return nil
}

// GetService Получение службы с сервера пользователя.
func (pg *PgStorage) GetService(ctx context.Context, serverID int, serviceID int, login string) (*models.Service, error) {
	query := `SELECT displayed_name, service_name, status 
			  FROM services 
			  WHERE id = $1 
			    AND server_id = $2 
			    AND server_id IN (
					SELECT id FROM servers 
					WHERE user_id = (SELECT id FROM users WHERE login = $3)
			    )`

	var service models.Service

	err := pg.DB.QueryRowContext(ctx, query, serviceID, serverID, login).Scan(&service.DisplayedName, &service.ServiceName, &service.Status)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, errs.NewErrServiceNotFound(login, serverID, serviceID, err)
		default:
			return nil, err
		}
	}

	return &service, nil
}

// ListServices Получение списка служб сервера, принадлежащего пользователю.
func (pg *PgStorage) ListServices(ctx context.Context, serverID int, login string) ([]models.Service, error) {

	// Сначала проверяем, принадлежит ли сервер пользователю
	var exists bool

	checkOwnershipQuery := `SELECT EXISTS(
							SELECT 1 FROM servers
							WHERE id = $1 AND user_id = (SELECT id FROM users WHERE login = $2)
							)`

	err := pg.DB.QueryRowContext(ctx, checkOwnershipQuery, serverID, login).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("ошибка при проверке владения сервером: %w", err)
	}

	if !exists {
		return nil, errs.NewErrServerNotFound(serverID, login, fmt.Errorf("сервер не найден или не принадлежит пользователю"))
	}

	// Теперь получаем службы
	query := `SELECT displayed_name, service_name, status 
			  FROM services 
			  WHERE server_id = $1`

	var services []models.Service

	rows, err := pg.DB.QueryContext(ctx, query, serverID)
	if err != nil {
		logger.Log.Error("Ошибка при получении списка служб сервера пользователя", logger.String("err", err.Error()))
		return nil, fmt.Errorf("ошибка при получении списка служб сервера пользователя: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var service models.Service

		err = rows.Scan(&service.DisplayedName, &service.ServiceName, &service.Status)
		if err != nil {
			logger.Log.Error("ошибка парсинга запроса на получение серверов пользователя", logger.String("err", err.Error()))
			return nil, err
		}

		services = append(services, service)
	}

	err = rows.Err()
	if err != nil {
		logger.Log.Error("Ошибка при обработке строк на получение информации о серверах пользователя", logger.String("err", err.Error()))
		return nil, err
	}

	return services, nil
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

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			logger.Log.Error("Пользователь с таким логином не найден", logger.String("err", err.Error()))
			return nil, errs.NewErrWrongLoginOrPassword(err)
		default:
			logger.Log.Error("Ошибка запроса", logger.String("err", err.Error()))
			return nil, err
		}
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

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return 0, errs.NewErrLoginNotFound(err)
		default:
			logger.Log.Error("Ошибка запроса", logger.String("err", err.Error()))
			return 0, err
		}
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
