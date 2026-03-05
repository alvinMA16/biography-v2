package quote

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/peizhengma/biography-v2/internal/domain/quote"
	quoteRepo "github.com/peizhengma/biography-v2/internal/repository/quote"
)

var (
	ErrNotFound = quoteRepo.ErrNotFound
)

// Service 激励语服务
type Service struct {
	repo *quoteRepo.Repository
}

// New 创建 Service
func New(repo *quoteRepo.Repository) *Service {
	return &Service{repo: repo}
}

// Create 创建激励语
func (s *Service) Create(ctx context.Context, input *quote.CreateQuoteInput) (*quote.Quote, error) {
	q := &quote.Quote{
		ID:        uuid.New(),
		Content:   input.Content,
		Type:      input.Type,
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if input.IsActive != nil {
		q.IsActive = *input.IsActive
	}
	if input.ShowGreeting != nil {
		q.ShowGreeting = *input.ShowGreeting
	}

	if err := s.repo.Create(ctx, q); err != nil {
		return nil, err
	}

	return q, nil
}

// GetByID 根据 ID 获取激励语
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*quote.Quote, error) {
	q, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, quoteRepo.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return q, nil
}

// Update 更新激励语
func (s *Service) Update(ctx context.Context, id uuid.UUID, input *quote.UpdateQuoteInput) (*quote.Quote, error) {
	q, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, quoteRepo.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if input.Content != nil {
		q.Content = *input.Content
	}
	if input.Type != nil {
		q.Type = *input.Type
	}
	if input.IsActive != nil {
		q.IsActive = *input.IsActive
	}
	if input.ShowGreeting != nil {
		q.ShowGreeting = *input.ShowGreeting
	}

	if err := s.repo.Update(ctx, q); err != nil {
		if errors.Is(err, quoteRepo.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return q, nil
}

// Delete 删除激励语
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	err := s.repo.Delete(ctx, id)
	if err != nil {
		if errors.Is(err, quoteRepo.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

// List 获取激励语列表
func (s *Service) List(ctx context.Context, quoteType *quote.Type, activeOnly bool, page, pageSize int) ([]*quote.Quote, int, error) {
	offset := (page - 1) * pageSize
	return s.repo.List(ctx, quoteType, activeOnly, pageSize, offset)
}

// GetRandom 随机获取一条激励语
func (s *Service) GetRandom(ctx context.Context, quoteType quote.Type) (*quote.Quote, error) {
	q, err := s.repo.GetRandom(ctx, quoteType)
	if err != nil {
		if errors.Is(err, quoteRepo.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return q, nil
}

// GetGreetingQuotes 获取问候语列表
func (s *Service) GetGreetingQuotes(ctx context.Context) ([]*quote.Quote, error) {
	return s.repo.GetGreetingQuotes(ctx)
}
