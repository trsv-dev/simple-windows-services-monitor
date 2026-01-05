package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
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
	DB     *sql.DB
	AESKey []byte
}

// InitStorage Инициализация хранилища.
func InitStorage(DatabaseURI string, AESKey []byte) (*PgStorage, error) {
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

	pgStorage := &PgStorage{DB: pg, AESKey: AESKey}

	logger.Log.Info("В качестве хранилища используется БД PostgreSQL")
	return pgStorage, nil
}

// AddServer Добавление нового сервера в БД.
func (pg *PgStorage) AddServer(ctx context.Context, server models.Server, userID int64) (*models.Server, error) {
	var newPassword string

	if server.Password != "" {
		// шифруем пароль для хранения в БД
		encryptedPassword, err := utils.EncryptAES([]byte(server.Password), pg.AESKey)
		if err != nil {
			logger.Log.Error("Не удалось зашифровать пароль", logger.String("err", err.Error()))
			return nil, err
		}
		newPassword = encryptedPassword
	} else {
		newPassword = server.Password
	}

	query := `INSERT INTO servers (user_id, name, address, username, password, fingerprint) VALUES ($1, $2, $3, $4, $5, $6)
			  RETURNING id, created_at`

	// обновляем значение id, created_at у уже переданной модели сервера
	err := pg.DB.QueryRowContext(ctx, query, userID, server.Name, server.Address, server.Username, newPassword, server.Fingerprint).
		Scan(&server.ID, &server.CreatedAt)

	var pgErr *pgconn.PgError
	if err != nil {
		switch {
		// если ошибка говорит о дубликате сервера - выходим из функции и возвращаем ошибку
		case errors.As(err, &pgErr) && pgErr.Code == "23505":
			return nil, errs.NewErrDuplicatedServer(server.Address, err)
		default:
			return nil, fmt.Errorf("ошибка при добавлении сервера: %w", err)
		}
	}

	// не показываем пароль в возвращаемом "наружу" сервере
	server.Password = ""

	return &server, nil
}

// EditServer Редактирование сервера, принадлежащего пользователю.
func (pg *PgStorage) EditServer(ctx context.Context, editedServer *models.Server, serverID int64, userID int64) (*models.Server, error) {
	var password string

	// если был передан новый пароль - шифруем его для передачи в БД
	if editedServer.Password != "" {
		encryptedPassword, err := utils.EncryptAES([]byte(editedServer.Password), pg.AESKey)
		if err != nil {
			logger.Log.Error("Не удалось зашифровать пароль", logger.String("err", err.Error()))
			return nil, err
		}

		password = encryptedPassword
	} else {
		// Если пароль не был передан, получаем текущий из БД
		var currentPassword string
		getCurrentPasswordQuery := `SELECT password FROM servers WHERE id = $1 AND user_id = $2`
		err := pg.DB.QueryRowContext(ctx, getCurrentPasswordQuery, serverID, userID).Scan(&currentPassword)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, errs.NewErrServerNotFound(serverID, userID, err)
			}
			return nil, fmt.Errorf("ошибка при получении текущего пароля: %w", err)
		}
		password = currentPassword
	}

	// обновляем сервер собранными данными и сразу возвращаем данные для создания возвращаемого "наружу" сервера
	updateQuery := `UPDATE servers SET name = $1, username = $2, address = $3, password = $4 
              WHERE id = $5 AND user_id = $6 
              RETURNING id, name, username, address, fingerprint, created_at`

	var returnedServer models.Server

	// не показываем пароль в возвращаемом "наружу" сервере
	err := pg.DB.QueryRowContext(ctx, updateQuery, editedServer.Name, editedServer.Username, editedServer.Address, password, serverID, userID).
		Scan(&returnedServer.ID, &returnedServer.Name, &returnedServer.Username, &returnedServer.Address, &returnedServer.Fingerprint, &returnedServer.CreatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errs.NewErrServerNotFound(serverID, userID, err)
		}
		return nil, fmt.Errorf("ошибка при обновлении информации о сервере: %w", err)
	}

	return &returnedServer, nil
}

// DelServer Удаление сервера, принадлежащего пользователю.
func (pg *PgStorage) DelServer(ctx context.Context, serverID int64, userID int64) error {
	query := `DELETE FROM servers 
       		  WHERE id = $1 AND user_id = $2`

	Result, err := pg.DB.ExecContext(ctx, query, serverID, userID)

	if err != nil {
		logger.Log.Error("Ошибка запроса", logger.String("err", err.Error()))
		return err
	}

	affectedRows, err := Result.RowsAffected()
	if err != nil {
		return fmt.Errorf("ошибка при выполнении запроса %w", err)
	}

	if affectedRows == 0 {
		return errs.NewErrServerNotFound(serverID, userID, fmt.Errorf("%w: затронутых строк %d", sql.ErrNoRows, affectedRows))
	}

	return nil
}

