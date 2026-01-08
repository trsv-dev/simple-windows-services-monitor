package postgres

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/errs"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
	"golang.org/x/crypto/bcrypt"
)

// init Инициализирует logger для тестов.
func init() {
	// инициализируем logger.Log через InitLogger
	logger.InitLogger("error", "stdout")
}

// TestAddServer Проверяет добавление сервера в базу данных.
func TestAddServer(t *testing.T) {
	// Подготовка тестовых данных
	fixedTime := time.Now()
	testUserID := int64(1)
	testServerID := int64(100)
	// AES ключ должен быть ровно 32 байта для AES-256
	aesKey := []byte("12345678901234567890123456789012")

	addServerQuery := `INSERT INTO servers (user_id, name, address, username, password, fingerprint) 
			  VALUES ($1, $2, $3, $4, $5, $6)
              RETURNING id, created_at`

	tests := []struct {
		name           string                                    // название теста
		server         models.Server                             // входные данные сервера
		userID         int64                                     // ID пользователя
		mockSetup      func(mock sqlmock.Sqlmock)                // настройка мока базы данных
		expectError    bool                                      // ожидается ли ошибка
		errorAssertion func(t *testing.T, err error)             // дополнительная проверка ошибки
		validate       func(t *testing.T, result *models.Server) // валидация результата
	}{
		{
			name: "успешное добавление сервера с паролем",
			server: models.Server{
				Name:        "Test Server",
				Address:     "192.168.1.100",
				Username:    "admin",
				Password:    "password123",
				Fingerprint: uuid.New(),
			},
			userID: testUserID,
			mockSetup: func(mock sqlmock.Sqlmock) {
				// Ожидаем SQL запрос с определенными параметрами
				mock.ExpectQuery(regexp.QuoteMeta(addServerQuery)).
					WithArgs(testUserID, "Test Server", "192.168.1.100", "admin", sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).
						AddRow(testServerID, fixedTime))
			},
			expectError: false,
			validate: func(t *testing.T, result *models.Server) {
				assert.NotNil(t, result)
				assert.Equal(t, testServerID, result.ID)
				assert.Equal(t, "Test Server", result.Name)
				assert.Equal(t, "192.168.1.100", result.Address)
				assert.Equal(t, "admin", result.Username)
				assert.Empty(t, result.Password) // пароль не должен возвращаться
				assert.Equal(t, fixedTime, result.CreatedAt)
			},
		},
		{
			name: "успешное добавление сервера без пароля",
			server: models.Server{
				Name:        "Test Server No Pass",
				Address:     "192.168.1.101",
				Username:    "user",
				Password:    "",
				Fingerprint: uuid.New(),
			},
			userID: testUserID,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(addServerQuery)).
					WithArgs(testUserID, "Test Server No Pass", "192.168.1.101", "user", "", sqlmock.AnyArg()).
					WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).
						AddRow(testServerID, fixedTime))
			},
			expectError: false,
			validate: func(t *testing.T, result *models.Server) {
				assert.NotNil(t, result)
				assert.Equal(t, testServerID, result.ID)
				assert.Empty(t, result.Password)
			},
		},
		{
			name: "ошибка дубликата сервера",
			server: models.Server{
				Name:        "Duplicate Server",
				Address:     "192.168.1.102",
				Username:    "admin",
				Password:    "pass",
				Fingerprint: uuid.New(),
			},
			userID: testUserID,
			mockSetup: func(mock sqlmock.Sqlmock) {
				// Симулируем ошибку уникального ограничения PostgreSQL
				mock.ExpectQuery(regexp.QuoteMeta(addServerQuery)).
					WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
						sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnError(&pgconn.PgError{Code: "23505"})
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				// Проверяем, что это ошибка дубликата
				var dupErr *errs.ErrDuplicatedServer
				assert.True(t, errors.As(err, &dupErr), "ошибка должна быть типа ErrDuplicatedServer")
			},
			validate: func(t *testing.T, result *models.Server) {
				assert.Nil(t, result)
			},
		},
		{
			name: "общая ошибка базы данных",
			server: models.Server{
				Name:        "Error Server",
				Address:     "192.168.1.103",
				Username:    "admin",
				Password:    "pass",
				Fingerprint: uuid.New(),
			},
			userID: testUserID,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(addServerQuery)).
					WillReturnError(errors.New("database connection error"))
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "ошибка при добавлении сервера")
			},
			validate: func(t *testing.T, result *models.Server) {
				assert.Nil(t, result)
			},
		},
	}

	// Запуск тестовых кейсов
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Создаем mock базы данных
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			// Настройка mock согласно тестовому сценарию
			tt.mockSetup(mock)

			// Создаем экземпляр PgStorage с mock БД
			pg := &PgStorage{
				DB:     db,
				AESKey: aesKey,
			}

			// Выполняем тестируемый метод
			result, err := pg.AddServer(context.Background(), tt.server, tt.userID)

			// Проверяем ошибки
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorAssertion != nil {
					tt.errorAssertion(t, err)
				}
			} else {
				assert.NoError(t, err)
			}

			// Валидация результата
			tt.validate(t, result)

			// Проверяем, что все ожидания mock выполнены
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestEditServer Проверяет редактирование существующего сервера.
func TestEditServer(t *testing.T) {
	fixedTime := time.Now()
	testUserID := int64(1)
	testServerID := int64(100)
	// AES ключ должен быть ровно 32 байта для AES-256
	aesKey := []byte("12345678901234567890123456789012")
	testFingerprint := uuid.New()

	editServerQuery := `UPDATE servers SET name = $1, username = $2, address = $3, password = $4
	         			WHERE id = $5 AND user_id = $6
	         			RETURNING id, name, username, address, fingerprint, created_at`

	tests := []struct {
		name           string                                    // название теста
		editedServer   *models.Server                            // данные для обновления
		serverID       int64                                     // ID сервера
		userID         int64                                     // ID пользователя
		mockSetup      func(mock sqlmock.Sqlmock)                // настройка мока
		expectError    bool                                      // ожидается ли ошибка
		errorAssertion func(t *testing.T, err error)             // дополнительная проверка ошибки
		validate       func(t *testing.T, result *models.Server) // валидация результата
	}{
		{
			name: "успешное редактирование сервера с новым паролем",
			editedServer: &models.Server{
				Name:     "Updated Server",
				Address:  "192.168.1.200",
				Username: "newadmin",
				Password: "newpassword",
			},
			serverID: testServerID,
			userID:   testUserID,
			mockSetup: func(mock sqlmock.Sqlmock) {
				// Ожидаем UPDATE запрос
				mock.ExpectQuery(regexp.QuoteMeta(editServerQuery)).
					WithArgs("Updated Server", "newadmin", "192.168.1.200",
						sqlmock.AnyArg(), testServerID, testUserID).
					WillReturnRows(sqlmock.NewRows([]string{"id", "name", "username", "address", "fingerprint", "created_at"}).
						AddRow(testServerID, "Updated Server", "newadmin", "192.168.1.200", testFingerprint, fixedTime))
			},
			expectError: false,
			validate: func(t *testing.T, result *models.Server) {
				assert.NotNil(t, result)
				assert.Equal(t, testServerID, result.ID)
				assert.Equal(t, "Updated Server", result.Name)
				assert.Equal(t, "newadmin", result.Username)
				assert.Empty(t, result.Password) // пароль не возвращается
			},
		},
		{
			name: "редактирование сервера без изменения пароля",
			editedServer: &models.Server{
				Name:     "Updated Server No Pass",
				Address:  "192.168.1.201",
				Username: "admin",
				Password: "", // пароль не передан
			},
			serverID: testServerID,
			userID:   testUserID,
			mockSetup: func(mock sqlmock.Sqlmock) {
				// Ожидаем SELECT для получения текущего пароля
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT password FROM servers WHERE id = $1 AND user_id = $2`)).
					WithArgs(testServerID, testUserID).
					WillReturnRows(sqlmock.NewRows([]string{"password"}).
						AddRow("encrypted_old_password"))

				// Ожидаем UPDATE запрос
				mock.ExpectQuery(regexp.QuoteMeta(editServerQuery)).
					WithArgs("Updated Server No Pass", "admin", "192.168.1.201",
						"encrypted_old_password", testServerID, testUserID).
					WillReturnRows(sqlmock.NewRows([]string{"id", "name", "username", "address", "fingerprint", "created_at"}).
						AddRow(testServerID, "Updated Server No Pass", "admin", "192.168.1.201", testFingerprint, fixedTime))
			},
			expectError: false,
			validate: func(t *testing.T, result *models.Server) {
				assert.NotNil(t, result)
				assert.Equal(t, testServerID, result.ID)
				assert.Equal(t, "Updated Server No Pass", result.Name)
			},
		},
		{
			name: "ошибка - сервер не найден",
			editedServer: &models.Server{
				Name:     "Nonexistent Server",
				Address:  "192.168.1.202",
				Username: "admin",
				Password: "pass",
			},
			serverID: testServerID,
			userID:   testUserID,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(editServerQuery)).
					WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
						sqlmock.AnyArg(), testServerID, testUserID).
					WillReturnError(sql.ErrNoRows)
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				var notFoundErr *errs.ErrServerNotFound
				assert.True(t, errors.As(err, &notFoundErr), "ошибка должна быть типа ErrServerNotFound")
			},
			validate: func(t *testing.T, result *models.Server) {
				assert.Nil(t, result)
			},
		},
		{
			name: "ошибка получения текущего пароля - сервер не найден",
			editedServer: &models.Server{
				Name:     "Test Server",
				Address:  "192.168.1.203",
				Username: "admin",
				Password: "",
			},
			serverID: testServerID,
			userID:   testUserID,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT password FROM servers WHERE id = $1 AND user_id = $2`)).
					WithArgs(testServerID, testUserID).
					WillReturnError(sql.ErrNoRows)
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				var notFoundErr *errs.ErrServerNotFound
				assert.True(t, errors.As(err, &notFoundErr), "ошибка должна быть типа ErrServerNotFound")
			},
			validate: func(t *testing.T, result *models.Server) {
				assert.Nil(t, result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tt.mockSetup(mock)

			pg := &PgStorage{
				DB:     db,
				AESKey: aesKey,
			}

			result, err := pg.EditServer(context.Background(), tt.editedServer, tt.serverID, tt.userID)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorAssertion != nil {
					tt.errorAssertion(t, err)
				}
			} else {
				assert.NoError(t, err)
			}

			tt.validate(t, result)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestDelServer Проверяет удаление сервера.
func TestDelServer(t *testing.T) {
	testUserID := int64(1)
	testServerID := int64(100)

	deleteServerQuery := `DELETE FROM servers 
             			  WHERE id = $1 AND user_id = $2`

	tests := []struct {
		name           string                     // название теста
		serverID       int64                      // ID сервера для удаления
		userID         int64                      // ID пользователя
		mockSetup      func(mock sqlmock.Sqlmock) // настройка мока
		expectError    bool                       // ожидается ли ошибка
		errorAssertion func(t *testing.T, err error)
	}{
		{
			name:     "успешное удаление сервера",
			serverID: testServerID,
			userID:   testUserID,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta(deleteServerQuery)).
					WithArgs(testServerID, testUserID).
					WillReturnResult(sqlmock.NewResult(0, 1)) // 1 строка затронута
			},
			expectError: false,
		},
		{
			name:     "ошибка - сервер не найден",
			serverID: testServerID,
			userID:   testUserID,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta(deleteServerQuery)).
					WithArgs(testServerID, testUserID).
					WillReturnResult(sqlmock.NewResult(0, 0)) // 0 строк затронуто
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				var notFoundErr *errs.ErrServerNotFound
				assert.True(t, errors.As(err, &notFoundErr), "ошибка должна быть типа ErrServerNotFound")
			},
		},
		{
			name:     "ошибка выполнения запроса",
			serverID: testServerID,
			userID:   testUserID,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta(deleteServerQuery)).
					WithArgs(testServerID, testUserID).
					WillReturnError(errors.New("database error"))
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "database error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tt.mockSetup(mock)

			pg := &PgStorage{DB: db}

			err = pg.DelServer(context.Background(), tt.serverID, tt.userID)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorAssertion != nil {
					tt.errorAssertion(t, err)
				}
			} else {
				assert.NoError(t, err)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestGetServer Проверяет получение информации о сервере без пароля.
func TestGetServer(t *testing.T) {
	fixedTime := time.Now()
	testUserID := int64(1)
	testServerID := int64(100)
	testFingerprint := uuid.New()

	getServerQuery := `SELECT id, name, address, username, fingerprint, created_at 
					   FROM servers 
              		   WHERE id = $1 AND user_id = $2`

	tests := []struct {
		name           string                                    // название теста
		serverID       int64                                     // ID сервера
		userID         int64                                     // ID пользователя
		mockSetup      func(mock sqlmock.Sqlmock)                // настройка мока
		expectError    bool                                      // ожидается ли ошибка
		errorAssertion func(t *testing.T, err error)             // дополнительная проверка ошибки
		validate       func(t *testing.T, result *models.Server) // валидация результата
	}{
		{
			name:     "успешное получение сервера",
			serverID: testServerID,
			userID:   testUserID,
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "name", "address", "username", "fingerprint", "created_at"}).
					AddRow(testServerID, "Test Server", "192.168.1.100", "admin", testFingerprint, fixedTime)
				mock.ExpectQuery(regexp.QuoteMeta(getServerQuery)).
					WithArgs(testServerID, testUserID).
					WillReturnRows(rows)
			},
			expectError: false,
			validate: func(t *testing.T, result *models.Server) {
				assert.NotNil(t, result)
				assert.Equal(t, testServerID, result.ID)
				assert.Equal(t, "Test Server", result.Name)
				assert.Equal(t, "192.168.1.100", result.Address)
				assert.Equal(t, "admin", result.Username)
				assert.Equal(t, testFingerprint, result.Fingerprint)
				assert.Equal(t, fixedTime, result.CreatedAt)
			},
		},
		{
			name:     "ошибка - сервер не найден",
			serverID: testServerID,
			userID:   testUserID,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(getServerQuery)).
					WithArgs(testServerID, testUserID).
					WillReturnError(sql.ErrNoRows)
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				var notFoundErr *errs.ErrServerNotFound
				assert.True(t, errors.As(err, &notFoundErr), "ошибка должна быть типа ErrServerNotFound")
			},
			validate: func(t *testing.T, result *models.Server) {
				assert.Nil(t, result)
			},
		},
		{
			name:     "общая ошибка базы данных",
			serverID: testServerID,
			userID:   testUserID,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(getServerQuery)).
					WithArgs(testServerID, testUserID).
					WillReturnError(errors.New("database error"))
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "database error")
			},
			validate: func(t *testing.T, result *models.Server) {
				assert.Nil(t, result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tt.mockSetup(mock)

			pg := &PgStorage{DB: db}

			result, err := pg.GetServer(context.Background(), tt.serverID, tt.userID)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorAssertion != nil {
					tt.errorAssertion(t, err)
				}
			} else {
				assert.NoError(t, err)
			}

			tt.validate(t, result)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestGetServerWithPassword Проверяет получение сервера с паролем.
func TestGetServerWithPassword(t *testing.T) {
	fixedTime := time.Now()
	testUserID := int64(1)
	testServerID := int64(100)
	// тестовый AES ключ 32 байта
	aesKey := []byte("12345678901234567890123456789012")

	getUserDataQuery := `SELECT id, name, address, username, password, fingerprint, created_at 
			  FROM servers 
              WHERE id = $1 AND user_id = $2`

	tests := []struct {
		name           string                                    // название теста
		serverID       int64                                     // ID сервера
		userID         int64                                     // ID пользователя
		dbPassword     string                                    // что вернет поле password из БД
		mockSetup      func(mock sqlmock.Sqlmock)                // настройка мока
		expectError    bool                                      // ожидается ли ошибка
		errorAssertion func(t *testing.T, err error)             // дополнительная проверка ошибки
		validate       func(t *testing.T, result *models.Server) // валидация результата
	}{
		{
			name:       "успешное получение и расшифровка пароля",
			serverID:   testServerID,
			userID:     testUserID,
			dbPassword: "dGVzdFBhc3M=", // base64 testPass -> utils.DecryptAES не поддерживает base64, вызовет ошибку
			mockSetup: func(mock sqlmock.Sqlmock) {
				// возвращаем данные с не пустым паролем
				row := sqlmock.NewRows([]string{"id", "name", "address", "username", "password", "fingerprint", "created_at"}).
					AddRow(testServerID, "TestSrv", "addr", "user", "invalidcipher", uuid.New(), fixedTime)
				mock.ExpectQuery(regexp.QuoteMeta(getUserDataQuery)).
					WithArgs(testServerID, testUserID).
					WillReturnRows(row)
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "не удалось расшифровать пароль")
			},
		},
		{
			name:       "успешное получение без пароля",
			serverID:   testServerID,
			userID:     testUserID,
			dbPassword: "", // пустой пароль
			mockSetup: func(mock sqlmock.Sqlmock) {
				row := sqlmock.NewRows([]string{"id", "name", "address", "username", "password", "fingerprint", "created_at"}).
					AddRow(testServerID, "TestSrv", "addr", "user", "", uuid.New(), fixedTime)
				mock.ExpectQuery(regexp.QuoteMeta(getUserDataQuery)).
					WithArgs(testServerID, testUserID).
					WillReturnRows(row)
			},
			expectError: false,
			validate: func(t *testing.T, result *models.Server) {
				assert.NotNil(t, result)
				assert.Equal(t, testServerID, result.ID)
				assert.Equal(t, "", result.Password)
			},
		},
		{
			name:     "ошибка - сервер не найден",
			serverID: testServerID,
			userID:   testUserID,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(getUserDataQuery)).
					WithArgs(testServerID, testUserID).
					WillReturnError(sql.ErrNoRows)
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				var notFoundErr *errs.ErrServerNotFound
				assert.True(t, errors.As(err, &notFoundErr))
			},
		},
		{
			name:     "общая ошибка запроса",
			serverID: testServerID,
			userID:   testUserID,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(getUserDataQuery)).
					WithArgs(testServerID, testUserID).
					WillReturnError(errors.New("db error"))
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "db error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tt.mockSetup(mock)

			pg := &PgStorage{DB: db, AESKey: aesKey}

			result, err := pg.GetServerWithPassword(context.Background(), tt.serverID, tt.userID)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorAssertion != nil {
					tt.errorAssertion(t, err)
				}
			} else {
				assert.NoError(t, err)
			}

			if tt.validate != nil {
				tt.validate(t, result)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestListServers Проверяет получение списка серверов пользователя.
func TestListServers(t *testing.T) {
	fixedTime := time.Now()
	testUserID := int64(1)
	fp1 := uuid.New()
	fp2 := uuid.New()

	listServersQuery := `SELECT id, name, address, username, fingerprint, created_at 
                         FROM servers WHERE user_id = $1
            			 ORDER BY name`

	tests := []struct {
		name           string                                      // название теста
		userID         int64                                       // ID пользователя
		mockSetup      func(mock sqlmock.Sqlmock)                  // настройка мока
		expectError    bool                                        // ожидается ли ошибка
		errorAssertion func(t *testing.T, err error)               // дополнительная проверка ошибки
		validate       func(t *testing.T, result []*models.Server) // валидация результата
	}{
		{
			name:   "успешное получение списка серверов",
			userID: testUserID,
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "name", "address", "username", "fingerprint", "created_at"}).
					AddRow(1, "Server 1", "192.168.1.1", "admin1", fp1, fixedTime).
					AddRow(2, "Server 2", "192.168.1.2", "admin2", fp2, fixedTime)
				mock.ExpectQuery(regexp.QuoteMeta(listServersQuery)).
					WithArgs(testUserID).
					WillReturnRows(rows)
			},
			expectError: false,
			validate: func(t *testing.T, result []*models.Server) {
				assert.NotNil(t, result)
				assert.Len(t, result, 2)
				assert.Equal(t, "Server 1", result[0].Name)
				assert.Equal(t, "Server 2", result[1].Name)
			},
		},
		{
			name:   "пустой список серверов",
			userID: testUserID,
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "name", "address", "username", "fingerprint", "created_at"})
				mock.ExpectQuery(regexp.QuoteMeta(listServersQuery)).
					WithArgs(testUserID).
					WillReturnRows(rows)
			},
			expectError: false,
			validate: func(t *testing.T, result []*models.Server) {
				// при пустом результате метод может вернуть nil или пустой слайс
				assert.Empty(t, result)
			},
		},
		{
			name:   "ошибка базы данных",
			userID: testUserID,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(listServersQuery)).
					WithArgs(testUserID).
					WillReturnError(errors.New("database error"))
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "ошибка при получении серверов пользователя")
			},
			validate: func(t *testing.T, result []*models.Server) {
				assert.Nil(t, result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tt.mockSetup(mock)

			pg := &PgStorage{DB: db}

			result, err := pg.ListServers(context.Background(), tt.userID)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorAssertion != nil {
					tt.errorAssertion(t, err)
				}
			} else {
				assert.NoError(t, err)
			}

			tt.validate(t, result)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestGetService Проверяет получение службы с сервера пользователя.
func TestGetService(t *testing.T) {
	fixedTime := time.Now()
	testUserID := int64(1)
	testServerID := int64(100)
	testServiceID := int64(10)

	getServerQuery := `SELECT id, displayed_name, service_name, status, created_at, updated_at 
                       FROM services 
                       WHERE id = $1 
                          AND server_id = $2 
                          AND server_id IN (
                               SELECT id FROM servers 
                               WHERE user_id = $3
                          )`

	tests := []struct {
		name           string                                     // название теста
		serverID       int64                                      // ID сервера
		serviceID      int64                                      // ID службы
		userID         int64                                      // ID пользователя
		mockSetup      func(mock sqlmock.Sqlmock)                 // настройка мока
		expectError    bool                                       // ожидается ли ошибка
		errorAssertion func(t *testing.T, err error)              // дополнительная проверка ошибки
		validate       func(t *testing.T, result *models.Service) // валидация результата
	}{
		{
			name:      "успешное получение службы",
			serverID:  testServerID,
			serviceID: testServiceID,
			userID:    testUserID,
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "displayed_name", "service_name", "status", "created_at", "updated_at"}).
					AddRow(testServiceID, "Web Server", "nginx", "Running", fixedTime, fixedTime)
				mock.ExpectQuery(regexp.QuoteMeta(getServerQuery)).
					WithArgs(testServiceID, testServerID, testUserID).
					WillReturnRows(rows)
			},
			expectError: false,
			validate: func(t *testing.T, result *models.Service) {
				assert.NotNil(t, result)
				assert.Equal(t, testServiceID, result.ID)
				assert.Equal(t, "Web Server", result.DisplayedName)
				assert.Equal(t, "nginx", result.ServiceName)
				assert.Equal(t, "Running", result.Status)
				assert.Equal(t, fixedTime, result.CreatedAt)
				assert.Equal(t, fixedTime, result.UpdatedAt)
			},
		},
		{
			name:      "ошибка - служба не найдена",
			serverID:  testServerID,
			serviceID: testServiceID,
			userID:    testUserID,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(getServerQuery)).
					WithArgs(testServiceID, testServerID, testUserID).
					WillReturnError(sql.ErrNoRows)
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				var notFoundErr *errs.ErrServiceNotFound
				assert.True(t, errors.As(err, &notFoundErr), "ошибка должна быть типа ErrServiceNotFound")
			},
			validate: func(t *testing.T, result *models.Service) {
				assert.Nil(t, result)
			},
		},
		{
			name:      "ошибка - служба не принадлежит пользователю",
			serverID:  testServerID,
			serviceID: testServiceID,
			userID:    int64(999), // другой пользователь
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(getServerQuery)).
					WithArgs(testServiceID, testServerID, int64(999)).
					WillReturnError(sql.ErrNoRows)
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				var notFoundErr *errs.ErrServiceNotFound
				assert.True(t, errors.As(err, &notFoundErr), "ошибка должна быть типа ErrServiceNotFound")
			},
			validate: func(t *testing.T, result *models.Service) {
				assert.Nil(t, result)
			},
		},
		{
			name:      "общая ошибка базы данных",
			serverID:  testServerID,
			serviceID: testServiceID,
			userID:    testUserID,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(getServerQuery)).
					WithArgs(testServiceID, testServerID, testUserID).
					WillReturnError(errors.New("database error"))
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "database error")
			},
			validate: func(t *testing.T, result *models.Service) {
				assert.Nil(t, result)
			},
		},
		{
			name:      "ошибка сканирования - неверный тип данных",
			serverID:  testServerID,
			serviceID: testServiceID,
			userID:    testUserID,
			mockSetup: func(mock sqlmock.Sqlmock) {
				// Возвращаем строку вместо int64 для ID
				rows := sqlmock.NewRows([]string{"id", "displayed_name", "service_name", "status", "created_at", "updated_at"}).
					AddRow("invalid_id", "Web Server", "nginx", "Running", fixedTime, fixedTime)
				mock.ExpectQuery(regexp.QuoteMeta(getServerQuery)).
					WithArgs(testServiceID, testServerID, testUserID).
					WillReturnRows(rows)
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				assert.NotNil(t, err)
			},
			validate: func(t *testing.T, result *models.Service) {
				assert.Nil(t, result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tt.mockSetup(mock)

			pg := &PgStorage{DB: db}

			result, err := pg.GetService(context.Background(), tt.serverID, tt.serviceID, tt.userID)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorAssertion != nil {
					tt.errorAssertion(t, err)
				}
			} else {
				assert.NoError(t, err)
			}

			tt.validate(t, result)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestListServices Проверяет получение списка служб сервера пользователя.
func TestListServices(t *testing.T) {
	fixedTime := time.Now()
	testUserID := int64(1)
	testServerID := int64(100)

	checkOwnershipQuery := `SELECT EXISTS(
						  	SELECT 1 FROM servers
                            WHERE id = $1 AND user_id = $2
                          )`

	getServicesQuery := `SELECT id, displayed_name, service_name, status, created_at, updated_at
                         FROM services 
                         WHERE server_id = $1
                         ORDER BY service_name`

	tests := []struct {
		name           string                                       // название теста
		serverID       int64                                        // ID сервера
		userID         int64                                        // ID пользователя
		mockSetup      func(mock sqlmock.Sqlmock)                   // настройка мока
		expectError    bool                                         // ожидается ли ошибка
		errorAssertion func(t *testing.T, err error)                // дополнительная проверка ошибки
		validate       func(t *testing.T, result []*models.Service) // валидация результата
	}{
		{
			name:     "успешное получение списка служб",
			serverID: testServerID,
			userID:   testUserID,
			mockSetup: func(mock sqlmock.Sqlmock) {
				// ожидаем проверку владения сервером
				ownershipRows := sqlmock.NewRows([]string{"exists"}).AddRow(true)
				mock.ExpectQuery(regexp.QuoteMeta(checkOwnershipQuery)).
					WithArgs(testServerID, testUserID).
					WillReturnRows(ownershipRows)

				// ожидаем запрос списка служб
				servicesRows := sqlmock.NewRows([]string{"id", "displayed_name", "service_name", "status", "created_at", "updated_at"}).
					AddRow(1, "Application Service", "AppService", "Running", fixedTime, fixedTime).
					AddRow(2, "Database Service", "DbService", "Stopped", fixedTime, fixedTime).
					AddRow(3, "Web Service", "WebService", "Running", fixedTime, fixedTime)
				mock.ExpectQuery(regexp.QuoteMeta(getServicesQuery)).
					WithArgs(testServerID).
					WillReturnRows(servicesRows)
			},
			expectError: false,
			validate: func(t *testing.T, result []*models.Service) {
				assert.Len(t, result, 3)
				assert.Equal(t, int64(1), result[0].ID)
				assert.Equal(t, "Application Service", result[0].DisplayedName)
				assert.Equal(t, "AppService", result[0].ServiceName)
				assert.Equal(t, "Running", result[0].Status)
				assert.Equal(t, fixedTime, result[0].CreatedAt)
				assert.Equal(t, fixedTime, result[0].UpdatedAt)

				assert.Equal(t, int64(2), result[1].ID)
				assert.Equal(t, "Database Service", result[1].DisplayedName)
				assert.Equal(t, "Stopped", result[1].Status)

				assert.Equal(t, int64(3), result[2].ID)
				assert.Equal(t, "Web Service", result[2].DisplayedName)
			},
		},
		{
			name:     "пустой список служб",
			serverID: testServerID,
			userID:   testUserID,
			mockSetup: func(mock sqlmock.Sqlmock) {
				// ожидаем проверку владения сервером
				ownershipRows := sqlmock.NewRows([]string{"exists"}).AddRow(true)
				mock.ExpectQuery(regexp.QuoteMeta(checkOwnershipQuery)).
					WithArgs(testServerID, testUserID).
					WillReturnRows(ownershipRows)

				// ожидаем запрос списка служб - пустой результат
				servicesRows := sqlmock.NewRows([]string{"id", "displayed_name", "service_name", "status", "created_at", "updated_at"})
				mock.ExpectQuery(regexp.QuoteMeta(getServicesQuery)).
					WithArgs(testServerID).
					WillReturnRows(servicesRows)
			},
			expectError: false,
			validate: func(t *testing.T, result []*models.Service) {
				assert.Empty(t, result)
			},
		},
		{
			name:     "ошибка - сервер не найден или не принадлежит пользователю",
			serverID: testServerID,
			userID:   testUserID,
			mockSetup: func(mock sqlmock.Sqlmock) {
				// ожидаем проверку владения сервером - возвращаем false
				ownershipRows := sqlmock.NewRows([]string{"exists"}).AddRow(false)
				mock.ExpectQuery(regexp.QuoteMeta(checkOwnershipQuery)).
					WithArgs(testServerID, testUserID).
					WillReturnRows(ownershipRows)
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				var notFoundErr *errs.ErrServerNotFound
				assert.True(t, errors.As(err, &notFoundErr), "ошибка должна быть типа ErrServerNotFound")
			},
			validate: func(t *testing.T, result []*models.Service) {
				assert.Nil(t, result)
			},
		},
		{
			name:     "ошибка при проверке владения сервером",
			serverID: testServerID,
			userID:   testUserID,
			mockSetup: func(mock sqlmock.Sqlmock) {
				// симулируем ошибку при проверке владения
				mock.ExpectQuery(regexp.QuoteMeta(checkOwnershipQuery)).
					WithArgs(testServerID, testUserID).
					WillReturnError(errors.New("database error"))
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "ошибка при проверке владения сервером")
			},
			validate: func(t *testing.T, result []*models.Service) {
				assert.Nil(t, result)
			},
		},
		{
			name:     "ошибка при получении списка служб",
			serverID: testServerID,
			userID:   testUserID,
			mockSetup: func(mock sqlmock.Sqlmock) {
				// ожидаем проверку владения сервером
				ownershipRows := sqlmock.NewRows([]string{"exists"}).AddRow(true)
				mock.ExpectQuery(regexp.QuoteMeta(checkOwnershipQuery)).
					WithArgs(testServerID, testUserID).
					WillReturnRows(ownershipRows)

				// симулируем ошибку при запросе списка служб
				mock.ExpectQuery(regexp.QuoteMeta(getServicesQuery)).
					WithArgs(testServerID).
					WillReturnError(errors.New("database error"))
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "ошибка при получении списка служб сервера пользователя")
			},
			validate: func(t *testing.T, result []*models.Service) {
				assert.Nil(t, result)
			},
		},
		{
			name:     "ошибка сканирования строки",
			serverID: testServerID,
			userID:   testUserID,
			mockSetup: func(mock sqlmock.Sqlmock) {
				// ожидаем проверку владения сервером
				ownershipRows := sqlmock.NewRows([]string{"exists"}).AddRow(true)
				mock.ExpectQuery(regexp.QuoteMeta(checkOwnershipQuery)).
					WithArgs(testServerID, testUserID).
					WillReturnRows(ownershipRows)

				// возвращаем строку с неправильным типом данных
				servicesRows := sqlmock.NewRows([]string{"id", "displayed_name", "service_name", "status", "created_at", "updated_at"}).
					AddRow("invalid_id", "Service", "SvcName", "Running", fixedTime, fixedTime)
				mock.ExpectQuery(regexp.QuoteMeta(getServicesQuery)).
					WithArgs(testServerID).
					WillReturnRows(servicesRows)
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				assert.NotNil(t, err)
			},
			validate: func(t *testing.T, result []*models.Service) {
				assert.Nil(t, result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tt.mockSetup(mock)

			pg := &PgStorage{DB: db}

			result, err := pg.ListServices(context.Background(), tt.serverID, tt.userID)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorAssertion != nil {
					tt.errorAssertion(t, err)
				}
			} else {
				assert.NoError(t, err)
			}

			tt.validate(t, result)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestCreateUser Проверяет создание пользователя.
func TestCreateUser(t *testing.T) {
	createUserQuery := `INSERT INTO users (login, password) VALUES ($1, $2)`

	tests := []struct {
		name           string                     // название теста
		user           *models.User               // данные пользователя
		mockSetup      func(mock sqlmock.Sqlmock) // настройка мока
		expectError    bool                       // ожидается ли ошибка
		errorAssertion func(t *testing.T, err error)
	}{
		{
			name: "успешное создание пользователя",
			user: &models.User{
				Login:    "testuser",
				Password: "password123",
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta(createUserQuery)).
					WithArgs("testuser", sqlmock.AnyArg()). // пароль хэшируется
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			expectError: false,
		},
		{
			name: "ошибка - логин уже занят",
			user: &models.User{
				Login:    "existinguser",
				Password: "password123",
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta(createUserQuery)).
					WithArgs("existinguser", sqlmock.AnyArg()).
					WillReturnError(&pgconn.PgError{Code: "23505"})
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				var loginTakenErr *errs.ErrLoginIsTaken
				assert.True(t, errors.As(err, &loginTakenErr), "ошибка должна быть типа ErrLoginIsTaken")
			},
		},
		{
			name: "общая ошибка базы данных",
			user: &models.User{
				Login:    "testuser",
				Password: "password123",
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta(createUserQuery)).
					WithArgs("testuser", sqlmock.AnyArg()).
					WillReturnError(errors.New("database error"))
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "ошибка создания пользователя")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tt.mockSetup(mock)

			pg := &PgStorage{DB: db}

			err = pg.CreateUser(context.Background(), tt.user)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorAssertion != nil {
					tt.errorAssertion(t, err)
				}
			} else {
				assert.NoError(t, err)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestGetUser Проверяет получение пользователя по логину и паролю.
func TestGetUser(t *testing.T) {
	// cоздаем хэшированный пароль для тестов
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("correctpassword"), bcrypt.DefaultCost)

	getUserQuery := `SELECT id, login, password 
					 FROM users 
					 WHERE login = $1`

	tests := []struct {
		name           string                                  // название теста
		user           *models.User                            // входные данные
		mockSetup      func(mock sqlmock.Sqlmock)              // настройка мока
		expectError    bool                                    // ожидается ли ошибка
		errorAssertion func(t *testing.T, err error)           // дополнительная проверка ошибки
		validate       func(t *testing.T, result *models.User) // валидация результата
	}{
		{
			name: "успешная авторизация пользователя",
			user: &models.User{
				Login:    "testuser",
				Password: "correctpassword",
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "login", "password"}).
					AddRow(1, "testuser", string(hashedPassword))
				mock.ExpectQuery(regexp.QuoteMeta(getUserQuery)).
					WithArgs("testuser").
					WillReturnRows(rows)
			},
			expectError: false,
			validate: func(t *testing.T, result *models.User) {
				assert.NotNil(t, result)
				assert.Equal(t, int64(1), result.ID)
				assert.Equal(t, "testuser", result.Login)
			},
		},
		{
			name: "ошибка - пользователь не найден",
			user: &models.User{
				Login:    "nonexistent",
				Password: "password",
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(getUserQuery)).
					WithArgs("nonexistent").
					WillReturnError(sql.ErrNoRows)
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				var wrongPassErr *errs.ErrWrongLoginOrPassword
				assert.True(t, errors.As(err, &wrongPassErr), "ошибка должна быть типа ErrWrongLoginOrPassword")
			},
			validate: func(t *testing.T, result *models.User) {
				assert.Nil(t, result)
			},
		},
		{
			name: "ошибка - неверный пароль",
			user: &models.User{
				Login:    "testuser",
				Password: "wrongpassword",
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "login", "password"}).
					AddRow(1, "testuser", string(hashedPassword))
				mock.ExpectQuery(regexp.QuoteMeta(getUserQuery)).
					WithArgs("testuser").
					WillReturnRows(rows)
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				var wrongPassErr *errs.ErrWrongLoginOrPassword
				assert.True(t, errors.As(err, &wrongPassErr), "ошибка должна быть типа ErrWrongLoginOrPassword")
			},
			validate: func(t *testing.T, result *models.User) {
				assert.Nil(t, result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tt.mockSetup(mock)

			pg := &PgStorage{DB: db}

			result, err := pg.GetUser(context.Background(), tt.user)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorAssertion != nil {
					tt.errorAssertion(t, err)
				}
			} else {
				assert.NoError(t, err)
			}

			tt.validate(t, result)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestAddService Проверяет добавление службы на сервер, принадлежащий пользователю.
func TestAddService(t *testing.T) {
	fixedTime := time.Now()
	testServerID := int64(100)
	testUserID := int64(1)
	//aesKey := []byte("12345678901234567890123456789012")

	fingerprintQuery := `SELECT fingerprint 
                    	 FROM servers 
                    	 WHERE id = $1 
                    	   AND user_id = $2`

	statusLookupQuery := `SELECT status, updated_at
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

	insertQuery := `INSERT INTO services (server_id, displayed_name, service_name, status, updated_at)
        			VALUES ($1, $2, $3, $4, $5)
        			RETURNING id, created_at, updated_at`

	tests := []struct {
		name           string
		serverID       int64
		userID         int64
		inputService   models.Service
		mockSetup      func(mock sqlmock.Sqlmock)
		expectError    bool
		errorAssertion func(t *testing.T, err error)
		validate       func(t *testing.T, result *models.Service)
	}{
		{
			name:     "успешное добавление новой службы без предыдущего статуса",
			serverID: testServerID, userID: testUserID,
			inputService: models.Service{
				DisplayedName: "App Service", ServiceName: "app", Status: "Running", UpdatedAt: fixedTime,
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				// запрос fingerprint
				mock.ExpectQuery(regexp.QuoteMeta(fingerprintQuery)).
					WithArgs(testServerID, testUserID).
					WillReturnRows(sqlmock.NewRows([]string{"fingerprint"}).
						AddRow(uuid.New()))
				// статус не найден (службы еще ни у одного пользователя не добавлено)
				mock.ExpectQuery(regexp.QuoteMeta(statusLookupQuery)).
					WithArgs(sqlmock.AnyArg(), testServerID, "app").
					WillReturnError(sql.ErrNoRows)
				// вставка
				mock.ExpectQuery(regexp.QuoteMeta(insertQuery)).
					WithArgs(testServerID, "App Service", "app", "Running", fixedTime).
					WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).
						AddRow(int64(1), fixedTime, fixedTime))
				mock.ExpectCommit()
			},
			expectError: false,
			validate: func(t *testing.T, result *models.Service) {
				assert.NotNil(t, result)
				assert.Equal(t, int64(1), result.ID)
				assert.Equal(t, "Running", result.Status)
			},
		},
		{
			name:     "успешное добавление со статусом из другого сервера",
			serverID: testServerID, userID: testUserID,
			inputService: models.Service{
				DisplayedName: "Cache", ServiceName: "redis", Status: "UNKNOWN", UpdatedAt: fixedTime,
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery(regexp.QuoteMeta(fingerprintQuery)).
					WithArgs(testServerID, testUserID).
					WillReturnRows(sqlmock.NewRows([]string{"fingerprint"}).
						AddRow(uuid.New()))
				// статус был найден (кто-то из пользователей уже добавил такую службу на свой сервер, совпадающий по fingerprint)
				mock.ExpectQuery(regexp.QuoteMeta(statusLookupQuery)).
					WithArgs(sqlmock.AnyArg(), testServerID, "redis").
					WillReturnRows(sqlmock.NewRows([]string{"status", "updated_at"}).
						AddRow("Stopped", fixedTime))
				mock.ExpectQuery(regexp.QuoteMeta(insertQuery)).
					WithArgs(testServerID, "Cache", "redis", "Stopped", fixedTime).
					WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).
						AddRow(int64(2), fixedTime, fixedTime))
				mock.ExpectCommit()
			},
			expectError: false,
			validate: func(t *testing.T, result *models.Service) {
				assert.Equal(t, "Stopped", result.Status)
			},
		},
		{
			name:     "ошибка начала транзакции",
			serverID: testServerID, userID: testUserID,
			inputService: models.Service{ServiceName: "app"},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin().WillReturnError(errors.New("tx error"))
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "не удалось начать транзакцию добавления службы")
			},
		},
		{
			name:     "ошибка получения сервера",
			serverID: testServerID, userID: testUserID,
			inputService: models.Service{ServiceName: "app"},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery(regexp.QuoteMeta(fingerprintQuery)).
					WithArgs(testServerID, testUserID).
					WillReturnError(sql.ErrNoRows)
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				var notFound *errs.ErrServerNotFound
				assert.True(t, errors.As(err, &notFound))
			},
		},
		{
			name:     "ошибка на вставке дубликата службы",
			serverID: testServerID, userID: testUserID,
			inputService: models.Service{DisplayedName: "App", ServiceName: "app", Status: "Running", UpdatedAt: fixedTime},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery(regexp.QuoteMeta(fingerprintQuery)).
					WithArgs(testServerID, testUserID).
					WillReturnRows(sqlmock.NewRows([]string{"fingerprint"}).AddRow(uuid.New()))
				mock.ExpectQuery(regexp.QuoteMeta(statusLookupQuery)).
					WithArgs(sqlmock.AnyArg(), testServerID, "app").
					WillReturnError(sql.ErrNoRows)
				mock.ExpectQuery(regexp.QuoteMeta(insertQuery)).
					WillReturnError(&pgconn.PgError{Code: "23505"})
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				var dupErr *errs.ErrDuplicatedService
				assert.True(t, errors.As(err, &dupErr))
			},
		},
		{
			name:     "общая ошибка при добавлении службы",
			serverID: testServerID, userID: testUserID,
			inputService: models.Service{DisplayedName: "App", ServiceName: "app", Status: "Running", UpdatedAt: fixedTime},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery(regexp.QuoteMeta(fingerprintQuery)).
					WithArgs(testServerID, testUserID).
					WillReturnRows(sqlmock.NewRows([]string{"fingerprint"}).AddRow(uuid.New()))
				mock.ExpectQuery(regexp.QuoteMeta(statusLookupQuery)).
					WithArgs(sqlmock.AnyArg(), testServerID, "app").
					WillReturnError(errors.New("status error"))
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "ошибка при проверке статуса других серверов")
			},
		},
		{
			name:     "ошибка коммита транзакции",
			serverID: testServerID, userID: testUserID,
			inputService: models.Service{DisplayedName: "App", ServiceName: "app", Status: "Running", UpdatedAt: fixedTime},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery(regexp.QuoteMeta(fingerprintQuery)).
					WithArgs(testServerID, testUserID).
					WillReturnRows(sqlmock.NewRows([]string{"fingerprint"}).AddRow(uuid.New()))
				mock.ExpectQuery(regexp.QuoteMeta(statusLookupQuery)).
					WithArgs(sqlmock.AnyArg(), testServerID, "app").
					WillReturnError(sql.ErrNoRows)
				mock.ExpectQuery(regexp.QuoteMeta(insertQuery)).
					WithArgs(testServerID, "App", "app", "Running", fixedTime).
					WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).
						AddRow(3, fixedTime, fixedTime))
				mock.ExpectCommit().WillReturnError(errors.New("commit error"))
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "ошибка при коммите транзакции добавления службы")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tt.mockSetup(mock)

			pg := &PgStorage{DB: db}

			result, err := pg.AddService(context.Background(), tt.serverID, tt.userID, tt.inputService)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorAssertion != nil {
					tt.errorAssertion(t, err)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestDelService Проверяет удаление службы с сервера пользователя.
func TestDelService(t *testing.T) {
	testUserID := int64(1)
	testServerID := int64(100)
	testServiceID := int64(10)

	deleteQuery := `DELETE FROM services 
              	 WHERE id = $1 
                 	AND server_id = $2
                	AND server_id IN (
                        SELECT id FROM servers 
                        WHERE user_id = $3
                 )`

	tests := []struct {
		name           string
		serverID       int64
		serviceID      int64
		userID         int64
		mockSetup      func(mock sqlmock.Sqlmock)
		expectError    bool
		errorAssertion func(t *testing.T, err error)
	}{
		{
			name:      "успешное удаление службы",
			serverID:  testServerID,
			serviceID: testServiceID,
			userID:    testUserID,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta(deleteQuery)).
					WithArgs(testServiceID, testServerID, testUserID).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:      "ошибка - служба не найдена",
			serverID:  testServerID,
			serviceID: testServiceID,
			userID:    testUserID,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta(deleteQuery)).
					WithArgs(testServiceID, testServerID, testUserID).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				var notFoundErr *errs.ErrServiceNotFound
				assert.True(t, errors.As(err, &notFoundErr), "ошибка должна быть ErrServiceNotFound")
			},
		},
		{
			name:      "ошибка выполнения запроса",
			serverID:  testServerID,
			serviceID: testServiceID,
			userID:    testUserID,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta(deleteQuery)).
					WithArgs(testServiceID, testServerID, testUserID).
					WillReturnError(errors.New("exec error"))
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "exec error")
			},
		},
		{
			name:      "ошибка при чтении RowsAffected",
			serverID:  testServerID,
			serviceID: testServiceID,
			userID:    testUserID,
			mockSetup: func(mock sqlmock.Sqlmock) {
				// возвращаем результат, который даст ошибку RowsAffected
				res := sqlmock.NewErrorResult(errors.New("rows error"))
				mock.ExpectExec(regexp.QuoteMeta(deleteQuery)).
					WithArgs(testServiceID, testServerID, testUserID).
					WillReturnResult(res)
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "ошибка при выполнении запроса")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tt.mockSetup(mock)

			pg := &PgStorage{DB: db}

			err = pg.DelService(context.Background(), tt.serverID, tt.serviceID, tt.userID)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorAssertion != nil {
					tt.errorAssertion(t, err)
				}
			} else {
				assert.NoError(t, err)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestChangeServiceStatus Проверяет изменение статуса службы.
func TestChangeServiceStatus(t *testing.T) {
	testServerID := int64(100)

	changeServiceStatusQuery := `UPDATE services SET status = $1, updated_at = CURRENT_TIMESTAMP
              					 WHERE service_name = $2 
                				 AND server_id IN (
                    				SELECT id FROM servers 
                    				WHERE fingerprint = (SELECT fingerprint FROM servers WHERE id = $3)
                				 )`

	tests := []struct {
		name           string                     // название теста
		serverID       int64                      // ID сервера
		serviceName    string                     // имя службы
		status         string                     // новый статус
		mockSetup      func(mock sqlmock.Sqlmock) // настройка мока
		expectError    bool                       // ожидается ли ошибка
		errorAssertion func(t *testing.T, err error)
	}{
		{
			name:        "успешное изменение статуса службы",
			serverID:    testServerID,
			serviceName: "TestService",
			status:      "Running",
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta(changeServiceStatusQuery)).
					WithArgs("Running", "TestService", testServerID).
					WillReturnResult(sqlmock.NewResult(0, 1)) // 1 строка обновлена
			},
			expectError: false,
		},
		{
			name:        "ошибка - служба не найдена",
			serverID:    testServerID,
			serviceName: "NonexistentService",
			status:      "Running",
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta(changeServiceStatusQuery)).
					WithArgs("Running", "NonexistentService", testServerID).
					WillReturnResult(sqlmock.NewResult(0, 0)) // 0 строк обновлено
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "ошибка изменения статуса службы")
			},
		},
		{
			name:        "ошибка базы данных",
			serverID:    testServerID,
			serviceName: "TestService",
			status:      "Running",
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta(changeServiceStatusQuery)).
					WithArgs("Running", "TestService", testServerID).
					WillReturnError(errors.New("database error"))
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "database error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tt.mockSetup(mock)

			pg := &PgStorage{DB: db}

			err = pg.ChangeServiceStatus(context.Background(), tt.serverID, tt.serviceName, tt.status)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorAssertion != nil {
					tt.errorAssertion(t, err)
				}
			} else {
				assert.NoError(t, err)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestBatchChangeServiceStatus Проверяет массовое изменение статуса служб.
func TestBatchChangeServiceStatus(t *testing.T) {
	fixedTime := time.Now()
	testServerID := int64(100)

	updateQuery := `UPDATE services SET status = $1, updated_at = $2
                    WHERE service_name = $3 
                    AND server_id IN (
                	    SELECT id FROM servers 
                	    WHERE fingerprint = (SELECT fingerprint FROM servers WHERE id = $4)
                    )`

	tests := []struct {
		name           string                     // название теста
		serverID       int64                      // ID сервера
		servicesBatch  []*models.Service          // батч служб для обновления
		mockSetup      func(mock sqlmock.Sqlmock) // настройка мока
		expectError    bool                       // ожидается ли ошибка
		errorAssertion func(t *testing.T, err error)
	}{
		{
			name:     "успешное обновление батча служб",
			serverID: testServerID,
			servicesBatch: []*models.Service{
				{ServiceName: "nginx", Status: "Running", UpdatedAt: fixedTime},
				{ServiceName: "postgresql", Status: "Running", UpdatedAt: fixedTime},
				{ServiceName: "redis", Status: "Stopped", UpdatedAt: fixedTime},
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				// ожидаем начало транзакции
				mock.ExpectBegin()

				// ожидаем подготовку запроса
				mock.ExpectPrepare(regexp.QuoteMeta(updateQuery))

				// ожидаем выполнение для каждой службы
				mock.ExpectExec(regexp.QuoteMeta(updateQuery)).
					WithArgs("Running", fixedTime, "nginx", testServerID).
					WillReturnResult(sqlmock.NewResult(0, 1)) // 1 строка обновлена

				mock.ExpectExec(regexp.QuoteMeta(updateQuery)).
					WithArgs("Running", fixedTime, "postgresql", testServerID).
					WillReturnResult(sqlmock.NewResult(0, 1))

				mock.ExpectExec(regexp.QuoteMeta(updateQuery)).
					WithArgs("Stopped", fixedTime, "redis", testServerID).
					WillReturnResult(sqlmock.NewResult(0, 1))

				// ожидаем коммит транзакции
				mock.ExpectCommit()
			},
			expectError: false,
		},
		{
			name:     "ошибка создания транзакции",
			serverID: testServerID,
			servicesBatch: []*models.Service{
				{ServiceName: "nginx", Status: "Running", UpdatedAt: fixedTime},
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin().WillReturnError(errors.New("transaction error"))
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "ошибка создания транзакции")
			},
		},
		{
			name:     "ошибка подготовки запроса",
			serverID: testServerID,
			servicesBatch: []*models.Service{
				{ServiceName: "nginx", Status: "Running", UpdatedAt: fixedTime},
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectPrepare(regexp.QuoteMeta(updateQuery)).WillReturnError(errors.New("prepare error"))
				mock.ExpectRollback()
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "ошибка подготовки запроса")
			},
		},
		{
			name:     "ошибка выполнения запроса для службы",
			serverID: testServerID,
			servicesBatch: []*models.Service{
				{ServiceName: "nginx", Status: "Running", UpdatedAt: fixedTime},
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectPrepare(regexp.QuoteMeta(updateQuery))
				mock.ExpectExec(regexp.QuoteMeta(updateQuery)).
					WithArgs("Running", fixedTime, "nginx", testServerID).
					WillReturnError(errors.New("exec error"))
				mock.ExpectRollback()
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "exec error")
			},
		},
		{
			name:     "ошибка - служба не найдена (0 строк обновлено)",
			serverID: testServerID,
			servicesBatch: []*models.Service{
				{ServiceName: "nonexistent", Status: "Running", UpdatedAt: fixedTime},
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectPrepare(regexp.QuoteMeta(updateQuery))
				mock.ExpectExec(regexp.QuoteMeta(updateQuery)).
					WithArgs("Running", fixedTime, "nonexistent", testServerID).
					WillReturnResult(sqlmock.NewResult(0, 0)) // 0 строк обновлено
				mock.ExpectRollback()
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "ошибка изменения статуса службы")
				assert.Contains(t, err.Error(), "nonexistent")
			},
		},
		{
			name:     "ошибка коммита транзакции",
			serverID: testServerID,
			servicesBatch: []*models.Service{
				{ServiceName: "nginx", Status: "Running", UpdatedAt: fixedTime},
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectPrepare(regexp.QuoteMeta(updateQuery))
				mock.ExpectExec(regexp.QuoteMeta(updateQuery)).
					WithArgs("Running", fixedTime, "nginx", testServerID).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit().WillReturnError(errors.New("commit error"))
				// убираем ExpectRollback()
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "ошибка при коммите транзакции")
			},
		},
		{
			name:     "успешное обновление одной службы",
			serverID: testServerID,
			servicesBatch: []*models.Service{
				{ServiceName: "nginx", Status: "Stopped", UpdatedAt: fixedTime},
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectPrepare(regexp.QuoteMeta(updateQuery))
				mock.ExpectExec(regexp.QuoteMeta(updateQuery)).
					WithArgs("Stopped", fixedTime, "nginx", testServerID).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			},
			expectError: false,
		},
		{
			name:          "пустой батч служб",
			serverID:      testServerID,
			servicesBatch: []*models.Service{},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectPrepare(regexp.QuoteMeta(updateQuery))
				// не ожидаем никаких Exec, так как батч пустой
				mock.ExpectCommit()
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tt.mockSetup(mock)

			pg := &PgStorage{DB: db}

			err = pg.BatchChangeServiceStatus(context.Background(), tt.serverID, tt.servicesBatch)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorAssertion != nil {
					tt.errorAssertion(t, err)
				}
			} else {
				assert.NoError(t, err)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestListUsers Проверяет получение списка всех пользователей.
func TestListUsers(t *testing.T) {
	listUsersQuery := `SELECT id, login FROM users`

	tests := []struct {
		name           string                                    // название теста
		mockSetup      func(mock sqlmock.Sqlmock)                // настройка мока
		expectError    bool                                      // ожидается ли ошибка
		errorAssertion func(t *testing.T, err error)             // дополнительная проверка ошибки
		validate       func(t *testing.T, result []*models.User) // валидация результата
	}{
		{
			name: "успешное получение списка пользователей",
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "login"}).
					AddRow(1, "user1").
					AddRow(2, "user2").
					AddRow(3, "user3")
				mock.ExpectQuery(regexp.QuoteMeta(listUsersQuery)).
					WillReturnRows(rows)
			},
			expectError: false,
			validate: func(t *testing.T, result []*models.User) {
				assert.NotNil(t, result)
				assert.Len(t, result, 3)
				assert.Equal(t, "user1", result[0].Login)
				assert.Equal(t, "user2", result[1].Login)
				assert.Equal(t, "user3", result[2].Login)
			},
		},
		{
			name: "пустой список пользователей",
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "login"})
				mock.ExpectQuery(regexp.QuoteMeta(listUsersQuery)).
					WillReturnRows(rows)
			},
			expectError: false,
			validate: func(t *testing.T, result []*models.User) {
				assert.Empty(t, result)
			},
		},
		{
			name: "ошибка базы данных",
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(listUsersQuery)).
					WillReturnError(errors.New("database error"))
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "ошибка при выполнении запроса на получение списка пользователей")
			},
			validate: func(t *testing.T, result []*models.User) {
				assert.Nil(t, result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tt.mockSetup(mock)

			pg := &PgStorage{DB: db}

			result, err := pg.ListUsers(context.Background())

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorAssertion != nil {
					tt.errorAssertion(t, err)
				}
			} else {
				assert.NoError(t, err)
			}

			tt.validate(t, result)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestGetUserServiceStatuses Проверяет получение списка статусов служб пользователя.
func TestGetUserServiceStatuses(t *testing.T) {
	fixedTime := time.Now()
	testUserID := int64(1)

	getUserServiceStatusesQuery := `SELECT id, server_id, status, updated_at 
                          			FROM services
                          			WHERE server_id IN (SELECT id FROM servers WHERE user_id = $1)`

	tests := []struct {
		name           string                                             // название теста
		userID         int64                                              // ID пользователя
		mockSetup      func(mock sqlmock.Sqlmock)                         // настройка мока
		expectError    bool                                               // ожидается ли ошибка
		errorAssertion func(t *testing.T, err error)                      // дополнительная проверка ошибки
		validate       func(t *testing.T, result []*models.ServiceStatus) // валидация результата
	}{
		{
			name:   "успешное получение списка статусов служб",
			userID: testUserID,
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "server_id", "status", "updated_at"}).
					AddRow(1, 100, "Running", fixedTime).
					AddRow(2, 100, "Stopped", fixedTime).
					AddRow(3, 101, "Running", fixedTime)
				mock.ExpectQuery(regexp.QuoteMeta(getUserServiceStatusesQuery)).
					WithArgs(testUserID).
					WillReturnRows(rows)
			},
			expectError: false,
			validate: func(t *testing.T, result []*models.ServiceStatus) {
				assert.Len(t, result, 3)
				assert.Equal(t, int64(1), result[0].ID)
				assert.Equal(t, int64(100), result[0].ServerID)
				assert.Equal(t, "Running", result[0].Status)
				assert.Equal(t, fixedTime, result[0].UpdatedAt)

				assert.Equal(t, int64(2), result[1].ID)
				assert.Equal(t, "Stopped", result[1].Status)

				assert.Equal(t, int64(3), result[2].ID)
				assert.Equal(t, int64(101), result[2].ServerID)
			},
		},
		{
			name:   "пустой список статусов служб",
			userID: testUserID,
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "server_id", "status", "updated_at"})
				mock.ExpectQuery(regexp.QuoteMeta(getUserServiceStatusesQuery)).
					WithArgs(testUserID).
					WillReturnRows(rows)
			},
			expectError: false,
			validate: func(t *testing.T, result []*models.ServiceStatus) {
				assert.Empty(t, result)
			},
		},
		{
			name:   "ошибка выполнения запроса",
			userID: testUserID,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(getUserServiceStatusesQuery)).
					WithArgs(testUserID).
					WillReturnError(errors.New("database error"))
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "ошибка при выполнении запроса статусов служб пользователя")
			},
			validate: func(t *testing.T, result []*models.ServiceStatus) {
				assert.Nil(t, result)
			},
		},
		{
			name:   "ошибка сканирования строки",
			userID: testUserID,
			mockSetup: func(mock sqlmock.Sqlmock) {
				// Возвращаем строку с неправильным типом данных (строка вместо int64 для id)
				rows := sqlmock.NewRows([]string{"id", "server_id", "status", "updated_at"}).
					AddRow("invalid_id", 100, "Running", fixedTime)
				mock.ExpectQuery(regexp.QuoteMeta(getUserServiceStatusesQuery)).
					WithArgs(testUserID).
					WillReturnRows(rows)
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				assert.NotNil(t, err)
			},
			validate: func(t *testing.T, result []*models.ServiceStatus) {
				assert.Nil(t, result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tt.mockSetup(mock)

			pg := &PgStorage{DB: db}

			result, err := pg.GetUserServiceStatuses(context.Background(), tt.userID)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorAssertion != nil {
					tt.errorAssertion(t, err)
				}
			} else {
				assert.NoError(t, err)
			}

			tt.validate(t, result)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestPing Проверяет доступность PostgreSQL с таймаутом.
func TestPing(t *testing.T) {
	tests := []struct {
		name           string                        // название теста
		mockSetup      func(mock sqlmock.Sqlmock)    // настройка мока
		expectError    bool                          // ожидается ли ошибка
		errorAssertion func(t *testing.T, err error) // дополнительная проверка ошибки
	}{
		{
			name: "успешный ping",
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectPing()
			},
			expectError: false,
		},
		{
			name: "ping возвращает ошибку",
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectPing().WillReturnError(errors.New("connection failed"))
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "connection failed")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
			require.NoError(t, err)

			tt.mockSetup(mock)

			pg := &PgStorage{DB: db}

			err = pg.Ping(context.Background())

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorAssertion != nil {
					tt.errorAssertion(t, err)
				}
			} else {
				assert.NoError(t, err)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestClose Проверяет закрытие соединения с базой данных.
func TestClose(t *testing.T) {
	tests := []struct {
		name           string                        // название теста
		mockSetup      func(mock sqlmock.Sqlmock)    // настройка мока
		expectError    bool                          // ожидается ли ошибка
		errorAssertion func(t *testing.T, err error) // дополнительная проверка ошибки
	}{
		{
			name: "успешное закрытие соединения",
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectClose()
			},
			expectError: false,
		},
		{
			name: "ошибка при закрытии соединения",
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectClose().WillReturnError(errors.New("close error"))
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "ошибка закрытия БД PostgreSQL")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)

			tt.mockSetup(mock)

			pg := &PgStorage{DB: db}

			err = pg.Close()

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorAssertion != nil {
					tt.errorAssertion(t, err)
				}
			} else {
				assert.NoError(t, err)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestListServersAddresses Проверяет корректность работы метода PgStorage.ListServersAddresses.
func TestListServersAddresses(t *testing.T) {
	query := `SELECT id, address FROM servers ORDER BY id`

	tests := []struct {
		name           string
		mockSetup      func(mock sqlmock.Sqlmock)
		expectError    bool
		errorAssertion func(t *testing.T, err error)
		validate       func(t *testing.T, result []*models.ServerStatus)
	}{
		{
			name: "успешное получение списка серверов",
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "address"}).
					AddRow(int64(1), "10.0.0.1").
					AddRow(int64(2), "10.0.0.2").
					AddRow(int64(3), "10.0.0.3")
				mock.ExpectQuery(regexp.QuoteMeta(query)).
					WillReturnRows(rows)
			},
			expectError: false,
			validate: func(t *testing.T, result []*models.ServerStatus) {
				require.NotNil(t, result)
				assert.Len(t, result, 3)
				assert.Equal(t, int64(1), result[0].ServerID)
				assert.Equal(t, "10.0.0.1", result[0].Address)
				assert.Equal(t, int64(2), result[1].ServerID)
				assert.Equal(t, "10.0.0.2", result[1].Address)
				assert.Equal(t, int64(3), result[2].ServerID)
				assert.Equal(t, "10.0.0.3", result[2].Address)
			},
		},
		{
			name: "пустой список серверов",
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "address"})
				mock.ExpectQuery(regexp.QuoteMeta(query)).
					WillReturnRows(rows)
			},
			expectError: false,
			validate: func(t *testing.T, result []*models.ServerStatus) {
				assert.Empty(t, result)
			},
		},
		{
			name: "ошибка выполнения запроса к БД",
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(query)).
					WillReturnError(errors.New("database error"))
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "ошибка при получении всех серверов")
			},
			validate: func(t *testing.T, result []*models.ServerStatus) {
				assert.Nil(t, result)
			},
		},
		{
			name: "ошибка парсинга строки (неправильный тип id)",
			mockSetup: func(mock sqlmock.Sqlmock) {
				// id как строка — должен вызвать ошибку при Scan в int64
				rows := sqlmock.NewRows([]string{"id", "address"}).
					AddRow("not-int", "10.0.0.1")
				mock.ExpectQuery(regexp.QuoteMeta(query)).
					WillReturnRows(rows)
			},
			expectError: true,
			errorAssertion: func(t *testing.T, err error) {
				assert.Error(t, err)
			},
			validate: func(t *testing.T, result []*models.ServerStatus) {
				assert.Nil(t, result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tt.mockSetup(mock)

			pg := &PgStorage{DB: db}

			result, err := pg.ListServersAddresses(context.Background())

			if tt.expectError {
				require.Error(t, err)
				if tt.errorAssertion != nil {
					tt.errorAssertion(t, err)
				}
			} else {
				require.NoError(t, err)
			}

			if tt.validate != nil {
				tt.validate(t, result)
			}

			// убеждаемся, что все ожидания моков выполнены
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
