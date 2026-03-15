package conversation

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/peizhengma/biography-v2/internal/domain/conversation"
	convRepo "github.com/peizhengma/biography-v2/internal/repository/conversation"
)

var (
	ErrNotFound         = errors.New("conversation not found")
	ErrNotOwner         = errors.New("not conversation owner")
	ErrAlreadyCompleted = errors.New("conversation already completed")
	ErrInvalidStatus    = errors.New("invalid status")
)

// Service 对话服务
type Service struct {
	repo *convRepo.Repository
}

// New 创建对话服务
func New(repo *convRepo.Repository) *Service {
	return &Service{repo: repo}
}

// Create 创建对话
func (s *Service) Create(ctx context.Context, userID uuid.UUID, input *conversation.CreateConversationInput) (*conversation.Conversation, error) {
	now := time.Now()
	mode := input.Mode
	if mode == "" {
		mode = conversation.ModeNormal
	}
	c := &conversation.Conversation{
		ID:        uuid.New(),
		UserID:    userID,
		Topic:     nilIfEmpty(input.Topic),
		Greeting:  nilIfEmpty(input.Greeting),
		Context:   nilIfEmpty(input.Context),
		Mode:      mode,
		Status:    conversation.StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.repo.Create(ctx, c); err != nil {
		return nil, err
	}

	return c, nil
}

// GetByID 获取对话
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*conversation.Conversation, error) {
	c, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, convRepo.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return c, nil
}

// GetByIDForUser 获取用户的对话（带权限检查）
func (s *Service) GetByIDForUser(ctx context.Context, id, userID uuid.UUID) (*conversation.Conversation, error) {
	c, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if c.UserID != userID {
		return nil, ErrNotOwner
	}

	return c, nil
}

// GetWithMessages 获取对话及消息
func (s *Service) GetWithMessages(ctx context.Context, id, userID uuid.UUID) (*conversation.Conversation, error) {
	c, err := s.repo.GetByIDWithMessages(ctx, id)
	if err != nil {
		if errors.Is(err, convRepo.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if c.UserID != userID {
		return nil, ErrNotOwner
	}

	return c, nil
}

// List 获取对话列表
func (s *Service) List(ctx context.Context, userID uuid.UUID, status *conversation.Status, limit, offset int) ([]*conversation.Conversation, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	filter := conversation.ListConversationsFilter{
		UserID: userID,
		Status: status,
		Limit:  limit,
		Offset: offset,
	}

	return s.repo.List(ctx, filter)
}

// GetActive 获取用户的活跃对话
func (s *Service) GetActive(ctx context.Context, userID uuid.UUID) (*conversation.Conversation, error) {
	c, err := s.repo.GetActiveByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, convRepo.ErrNotFound) {
			return nil, nil // 没有活跃对话不是错误
		}
		return nil, err
	}
	return c, nil
}

// Complete 完成对话
func (s *Service) Complete(ctx context.Context, id, userID uuid.UUID) error {
	c, err := s.GetByIDForUser(ctx, id, userID)
	if err != nil {
		return err
	}

	if c.Status == conversation.StatusCompleted {
		return ErrAlreadyCompleted
	}

	return s.repo.UpdateStatus(ctx, id, conversation.StatusCompleted)
}

// Abandon 放弃对话
func (s *Service) Abandon(ctx context.Context, id, userID uuid.UUID) error {
	c, err := s.GetByIDForUser(ctx, id, userID)
	if err != nil {
		return err
	}

	if c.Status != conversation.StatusActive {
		return ErrInvalidStatus
	}

	return s.repo.UpdateStatus(ctx, id, conversation.StatusAbandoned)
}

// UpdateSummary 更新对话摘要
func (s *Service) UpdateSummary(ctx context.Context, id uuid.UUID, summary string) error {
	return s.repo.UpdateSummary(ctx, id, summary)
}

// AddMessage 添加消息
func (s *Service) AddMessage(ctx context.Context, conversationID uuid.UUID, role, content string) (*conversation.Message, error) {
	msg := &conversation.Message{
		ID:             uuid.New(),
		ConversationID: conversationID,
		Role:           role,
		Content:        content,
		CreatedAt:      time.Now(),
	}

	if err := s.repo.AddMessage(ctx, msg); err != nil {
		return nil, err
	}

	return msg, nil
}

// AddUserMessage 添加用户消息
func (s *Service) AddUserMessage(ctx context.Context, conversationID uuid.UUID, content string) (*conversation.Message, error) {
	return s.AddMessage(ctx, conversationID, "user", content)
}

// AddAssistantMessage 添加助手消息
func (s *Service) AddAssistantMessage(ctx context.Context, conversationID uuid.UUID, content string) (*conversation.Message, error) {
	return s.AddMessage(ctx, conversationID, "assistant", content)
}

// GetMessages 获取对话消息
func (s *Service) GetMessages(ctx context.Context, conversationID, userID uuid.UUID, limit, offset int) ([]conversation.Message, error) {
	// 检查权限
	c, err := s.GetByIDForUser(ctx, conversationID, userID)
	if err != nil {
		return nil, err
	}
	_ = c

	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	return s.repo.GetMessages(ctx, conversationID, offset, limit)
}

// GetAllMessages 获取所有消息（内部使用，不检查权限）
func (s *Service) GetAllMessages(ctx context.Context, conversationID uuid.UUID) ([]conversation.Message, error) {
	return s.repo.GetMessages(ctx, conversationID, 0, 10000)
}

// GetMessageCount 获取消息数量
func (s *Service) GetMessageCount(ctx context.Context, conversationID uuid.UUID) (int, error) {
	return s.repo.GetMessageCount(ctx, conversationID)
}

// Delete 删除对话
func (s *Service) Delete(ctx context.Context, id, userID uuid.UUID) error {
	c, err := s.GetByIDForUser(ctx, id, userID)
	if err != nil {
		return err
	}
	_ = c

	return s.repo.Delete(ctx, id)
}

// ============================================
// Admin 方法
// ============================================

// AdminList 获取对话列表（管理员，可跨用户）
func (s *Service) AdminList(ctx context.Context, userID *uuid.UUID, status *conversation.Status, limit, offset int) ([]*conversation.Conversation, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return s.repo.ListAll(ctx, userID, status, limit, offset)
}

// AdminGetWithMessages 管理员获取对话详情（不检查权限）
func (s *Service) AdminGetWithMessages(ctx context.Context, id uuid.UUID) (*conversation.Conversation, error) {
	c, err := s.repo.GetByIDWithMessages(ctx, id)
	if err != nil {
		if errors.Is(err, convRepo.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return c, nil
}

// nilIfEmpty 空字符串转 nil
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