// GetServer Получение информации о сервере, принадлежащем пользователю.
// Вызывается когда нужно отдать наружу инфо о сервере через API.
func (pg *PgStorage) GetServer(ctx context.Context, serverID int64, userID int64) (*models.Server, error) {
	var server models.Server

	query := `SELECT id, name, address, username, fingerprint, created_at FROM servers 
              WHERE id = $1 AND user_id = $2`

	err := pg.DB.QueryRowContext(ctx, query, serverID, userID).
		Scan(&server.ID, &server.Name, &server.Address, &server.Username, &server.Fingerprint, &server.CreatedAt)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, errs.NewErrServerNotFound(serverID, userID, err)
		default:
			return nil, err
		}
	}

	return &server, nil
}

// GetServerWithPassword Получение информации о сервере (с ПАРОЛЕМ), принадлежащем пользователю.
// Использовать ТОЛЬКО внутри бизнес-логики (WinRM).
// Никогда не отдавать наружу через API!
func (pg *PgStorage) GetServerWithPassword(ctx context.Context, serverID int64, userID int64) (*models.Server, error) {
	var server models.Server

	query := `SELECT id, name, address, username, password, fingerprint, created_at FROM servers 
              WHERE id = $1 AND user_id = $2`

	err := pg.DB.QueryRowContext(ctx, query, serverID, userID).
		Scan(&server.ID, &server.Name, &server.Address, &server.Username, &server.Password, &server.Fingerprint, &server.CreatedAt)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, errs.NewErrServerNotFound(serverID, userID, err)
		default:
			return nil, err
		}
	}

	// расшифровываем пароль
	if server.Password != "" {
		decrypted, err := utils.DecryptAES(server.Password, pg.AESKey)
		if err != nil {
			return nil, fmt.Errorf("не удалось расшифровать пароль: %w", err)
		}
		server.Password = decrypted
	}

	return &server, nil
}

// ListServers Отображение списка серверов, принадлежащих пользователю.
func (pg *PgStorage) ListServers(ctx context.Context, userID int64) ([]*models.Server, error) {
	query := `SELECT id, name, address, username, fingerprint, created_at 
			  FROM servers WHERE user_id = $1
			  ORDER BY name`

	rows, err := pg.DB.QueryContext(ctx, query, userID)
	if err != nil {
		logger.Log.Error("Ошибка при получении списка серверов пользователя", logger.String("err", err.Error()))
		return nil, fmt.Errorf("ошибка при получении серверов пользователя: %w", err)
	}
	defer rows.Close()

	var servers []*models.Server

	for rows.Next() {
		var server models.Server
		err = rows.Scan(&server.ID, &server.Name, &server.Address, &server.Username, &server.Fingerprint, &server.CreatedAt)
		if err != nil {
			logger.Log.Error("ошибка парсинга запроса на получение серверов пользователя", logger.String("err", err.Error()))
			return nil, err
		}

		servers = append(servers, &server)
	}

	err = rows.Err()
	if err != nil {
		logger.Log.Error("Ошибка при обработке строк на получение информации о серверах пользователя", logger.String("err", err.Error()))
		return nil, err
	}

	return servers, nil
}

