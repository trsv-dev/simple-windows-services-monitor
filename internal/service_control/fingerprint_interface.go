package service_control

import (
	"context"

	"github.com/google/uuid"
)

//go:generate mockgen -destination=mocks/mock_fingerprinter.go -package=mocks . Fingerprinter

// Fingerprinter Интерфейс для получения fingerprint сервера.
type Fingerprinter interface {
	GetFingerprint(ctx context.Context, address, username, password string) (uuid.UUID, error)
}
