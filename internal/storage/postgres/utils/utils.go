package utils

import (
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/migrations"
)

// ApplyMigrations применяет все миграции из embed.FS
func ApplyMigrations(DatabaseURI string) error {
	// создаем источник миграций с использованием встроенной файловой системы
	// и указываем, что миграции находятся в папке "migrations" внутри этой системы.
	// iofs.New() возвращает объект, который предоставляет доступ к этим миграциям.
	d, err := iofs.New(migrations.Files, ".")
	if err != nil {
		logger.Log.Error("Ошибка подготовки встраивания миграций", logger.String("err", err.Error()))
		return fmt.Errorf("ошибка подготовки встраивания миграций: %w", err)
	}

	// создаем новый экземпляр миграции (мигратор), используя источник "iofs" (встраиваемая файловая система),
	// и передаем строку подключения к базе данных (dbCredentials).
	// Этот объект будет использоваться для выполнения миграций в базе данных.
	m, err := migrate.NewWithSourceInstance("iofs", d, DatabaseURI)
	if err != nil {
		logger.Log.Error("Ошибка подготовки миграций", logger.String("err", err.Error()))
		return fmt.Errorf("ошибка подготовки миграций: %w", err)
	}

	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil {
			logger.Log.Warn("Ошибка закрытия источника миграций", logger.String("err", srcErr.Error()))
		}
		if dbErr != nil {
			logger.Log.Warn("Ошибка закрытия соединения мигратора", logger.String("err", dbErr.Error()))
		}
	}()

	// применяем все миграции к базе данных. Если возникнут ошибки, они будут обработаны в следующем условии.
	err = m.Up()
	if err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			logger.Log.Info("Нет новых миграций", logger.String("info", err.Error()))
			return nil
		}
		logger.Log.Error("ошибка миграции", logger.String("err", err.Error()))
		return fmt.Errorf("ошибка применения миграции: %w", err)
	}

	logger.Log.Info("Миграции были применены")
	return nil
}