// AddService Добавление службы на сервер, принадлежащий пользователю.
func (pg *PgStorage) AddService(ctx context.Context, serverID int64, userID int64, service models.Service) (*models.Service, error) {
	// создаем транзакцию при добавлении службы, чтобы гарантированно получить из базы актуальный
	// статус службы и время его изменения и не попасть в ситуацию, когда кто-то параллельно изменил ее статус
	// и время изменения (например, сделав какую-то операцию над службой)
	tx, err := pg.DB.BeginTx(ctx, nil)
	if err != nil {
		logger.Log.Error("Ошибка транзакции при добавлении службы", logger.String("err", err.Error()))
		return nil, fmt.Errorf("не удалось начать транзакцию добавления службы: %w", err)
	}
	defer tx.Rollback()

	// получаем fingerprint сервера и проверяем пользователя
	var fingerprint uuid.UUID
	queryFingerprint := `SELECT fingerprint 
                    FROM servers 
                    WHERE id = $1 AND user_id = $2`
	err = tx.QueryRowContext(ctx, queryFingerprint, serverID, userID).Scan(&fingerprint)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errs.NewErrServerNotFound(serverID, userID, err)
		}
		return nil, fmt.Errorf("ошибка при получении сервера: %w", err)
	}

	// проверяем, есть ли такая служба на других серверах с тем же fingerprint
	var lastStatus string
	var lastUpdated time.Time

	queryStatusUpdatedAt := `SELECT status, updated_at
							 FROM services
							 WHERE service_name = $3
								AND server_id IN (
									SELECT id
									FROM servers
									WHERE fingerprint = $1
									AND id <> $2
								)
							ORDER BY updated_at DESC
							LIMIT 1;`

	err = tx.QueryRowContext(ctx, queryStatusUpdatedAt, fingerprint, serverID, service.ServiceName).
		Scan(&lastStatus, &lastUpdated)

	switch {
	case err == nil:
		// если такая служба на других серверах с тем же fingerprint уже существует -
		// берем значение status и updated_at у нее
		// (сортировка в queryStatusUpdatedAt выведет службу с самыми последними изменениями)
		service.Status = lastStatus
		service.UpdatedAt = lastUpdated
	case errors.Is(err, sql.ErrNoRows):
		// если ErrNoRows — оставляем status и updated_at, которые пришли из WinRM (уже заполнены в хэндлере)
	default:
		return nil, fmt.Errorf("ошибка при проверке статуса других серверов: %w", err)
	}

	// вставляем новую службу
	queryInsert := `
        INSERT INTO services (server_id, displayed_name, service_name, status, updated_at)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING id, created_at, updated_at
    `
	err = tx.QueryRowContext(ctx, queryInsert, serverID, service.DisplayedName, service.ServiceName, service.Status, service.UpdatedAt).
		Scan(&service.ID, &service.CreatedAt, &service.UpdatedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, errs.NewErrDuplicatedService(service.ServiceName, err)
		}
		return nil, fmt.Errorf("ошибка при добавлении службы: %w", err)
	}

	if err = tx.Commit(); err != nil {
		logger.Log.Error("Ошибка при коммите транзакции добавления службы", logger.String("err", err.Error()))
		return nil, fmt.Errorf("ошибка при коммите транзакции добавления службы: %w", err)
	}

	return &service, nil
}

// DelService Удаление службы с сервера пользователя.
func (pg *PgStorage) DelService(ctx context.Context, serverID int64, serviceID int64, userID int64) error {
	query := `DELETE FROM services 
              WHERE id = $1 
                AND server_id = $2
                AND server_id IN (
                	SELECT id FROM servers 
                	WHERE user_id = $3
                )`

	Result, err := pg.DB.ExecContext(ctx, query, serviceID, serverID, userID)
	if err != nil {
		logger.Log.Error("Ошибка запроса", logger.String("err", err.Error()))
		return err
	}

	affectedRows, err := Result.RowsAffected()
	if err != nil {
		return fmt.Errorf("ошибка при выполнении запроса %w", err)
	}

	if affectedRows == 0 {
		return errs.NewErrServiceNotFound(userID, serverID, serviceID, fmt.Errorf("%w: затронутых строк %d", sql.ErrNoRows, affectedRows))
	}

	return nil
}

// ChangeServiceStatus Изменение статуса службы.
func (pg *PgStorage) ChangeServiceStatus(ctx context.Context, serverID int64, serviceName string, status string) error {
	// если один пользователь обновляет статус и время службы на сервере,
	// то эти изменения применяются для всех пользователей, у которых добавлен данный сервер.
	query := `UPDATE services SET status = $1, updated_at = CURRENT_TIMESTAMP
              WHERE service_name = $2 
                AND server_id IN (
                	SELECT id FROM servers 
                	WHERE fingerprint = (SELECT fingerprint FROM servers WHERE id = $3)
                )`

	Result, err := pg.DB.ExecContext(ctx, query, status, serviceName, serverID)

	if err != nil {
		logger.Log.Error("Ошибка запроса", logger.String("err", err.Error()))
		return err
	}

	affectedRows, err := Result.RowsAffected()
	if err != nil {
		return fmt.Errorf("ошибка при выполнении запроса %w", err)
	}

	if affectedRows == 0 {
		return fmt.Errorf("ошибка изменения статуса службы `%s` на сервере %d", serviceName, serverID)
	}

	return nil
}

