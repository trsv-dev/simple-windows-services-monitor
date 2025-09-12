package service_control

// пока этот Manager - избыточное архитектурное излишество

//////////////////////////////////////////////////////////

//import (
//	"context"
//	"fmt"
//)
//
//// Manager Слой бизнес-логики для работы со службами.
//type Manager interface {
//	StartService(ctx context.Context, serverID int, serviceName string) error
//	StopService(ctx context.Context, serverID int, serviceName string) error
//	RestartService(ctx context.Context, serverID int, serviceName string) error
//	GetStatus(ctx context.Context, serverID int, serviceName string) (string, error)
//}
//
//type WinRMManager struct {
//	client WinRMClient
//}
//
//func NewWinRMManager(client WinRMClient) *WinRMManager {
//	return &WinRMManager{client: client}
//}
//
//func (m *WinRMManager) StartService(ctx context.Context, serverID int, serviceName string) error {
//	_, err := m.client.RunCommand(ctx, fmt.Sprintf("sc start %s", serviceName))
//	return err
//}
//
//func (m *WinRMManager) StopService(ctx context.Context, serverID int, serviceName string) error {
//	_, err := m.client.RunCommand(ctx, fmt.Sprintf("sc stop %s", serviceName))
//	return err
//}
//
//func (m *WinRMManager) RestartService(ctx context.Context, serverID int, serviceName string) error {
//	if err := m.StopService(ctx, serverID, serviceName); err != nil {
//		return err
//	}
//	return m.StartService(ctx, serverID, serviceName)
//}
//
//func (m *WinRMManager) GetStatus(ctx context.Context, serverID int, serviceName string) (string, error) {
//	return m.client.RunCommand(ctx, fmt.Sprintf("sc query %s", serviceName))
//}
