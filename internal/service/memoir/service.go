package memoir

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/peizhengma/biography-v2/internal/domain/memoir"
	memoirRepo "github.com/peizhengma/biography-v2/internal/repository/memoir"
)

var (
	ErrNotFound = errors.New("memoir not found")
	ErrNotOwner = errors.New("not memoir owner")
)

// Service 回忆录服务
type Service struct {
	repo *memoirRepo.Repository
}

// New 创建回忆录服务
func New(repo *memoirRepo.Repository) *Service {
	return &Service{repo: repo}
}

// Create 创建回忆录
func (s *Service) Create(ctx context.Context, userID uuid.UUID, input *memoir.CreateMemoirInput) (*memoir.Memoir, error) {
	// 获取下一个排序值
	maxOrder, err := s.repo.GetMaxSortOrder(ctx, userID)
	if err != nil {
		maxOrder = 0
	}

	now := time.Now()
	m := &memoir.Memoir{
		ID:             uuid.New(),
		UserID:         userID,
		ConversationID: input.ConversationID,
		Title:          input.Title,
		Content:        input.Content,
		TimePeriod:     nilIfEmpty(input.TimePeriod),
		StartYear:      input.StartYear,
		EndYear:        input.EndYear,
		SortOrder:      maxOrder + 1,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.repo.Create(ctx, m); err != nil {
		return nil, err
	}

	return m, nil
}

// CreateFromGenerated 从 AI 生成的结果创建回忆录
func (s *Service) CreateFromGenerated(ctx context.Context, userID uuid.UUID, conversationID uuid.UUID, generated *memoir.GeneratedMemoir) (*memoir.Memoir, error) {
	input := &memoir.CreateMemoirInput{
		ConversationID: &conversationID,
		Title:          generated.Title,
		Content:        generated.Content,
		TimePeriod:     generated.TimePeriod,
		StartYear:      generated.StartYear,
		EndYear:        generated.EndYear,
	}

	return s.Create(ctx, userID, input)
}

// GetByID 获取回忆录
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*memoir.Memoir, error) {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, memoirRepo.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return m, nil
}

// GetByIDForUser 获取用户的回忆录（带权限检查）
func (s *Service) GetByIDForUser(ctx context.Context, id, userID uuid.UUID) (*memoir.Memoir, error) {
	m, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if m.UserID != userID {
		return nil, ErrNotOwner
	}

	return m, nil
}

// Update 更新回忆录
func (s *Service) Update(ctx context.Context, id, userID uuid.UUID, input *memoir.UpdateMemoirInput) (*memoir.Memoir, error) {
	m, err := s.GetByIDForUser(ctx, id, userID)
	if err != nil {
		return nil, err
	}

	// 更新字段
	if input.Title != nil {
		m.Title = *input.Title
	}
	if input.Content != nil {
		m.Content = *input.Content
	}
	if input.TimePeriod != nil {
		m.TimePeriod = input.TimePeriod
	}
	if input.StartYear != nil {
		m.StartYear = input.StartYear
	}
	if input.EndYear != nil {
		m.EndYear = input.EndYear
	}
	if input.SortOrder != nil {
		m.SortOrder = *input.SortOrder
	}

	if err := s.repo.Update(ctx, m); err != nil {
		return nil, err
	}

	return m, nil
}

// Delete 删除回忆录（软删除）
func (s *Service) Delete(ctx context.Context, id, userID uuid.UUID) error {
	m, err := s.GetByIDForUser(ctx, id, userID)
	if err != nil {
		return err
	}
	_ = m

	return s.repo.SoftDelete(ctx, id)
}

// List 获取回忆录列表
func (s *Service) List(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*memoir.Memoir, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	filter := memoir.ListMemoirsFilter{
		UserID: userID,
		Limit:  limit,
		Offset: offset,
	}

	return s.repo.List(ctx, filter)
}

