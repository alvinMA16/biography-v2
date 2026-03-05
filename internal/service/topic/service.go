package topic

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/peizhengma/biography-v2/internal/domain/topic"
	topicRepo "github.com/peizhengma/biography-v2/internal/repository/topic"
)

var (
	ErrNotFound = errors.New("topic not found")
	ErrNotOwner = errors.New("not topic owner")
)

// Service 话题服务
type Service struct {
	repo *topicRepo.Repository
}

// New 创建话题服务
func New(repo *topicRepo.Repository) *Service {
	return &Service{repo: repo}
}

// Create 创建话题
func (s *Service) Create(ctx context.Context, userID uuid.UUID, input *topic.CreateTopicInput) (*topic.TopicCandidate, error) {
	source := input.Source
	if source == "" {
		source = topic.SourceManual
	}

	now := time.Now()
	t := &topic.TopicCandidate{
		ID:        uuid.New(),
		UserID:    userID,
		Title:     input.Title,
		Greeting:  nilIfEmpty(input.Greeting),
		Context:   nilIfEmpty(input.Context),
		Status:    topic.StatusApproved, // 手动创建的话题直接通过
		Source:    source,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.repo.Create(ctx, t); err != nil {
		return nil, err
	}

	return t, nil
}

// CreateFromGenerated 从 AI 生成的结果批量创建话题
func (s *Service) CreateFromGenerated(ctx context.Context, userID uuid.UUID, generated []topic.GeneratedTopic) ([]*topic.TopicCandidate, error) {
	now := time.Now()
	topics := make([]*topic.TopicCandidate, len(generated))

	for i, g := range generated {
		topics[i] = &topic.TopicCandidate{
			ID:        uuid.New(),
			UserID:    userID,
			Title:     g.Title,
			Greeting:  nilIfEmpty(g.Greeting),
			Context:   nilIfEmpty(g.Context),
			Status:    topic.StatusPending, // AI 生成的话题需要审核
			Source:    topic.SourceAI,
			CreatedAt: now,
			UpdatedAt: now,
		}
	}

	if err := s.repo.CreateBatch(ctx, topics); err != nil {
		return nil, err
	}

	return topics, nil
}

// GetByID 获取话题
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*topic.TopicCandidate, error) {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, topicRepo.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return t, nil
}

// GetByIDForUser 获取用户的话题（带权限检查）
func (s *Service) GetByIDForUser(ctx context.Context, id, userID uuid.UUID) (*topic.TopicCandidate, error) {
	t, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if t.UserID != userID {
		return nil, ErrNotOwner
	}

	return t, nil
}

// Update 更新话题
func (s *Service) Update(ctx context.Context, id, userID uuid.UUID, input *topic.UpdateTopicInput) (*topic.TopicCandidate, error) {
	t, err := s.GetByIDForUser(ctx, id, userID)
	if err != nil {
		return nil, err
	}

	if input.Title != nil {
		t.Title = *input.Title
	}
	if input.Greeting != nil {
		t.Greeting = input.Greeting
	}
	if input.Context != nil {
		t.Context = input.Context
	}
	if input.Status != nil {
		t.Status = *input.Status
	}

	if err := s.repo.Update(ctx, t); err != nil {
		return nil, err
	}

	return t, nil
}

// Delete 删除话题
func (s *Service) Delete(ctx context.Context, id, userID uuid.UUID) error {
	t, err := s.GetByIDForUser(ctx, id, userID)
	if err != nil {
		return err
	}
	_ = t

	return s.repo.Delete(ctx, id)
}

// List 获取话题列表
func (s *Service) List(ctx context.Context, userID uuid.UUID, status *topic.Status, limit, offset int) ([]*topic.TopicCandidate, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	return s.repo.List(ctx, userID, status, limit, offset)
}

// GetTopicOptions 获取可选话题（用于前端展示）
func (s *Service) GetTopicOptions(ctx context.Context, userID uuid.UUID, limit int) ([]topic.TopicOption, error) {
	if limit <= 0 {
		limit = 8
	}

	topics, err := s.repo.GetAvailableTopics(ctx, userID, limit)
	if err != nil {
		return nil, err
	}

	options := make([]topic.TopicOption, len(topics))
	for i, t := range topics {
		options[i] = topic.TopicOption{
			ID:       t.ID,
			Title:    t.Title,
			Greeting: deref(t.Greeting),
			Context:  deref(t.Context),
		}
	}

	return options, nil
}

// Approve 批准话题
func (s *Service) Approve(ctx context.Context, id, userID uuid.UUID) error {
	t, err := s.GetByIDForUser(ctx, id, userID)
	if err != nil {
		return err
	}
	_ = t

	return s.repo.UpdateStatus(ctx, id, topic.StatusApproved)
}

// Reject 拒绝话题
func (s *Service) Reject(ctx context.Context, id, userID uuid.UUID) error {
	t, err := s.GetByIDForUser(ctx, id, userID)
	if err != nil {
		return err
	}
	_ = t

	return s.repo.UpdateStatus(ctx, id, topic.StatusRejected)
}

// MarkAsUsed 标记话题为已使用
func (s *Service) MarkAsUsed(ctx context.Context, id uuid.UUID) error {
	return s.repo.UpdateStatus(ctx, id, topic.StatusUsed)
}

// GetPendingCount 获取待审核话题数量
func (s *Service) GetPendingCount(ctx context.Context, userID uuid.UUID) (int, error) {
	return s.repo.CountByStatus(ctx, userID, topic.StatusPending)
}

// GetAvailableCount 获取可用话题数量
func (s *Service) GetAvailableCount(ctx context.Context, userID uuid.UUID) (int, error) {
	return s.repo.CountByStatus(ctx, userID, topic.StatusApproved)
}

// GetExistingTitles 获取已有话题标题（用于 AI 生成时避免重复）
func (s *Service) GetExistingTitles(ctx context.Context, userID uuid.UUID) ([]string, error) {
	return s.repo.GetTopicTitles(ctx, userID)
}

// NeedsMoreTopics 检查是否需要生成更多话题
func (s *Service) NeedsMoreTopics(ctx context.Context, userID uuid.UUID, threshold int) (bool, error) {
	count, err := s.GetAvailableCount(ctx, userID)
	if err != nil {
		return false, err
	}
	return count < threshold, nil
}

// nilIfEmpty 空字符串转 nil
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// deref 安全解引用
func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
