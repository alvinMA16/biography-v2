package welcome

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/peizhengma/biography-v2/internal/domain/welcome"
	welcomeRepo "github.com/peizhengma/biography-v2/internal/repository/welcome"
)

var (
	ErrNotFound = errors.New("welcome message not found")
)

// Service 欢迎语服务
type Service struct {
	repo *welcomeRepo.Repository
}

// New 创建 Service
func New(repo *welcomeRepo.Repository) *Service {
	return &Service{repo: repo}
}

// Create 创建欢迎语
func (s *Service) Create(ctx context.Context, input *welcome.CreateInput) (*welcome.Message, error) {
	now := time.Now()
	m := &welcome.Message{
		ID:           uuid.New(),
		Content:      input.Content,
		ShowGreeting: true,
		IsActive:     true,
		SortOrder:    0,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if input.ShowGreeting != nil {
		m.ShowGreeting = *input.ShowGreeting
	}
	if input.IsActive != nil {
		m.IsActive = *input.IsActive
	}
	if input.SortOrder != nil {
		m.SortOrder = *input.SortOrder
	}

	if err := s.repo.Create(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

// GetByID 根据 ID 获取欢迎语
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*welcome.Message, error) {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, welcomeRepo.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return m, nil
}

// Update 更新欢迎语
func (s *Service) Update(ctx context.Context, id uuid.UUID, input *welcome.UpdateInput) (*welcome.Message, error) {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, welcomeRepo.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if input.Content != nil {
		m.Content = *input.Content
	}
	if input.ShowGreeting != nil {
		m.ShowGreeting = *input.ShowGreeting
	}
	if input.IsActive != nil {
		m.IsActive = *input.IsActive
	}
	if input.SortOrder != nil {
		m.SortOrder = *input.SortOrder
	}

	if err := s.repo.Update(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

// Delete 删除欢迎语
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		if errors.Is(err, welcomeRepo.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

// List 获取欢迎语列表
func (s *Service) List(ctx context.Context, limit, offset int) ([]*welcome.Message, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return s.repo.List(ctx, limit, offset)
}

// GetActive 获取启用的欢迎语
func (s *Service) GetActive(ctx context.Context) ([]*welcome.Message, error) {
	return s.repo.GetActive(ctx)
}