// BatchChangeServiceStatus Изменение статуса служб батчем.
func (pg *PgStorage) BatchChangeServiceStatus(ctx context.Context, serverID int64, servicesBatch []*models.Service) error {
	// если один пользователь обновляет статус и время службы на сервере,
	// то эти изменения применяются для всех пользователей, у которых добавлен данный сервер.
	query := `UPDATE services SET status = $1, updated_at = $2
              WHERE service_name = $3 
                AND server_id IN (
                	SELECT id FROM servers 
                	WHERE fingerprint = (SELECT fingerprint FROM servers WHERE id = $4)
                )`

	// создаем транзакцию
	tx, err := pg.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ошибка создания транзакции %w ", err)
	}

	defer tx.Rollback()

	// создаем подготовленный запрос
	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("ошибка подготовки запроса %w ", err)
	}

	defer stmt.Close()

	for _, service := range servicesBatch {
		Result, err := stmt.ExecContext(ctx, service.Status, service.UpdatedAt, service.ServiceName, serverID)

		if err != nil {
			logger.Log.Error("Ошибка запроса", logger.String("err", err.Error()))
			return err
		}

		affectedRows, err := Result.RowsAffected()
		if err != nil {
			return fmt.Errorf("ошибка при выполнении запроса %w", err)
		}

		if affectedRows == 0 {
			return fmt.Errorf("ошибка изменения статуса службы `%s` на сервере %d", service.ServiceName, serverID)
		}
	}

	// коммитим транзакцию
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("ошибка при коммите транзакции: %v", err)
	}

	return nil
}

// GetService Получение службы с сервера пользователя.
func (pg *PgStorage) GetService(ctx context.Context, serverID int64, serviceID int64, userID int64) (*models.Service, error) {
	query := `SELECT id, displayed_name, service_name, status, created_at, updated_at 
			  FROM services 
			  WHERE id = $1 
			    AND server_id = $2 
			    AND server_id IN (
					SELECT id FROM servers 
					WHERE user_id = $3
			    )`

	var service models.Service

	err := pg.DB.QueryRowContext(ctx, query, serviceID, serverID, userID).
		Scan(&service.ID, &service.DisplayedName, &service.ServiceName, &service.Status, &service.CreatedAt, &service.UpdatedAt)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, errs.NewErrServiceNotFound(userID, serverID, serviceID, err)
		default:
			return nil, err
		}
	}

	return &service, nil
}

