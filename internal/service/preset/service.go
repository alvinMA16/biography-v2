package preset

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/peizhengma/biography-v2/internal/domain/preset"
	presetRepo "github.com/peizhengma/biography-v2/internal/repository/preset"
)

var (
	ErrNotFound = presetRepo.ErrNotFound
)

// Service 预设话题服务
type Service struct {
	repo *presetRepo.Repository
}

// New 创建 Service
func New(repo *presetRepo.Repository) *Service {
	return &Service{repo: repo}
}

// Create 创建预设话题
func (s *Service) Create(ctx context.Context, input *preset.CreateTopicInput) (*preset.Topic, error) {
	now := time.Now()
	t := &preset.Topic{
		ID:        uuid.New(),
		Topic:     input.Topic,
		Greeting:  input.Greeting,
		IsActive:  true,
		SortOrder: 0,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if input.ChatContext != "" {
		t.ChatContext = &input.ChatContext
	}
	if input.AgeStart != nil {
		t.AgeStart = input.AgeStart
	}
	if input.AgeEnd != nil {
		t.AgeEnd = input.AgeEnd
	}
	if input.IsActive != nil {
		t.IsActive = *input.IsActive
	}
	if input.SortOrder != nil {
		t.SortOrder = *input.SortOrder
	}

	if err := s.repo.Create(ctx, t); err != nil {
		return nil, err
	}

	return t, nil
}

// GetByID 根据 ID 获取预设话题
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*preset.Topic, error) {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, presetRepo.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return t, nil
}

// Update 更新预设话题
func (s *Service) Update(ctx context.Context, id uuid.UUID, input *preset.UpdateTopicInput) (*preset.Topic, error) {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, presetRepo.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if input.Topic != nil {
		t.Topic = *input.Topic
	}
	if input.Greeting != nil {
		t.Greeting = *input.Greeting
	}
	if input.ChatContext != nil {
		t.ChatContext = input.ChatContext
	}
	if input.AgeStart != nil {
		t.AgeStart = input.AgeStart
	}
	if input.AgeEnd != nil {
		t.AgeEnd = input.AgeEnd
	}
	if input.IsActive != nil {
		t.IsActive = *input.IsActive
	}
	if input.SortOrder != nil {
		t.SortOrder = *input.SortOrder
	}

	if err := s.repo.Update(ctx, t); err != nil {
		if errors.Is(err, presetRepo.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return t, nil
}

// Delete 删除预设话题
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	err := s.repo.Delete(ctx, id)
	if err != nil {
		if errors.Is(err, presetRepo.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

// List 获取预设话题列表
func (s *Service) List(ctx context.Context, activeOnly bool, page, pageSize int) ([]*preset.Topic, int, error) {
	filter := preset.ListTopicsFilter{
		ActiveOnly: activeOnly,
		Limit:      pageSize,
		Offset:     (page - 1) * pageSize,
	}
	return s.repo.List(ctx, filter)
}

// GetActiveTopics 获取所有激活的预设话题
func (s *Service) GetActiveTopics(ctx context.Context) ([]*preset.Topic, error) {
	return s.repo.GetActiveTopics(ctx)
}
