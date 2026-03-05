package audit

import (
	"context"

	"github.com/google/uuid"
	"github.com/peizhengma/biography-v2/internal/domain/audit"
	auditRepo "github.com/peizhengma/biography-v2/internal/repository/audit"
)

// Service 审计日志服务
type Service struct {
	repo *auditRepo.Repository
}

// New 创建 Service
func New(repo *auditRepo.Repository) *Service {
	return &Service{repo: repo}
}

// Log 记录审计日志
func (s *Service) Log(ctx context.Context, input audit.CreateInput) error {
	return s.repo.Log(ctx, input)
}

// List 获取审计日志列表
func (s *Service) List(ctx context.Context, limit, offset int) ([]*audit.Log, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return s.repo.ListSimple(ctx, limit, offset)
}

// ListByFilter 根据过滤条件获取审计日志
func (s *Service) ListByFilter(ctx context.Context, filter audit.ListFilter) ([]*audit.Log, int, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	return s.repo.List(ctx, filter)
}

// GetByID 根据 ID 获取审计日志
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*audit.Log, error) {
	return s.repo.GetByID(ctx, id)
}

// LogUserAction 记录用户相关操作
func (s *Service) LogUserAction(ctx context.Context, adminID *uuid.UUID, action audit.Action, targetID uuid.UUID, targetLabel string, detail map[string]any, ip string) error {
	return s.Log(ctx, audit.CreateInput{
		AdminID:     adminID,
		Action:      action,
		TargetType:  audit.TargetTypeUser,
		TargetID:    &targetID,
		TargetLabel: targetLabel,
		Detail:      detail,
		IPAddress:   ip,
	})
}
