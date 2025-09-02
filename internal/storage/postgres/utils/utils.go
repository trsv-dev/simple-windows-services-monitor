package utils

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/migrations"
)

// ApplyMigrations применяет все миграции из embed.FS
func ApplyMigrations(db *sql.DB) error {
	entries, err := migrations.Files.ReadDir(".")
	if err != nil {
		logger.Log.Error("Ошибка при чтении списка миграций", logger.String("err", err.Error()))
		return fmt.Errorf("не удалось прочитать список миграций: %w", err)
	}

	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".sql" || !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}

		sqlBytes, err := migrations.Files.ReadFile(e.Name())
		if err != nil {
			logger.Log.Error("Ошибка при чтении миграции", logger.String("err", err.Error()))
			return fmt.Errorf("не удалось прочитать миграцию %s: %w", e.Name(), err)
		}

		if _, err := db.Exec(string(sqlBytes)); err != nil {
			logger.Log.Error("Ошибка при выполнении миграции", logger.String("err", err.Error()))
			return fmt.Errorf("не удалось выполнить миграцию %s: %w", e.Name(), err)
		}

		logger.Log.Info("Миграция успешно применена", logger.String("success", e.Name()))
	}

	logger.Log.Info("Все UP-миграции успешно применены")
	return nil
}
