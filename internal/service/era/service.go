package era

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/peizhengma/biography-v2/internal/domain/era"
	eraRepo "github.com/peizhengma/biography-v2/internal/repository/era"
)

var (
	ErrNotFound = eraRepo.ErrNotFound
)

// Service 时代记忆预设服务
type Service struct {
	repo *eraRepo.Repository
}

// New 创建 Service
func New(repo *eraRepo.Repository) *Service {
	return &Service{repo: repo}
}

// Create 创建时代记忆预设
func (s *Service) Create(ctx context.Context, input *era.CreateMemoryPresetInput) (*era.MemoryPreset, error) {
	now := time.Now()
	m := &era.MemoryPreset{
		ID:        uuid.New(),
		StartYear: input.StartYear,
		EndYear:   input.EndYear,
		Content:   input.Content,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if input.Category != "" {
		m.Category = &input.Category
	}

	if err := s.repo.Create(ctx, m); err != nil {
		return nil, err
	}

	return m, nil
}

// GetByID 根据 ID 获取时代记忆预设
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*era.MemoryPreset, error) {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, eraRepo.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return m, nil
}

// Update 更新时代记忆预设
func (s *Service) Update(ctx context.Context, id uuid.UUID, input *era.UpdateMemoryPresetInput) (*era.MemoryPreset, error) {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, eraRepo.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if input.StartYear != nil {
		m.StartYear = *input.StartYear
	}
	if input.EndYear != nil {
		m.EndYear = *input.EndYear
	}
	if input.Category != nil {
		m.Category = input.Category
	}
	if input.Content != nil {
		m.Content = *input.Content
	}

	if err := s.repo.Update(ctx, m); err != nil {
		if errors.Is(err, eraRepo.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return m, nil
}

// Delete 删除时代记忆预设
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	err := s.repo.Delete(ctx, id)
	if err != nil {
		if errors.Is(err, eraRepo.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

// List 获取时代记忆预设列表
func (s *Service) List(ctx context.Context, category *string, page, pageSize int) ([]*era.MemoryPreset, int, error) {
	filter := era.ListMemoryPresetsFilter{
		Category: category,
		Limit:    pageSize,
		Offset:   (page - 1) * pageSize,
	}
	return s.repo.List(ctx, filter)
}

// GetByBirthYear 根据出生年份获取相关时代记忆
func (s *Service) GetByBirthYear(ctx context.Context, birthYear int) ([]*era.MemoryPreset, error) {
	return s.repo.GetByYearRange(ctx, birthYear)
}