// ListAll 获取用户所有回忆录（时间线排序）
func (s *Service) ListAll(ctx context.Context, userID uuid.UUID) ([]*memoir.Memoir, error) {
	return s.repo.ListByUserID(ctx, userID)
}

// ListByUserID 获取用户所有回忆录
func (s *Service) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*memoir.Memoir, error) {
	return s.repo.ListByUserID(ctx, userID)
}

// GetByConversationID 根据对话 ID 获取回忆录
func (s *Service) GetByConversationID(ctx context.Context, conversationID uuid.UUID) (*memoir.Memoir, error) {
	m, err := s.repo.GetByConversationID(ctx, conversationID)
	if err != nil {
		if errors.Is(err, memoirRepo.ErrNotFound) {
			return nil, nil // 没有对应回忆录不是错误
		}
		return nil, err
	}
	return m, nil
}

// ListByConversationID 根据对话 ID 获取所有回忆录
func (s *Service) ListByConversationID(ctx context.Context, conversationID uuid.UUID) ([]*memoir.Memoir, error) {
	return s.repo.ListByConversationID(ctx, conversationID)
}

// Count 获取用户回忆录数量
func (s *Service) Count(ctx context.Context, userID uuid.UUID) (int, error) {
	return s.repo.Count(ctx, userID)
}

// ReorderMemoirs 重新排序回忆录
func (s *Service) ReorderMemoirs(ctx context.Context, userID uuid.UUID, memoirIDs []uuid.UUID) error {
	// 验证所有回忆录属于该用户
	for _, id := range memoirIDs {
		_, err := s.GetByIDForUser(ctx, id, userID)
		if err != nil {
			return err
		}
	}

	// 更新排序
	orders := make(map[uuid.UUID]int)
	for i, id := range memoirIDs {
		orders[id] = i + 1
	}

	return s.repo.UpdateSortOrders(ctx, orders)
}

// GetMemoirTitles 获取用户回忆录标题列表（用于 AI 生成话题时避免重复）
func (s *Service) GetMemoirTitles(ctx context.Context, userID uuid.UUID) ([]string, error) {
	memoirs, err := s.repo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	titles := make([]string, len(memoirs))
	for i, m := range memoirs {
		titles[i] = m.Title
	}

	return titles, nil
}

// ============================================
// Admin 方法
// ============================================

// AdminList 获取回忆录列表（管理员，可跨用户）
func (s *Service) AdminList(ctx context.Context, limit, offset int, includeDeleted bool) ([]*memoir.Memoir, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return s.repo.ListAll(ctx, limit, offset, includeDeleted)
}

// AdminGetByID 管理员获取回忆录（不检查权限）
func (s *Service) AdminGetByID(ctx context.Context, id uuid.UUID) (*memoir.Memoir, error) {
	return s.GetByID(ctx, id)
}

// AdminUpdate 管理员更新回忆录（不检查权限）
func (s *Service) AdminUpdate(ctx context.Context, id uuid.UUID, input *memoir.UpdateMemoirInput) (*memoir.Memoir, error) {
	m, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if input.Title != nil {
		m.Title = *input.Title
	}
	if input.Content != nil {
		m.Content = *input.Content
	}
	if input.TimePeriod != nil {
		m.TimePeriod = input.TimePeriod
	}
	if input.StartYear != nil {
		m.StartYear = input.StartYear
	}
	if input.EndYear != nil {
		m.EndYear = input.EndYear
	}
	if input.SortOrder != nil {
		m.SortOrder = *input.SortOrder
	}

	if err := s.repo.Update(ctx, m); err != nil {
		return nil, err
	}

	return m, nil
}

// AdminDelete 管理员删除回忆录（软删除，不检查权限）
func (s *Service) AdminDelete(ctx context.Context, id uuid.UUID) error {
	_, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	return s.repo.SoftDelete(ctx, id)
}

// nilIfEmpty 空字符串转 nil
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