// ListServices Получение списка служб сервера, принадлежащего пользователю.
func (pg *PgStorage) ListServices(ctx context.Context, serverID int64, userID int64) ([]*models.Service, error) {

	// Сначала проверяем, принадлежит ли сервер пользователю
	var exists bool

	checkOwnershipQuery := `SELECT EXISTS(
							SELECT 1 FROM servers
							WHERE id = $1 AND user_id = $2
							)`

	err := pg.DB.QueryRowContext(ctx, checkOwnershipQuery, serverID, userID).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("ошибка при проверке владения сервером: %w", err)
	}

	if !exists {
		return nil, errs.NewErrServerNotFound(serverID, userID, fmt.Errorf("сервер не найден или не принадлежит пользователю"))
	}

	// Теперь получаем службы
	query := `SELECT id, displayed_name, service_name, status, created_at, updated_at
			  FROM services 
			  WHERE server_id = $1
			  ORDER BY service_name`

	var services []*models.Service

	rows, err := pg.DB.QueryContext(ctx, query, serverID)
	if err != nil {
		logger.Log.Error("Ошибка при получении списка служб сервера пользователя", logger.String("err", err.Error()))
		return nil, fmt.Errorf("ошибка при получении списка служб сервера пользователя: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var service models.Service

		err = rows.Scan(&service.ID, &service.DisplayedName, &service.ServiceName, &service.Status, &service.CreatedAt, &service.UpdatedAt)
		if err != nil {
			logger.Log.Error("ошибка парсинга запроса на получение серверов пользователя", logger.String("err", err.Error()))
			return nil, err
		}

		services = append(services, &service)
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

	_, err = pg.DB.ExecContext(ctx, query, user.Login, string(hashedPassword))
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

	query := `SELECT id, login, password FROM users WHERE login = $1`
	err := pg.DB.QueryRowContext(ctx, query, user.Login).Scan(&userFromDB.ID, &userFromDB.Login, &userFromDB.Password)

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

//// GetUserIDByLogin Возвращает userID пользователя если он зарегистрирован.
//func (pg *PgStorage) GetUserIDByLogin(ctx context.Context, login string) (int, error) {
//	var userID int
//
//	query := `SELECT id FROM users WHERE login = $1`
//
//	err := pg.DB.QueryRowContext(ctx, query, login).Scan(&userID)
//
//	if err != nil {
//		switch {
//		case errors.Is(err, sql.ErrNoRows):
//			return 0, errs.NewErrLoginNotFound(err)
//		default:
//			logger.Log.Error("Ошибка запроса", logger.String("err", err.Error()))
//			return 0, err
//		}
//	}
//
//	return userID, nil
//}

// ListUsers Получение списка всех пользователей.
func (pg *PgStorage) ListUsers(ctx context.Context) ([]*models.User, error) {
	var users []*models.User

	query := `SELECT id, login FROM users`

	rows, err := pg.DB.QueryContext(ctx, query)
	if err != nil {
		logger.Log.Error("Ошибка при выполнении запроса на получение списка пользователей", logger.String("err", err.Error()))
		return nil, fmt.Errorf("ошибка при выполнении запроса на получение списка пользователей: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var user models.User

		err = rows.Scan(&user.ID, &user.Login)
		if err != nil {
			logger.Log.Error("Ошибка сканирования строки списка пользователей", logger.String("err", err.Error()))
			return nil, err
		}

		users = append(users, &user)
	}

	err = rows.Err()
	if err != nil {
		logger.Log.Error("Ошибка при обработке строк списка пользователей", logger.String("err", err.Error()))
		return nil, err
	}

	return users, nil
}

// GetUserServiceStatuses возвращает все службы со статусами, которые принадлежат указанному пользователю.
func (pg *PgStorage) GetUserServiceStatuses(ctx context.Context, userID int64) ([]*models.ServiceStatus, error) {
	query := `SELECT id, server_id, status, updated_at 
			  FROM services
			  WHERE server_id IN (SELECT id FROM servers WHERE user_id = $1)`

	var statuses []*models.ServiceStatus

	rows, err := pg.DB.QueryContext(ctx, query, userID)
	if err != nil {
		logger.Log.Error("Ошибка при выполнении запроса статусов служб пользователя", logger.String("err", err.Error()))
		return nil, fmt.Errorf("ошибка при выполнении запроса статусов служб пользователя: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var status models.ServiceStatus

		err = rows.Scan(&status.ID, &status.ServerID, &status.Status, &status.UpdatedAt)
		if err != nil {
			logger.Log.Error("Ошибка сканирования строки статусов служб пользователя", logger.String("err", err.Error()))
			return nil, err
		}

		statuses = append(statuses, &status)
	}

	err = rows.Err()
	if err != nil {
		logger.Log.Error("Ошибка при обработке строк статусов служб пользователя", logger.String("err", err.Error()))
		return nil, err
	}

	return statuses, nil
}

// Ping Проверяет доступность PostgreSQL с таймаутом.
func (pg *PgStorage) Ping(ctx context.Context) error {
	if err := pg.DB.PingContext(ctx); err != nil {
		logger.Log.Error("База данных Postgres не отвечает", logger.String("error", err.Error()))
		return err
	}

	return nil
}

// Close Закрытие соединения с БД.
func (pg *PgStorage) Close() error {
	err := pg.DB.Close()
	if err != nil {
		logger.Log.Error("Ошибка закрытия соединения с БД PostgreSQL", logger.String("err", err.Error()))
		return fmt.Errorf("ошибка закрытия БД PostgreSQL: %w", err)
	}

	return nil
}

//func (pg *PgStorage) ListServersAddresses(ctx context.Context) ([]*models.ServerStatus, error) {
//	query := `SELECT id, address FROM servers ORDER BY id`
//
//	rows, err := pg.DB.QueryContext(ctx, query)
//	if err != nil {
//		logger.Log.Error("Ошибка при получении списка всех серверов", logger.String("err", err.Error()))
//		return nil, fmt.Errorf("ошибка при получении всех серверов: %w", err)
//	}
//	defer rows.Close()
//
//	var servers []*models.ServerStatus
//
//	for rows.Next() {
//		var server models.ServerStatus
//		err = rows.Scan(&server.ServerID, &server.Address)
//		if err != nil {
//			logger.Log.Error("ошибка парсинга запроса на получение всех серверов", logger.String("err", err.Error()))
//			return nil, err
//		}
//
//		servers = append(servers, &server)
//	}
//
//	err = rows.Err()
//	if err != nil {
//		logger.Log.Error("Ошибка при обработке строк на получение информации о всех серверах", logger.String("err", err.Error()))
//		return nil, err
//	}
//
//	return servers, nil
//}
