package models

import (
	"context"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/contextkeys"
)

// ContextCredentials Получение login, userID, serverID, serviceID из r.Context()
type ContextCredentials struct {
	Login     string
	UserID    int64
	ServerID  int64
	ServiceID int64
}

// GetContextCreds Вытаскивает данные из контекста и возвращает структуру.
func GetContextCreds(ctx context.Context) *ContextCredentials {
	creds := &ContextCredentials{}

	// Login (string)
	if v := ctx.Value(contextkeys.Login); v != nil {
		if login, ok := v.(string); ok {
			creds.Login = login
		}
	}

	// UserID (int64)
	if v := ctx.Value(contextkeys.ID); v != nil {
		if userID, ok := v.(int64); ok {
			creds.UserID = userID
		}
	}

	// ServerID (int64)
	if v := ctx.Value(contextkeys.ServerID); v != nil {
		if serverID, ok := v.(int64); ok {
			creds.ServerID = serverID
		}
	}

	// ServiceID (int64)
	if v := ctx.Value(contextkeys.ServiceID); v != nil {
		if ServiceID, ok := v.(int64); ok {
			creds.ServiceID = ServiceID
		}
	}

	return creds
}
