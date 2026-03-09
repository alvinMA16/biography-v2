package flow

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/peizhengma/biography-v2/internal/domain/conversation"
	"github.com/peizhengma/biography-v2/internal/domain/memoir"
	"github.com/peizhengma/biography-v2/internal/domain/topic"
	"github.com/peizhengma/biography-v2/internal/domain/user"
	convService "github.com/peizhengma/biography-v2/internal/service/conversation"
	llmService "github.com/peizhengma/biography-v2/internal/service/llm"
	memoirService "github.com/peizhengma/biography-v2/internal/service/memoir"
	topicService "github.com/peizhengma/biography-v2/internal/service/topic"
	userService "github.com/peizhengma/biography-v2/internal/service/user"
)

var (
	ErrConversationNotFound = errors.New("conversation not found")
	ErrNotOwner             = errors.New("not conversation owner")
	ErrAlreadyCompleted     = errors.New("conversation already completed")
)

// Service 对话流程服务，协调对话结束后的各项任务
type Service struct {
	userService   *userService.Service
	convService   *convService.Service
	memoirService *memoirService.Service
	topicService  *topicService.Service
	llmService    *llmService.Service
}

// New 创建流程服务
func New(
	userSvc *userService.Service,
	convSvc *convService.Service,
	memoirSvc *memoirService.Service,
	topicSvc *topicService.Service,
	llmSvc *llmService.Service,
) *Service {
	return &Service{
		userService:   userSvc,
		convService:   convSvc,
		memoirService: memoirSvc,
		topicService:  topicSvc,
		llmService:    llmSvc,
	}
}

// EndConversationResult 对话结束结果
type EndConversationResult struct {
	Conversation *conversation.Conversation `json:"conversation"`
	Memoir       *memoir.Memoir             `json:"memoir,omitempty"`
	Summary      string                     `json:"summary,omitempty"`
	Message      string                     `json:"message"`
}

// EndConversation 结束对话（同步处理）
func (s *Service) EndConversation(ctx context.Context, conversationID, userID uuid.UUID) (*EndConversationResult, error) {
	// 获取并验证对话
	conv, err := s.convService.GetByIDForUser(ctx, conversationID, userID)
	if err != nil {
		if errors.Is(err, convService.ErrNotFound) {
			return nil, ErrConversationNotFound
		}
		if errors.Is(err, convService.ErrNotOwner) {
			return nil, ErrNotOwner
		}
		return nil, err
	}

	if conv.Status == conversation.StatusCompleted {
		return nil, ErrAlreadyCompleted
	}

	// 更新状态为已完成
	if err := s.convService.Complete(ctx, conversationID, userID); err != nil {
		return nil, err
	}

	// 获取用户信息
	u, err := s.userService.GetByID(ctx, userID)
	if err != nil {
		log.Printf("[Flow] 获取用户信息失败: %v", err)
		return &EndConversationResult{
			Conversation: conv,
			Message:      "对话已结束，但后续处理失败",
		}, nil
	}

	// 获取对话消息
	messages, err := s.convService.GetAllMessages(ctx, conversationID)
	if err != nil {
		log.Printf("[Flow] 获取对话消息失败: %v", err)
		return &EndConversationResult{
			Conversation: conv,
			Message:      "对话已结束，但后续处理失败",
		}, nil
	}

	// 构建对话文本
	conversationText := buildConversationText(messages)
	if len(conversationText) < 50 {
		return &EndConversationResult{
			Conversation: conv,
			Message:      "对话已结束，内容太短无法生成摘要",
		}, nil
	}

	result := &EndConversationResult{
		Conversation: conv,
	}

	// 资料未完整时，先结束对话，不做 AI 自动资料写回。
	if !u.OnboardingCompleted {
		log.Printf("[Flow] 用户资料未完整，跳过自动资料写回: %s", userID)
		result.Message = "对话已结束，用户资料待人工完善"
	} else {
		// 资料完整时，执行正常流程。
		summary := s.generateSummary(ctx, conv, conversationText)
		result.Summary = summary

		m := s.autoGenerateMemoir(ctx, u, conv, conversationText)
		result.Memoir = m

		s.handleTopicPool(ctx, u)

		result.Message = "对话已结束，已生成摘要和回忆录"
	}

	return result, nil
}

// EndConversationQuick 快速结束对话（后台异步处理）
func (s *Service) EndConversationQuick(ctx context.Context, conversationID, userID uuid.UUID) error {
	// 获取并验证对话
	conv, err := s.convService.GetByIDForUser(ctx, conversationID, userID)
	if err != nil {
		if errors.Is(err, convService.ErrNotFound) {
			return ErrConversationNotFound
		}
		if errors.Is(err, convService.ErrNotOwner) {
			return ErrNotOwner
		}
		return err
	}

	if conv.Status == conversation.StatusCompleted {
		return ErrAlreadyCompleted
	}

	// 更新状态为已完成
	if err := s.convService.Complete(ctx, conversationID, userID); err != nil {
		return err
	}

	// 启动后台处理（使用 goroutine）
	go s.processConversationEnd(conversationID, userID)

	return nil
}

// processConversationEnd 后台处理对话结束后的任务
func (s *Service) processConversationEnd(conversationID, userID uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	log.Printf("[Flow] 开始处理对话结束任务: %s", conversationID)

	// 获取用户信息
	u, err := s.userService.GetByID(ctx, userID)
	if err != nil {
		log.Printf("[Flow] 获取用户信息失败: %v", err)
		return
	}

	// 获取对话
	conv, err := s.convService.GetByID(ctx, conversationID)
	if err != nil {
		log.Printf("[Flow] 获取对话失败: %v", err)
		return
	}

	// 获取对话消息
	messages, err := s.convService.GetAllMessages(ctx, conversationID)
	if err != nil {
		log.Printf("[Flow] 获取对话消息失败: %v", err)
		return
	}

	conversationText := buildConversationText(messages)
	if len(conversationText) < 50 {
		log.Printf("[Flow] 对话内容太短，跳过处理: %s", conversationID)
		return
	}

	if !u.OnboardingCompleted {
		log.Printf("[Flow] 用户资料未完整，跳过自动资料写回: %s", userID)
	} else {
		// 生成摘要
		s.generateSummary(ctx, conv, conversationText)

		// 自动生成回忆录
		s.autoGenerateMemoir(ctx, u, conv, conversationText)

		// 处理话题池
		s.handleTopicPool(ctx, u)
	}

	log.Printf("[Flow] 对话结束任务完成: %s", conversationID)
}

// generateSummary 生成对话摘要
func (s *Service) generateSummary(ctx context.Context, conv *conversation.Conversation, conversationText string) string {
	topicTitle := ""
	if conv.Topic != nil {
		topicTitle = *conv.Topic
	}

	summary, err := s.llmService.GenerateSummary(ctx, topicTitle, conversationText)
	if err != nil {
		log.Printf("[Flow] 生成摘要失败: %v", err)
		return ""
	}

	// 更新对话摘要
	if err := s.convService.UpdateSummary(ctx, conv.ID, summary); err != nil {
		log.Printf("[Flow] 更新摘要失败: %v", err)
	} else {
		log.Printf("[Flow] 生成摘要成功: %s...", truncate(summary, 50))
	}

	return summary
}

// autoGenerateMemoir 自动生成回忆录
func (s *Service) autoGenerateMemoir(ctx context.Context, u *user.User, conv *conversation.Conversation, conversationText string) *memoir.Memoir {
	// 检查是否已有回忆录
	existing, _ := s.memoirService.GetByConversationID(ctx, conv.ID)
	if existing != nil {
		log.Printf("[Flow] 该对话已有回忆录，跳过自动生成")
		return existing
	}

	log.Printf("[Flow] 自动生成回忆录: %s", conv.ID)

	// 调用 LLM 生成回忆录
	topicTitle := ""
	if conv.Topic != nil {
		topicTitle = *conv.Topic
	}

	result, err := s.llmService.GenerateMemoir(ctx, &llmService.GenerateMemoirInput{
		UserName:     u.DisplayName(),
		BirthYear:    u.BirthYear,
		Hometown:     derefString(u.Hometown),
		Topic:        topicTitle,
		Conversation: conversationText,
	})
	if err != nil {
		log.Printf("[Flow] 生成回忆录内容失败: %v", err)
		return nil
	}

	// 创建回忆录
	input := &memoir.CreateMemoirInput{
		ConversationID: &conv.ID,
		Title:          result.Title,
		Content:        result.Content,
		TimePeriod:     result.TimePeriod,
		StartYear:      result.StartYear,
		EndYear:        result.EndYear,
	}

	m, err := s.memoirService.Create(ctx, u.ID, input)
	if err != nil {
		log.Printf("[Flow] 创建回忆录失败: %v", err)
		return nil
	}

	log.Printf("[Flow] 回忆录生成成功: %s", m.Title)
	return m
}

// handleTopicPool 根据回忆录数量处理话题池
func (s *Service) handleTopicPool(ctx context.Context, u *user.User) {
	// 获取已完成回忆录数量
	count, err := s.memoirService.Count(ctx, u.ID)
	if err != nil {
		log.Printf("[Flow] 获取回忆录数量失败: %v", err)
		return
	}

	if count == 1 {
		// 刚完成第一篇回忆录，首次生成个性化话题
		log.Printf("[Flow] 用户完成第一篇回忆录，生成个性化话题")
		s.generatePersonalizedTopics(ctx, u)
	} else if count > 1 {
		// 已有多篇回忆录，审查话题池
		log.Printf("[Flow] 用户已有 %d 篇回忆录，审查话题池", count)
		s.reviewTopicPool(ctx, u)
	}
}

// generatePersonalizedTopics 生成个性化话题
func (s *Service) generatePersonalizedTopics(ctx context.Context, u *user.User) {
	// 获取已有话题标题
	existingTopics, _ := s.topicService.GetAvailableForUser(ctx, u.ID, 100)
	var existingTitles []string
	for _, t := range existingTopics {
		existingTitles = append(existingTitles, t.Title)
	}

	// 获取已有回忆录标题
	memoirs, _ := s.memoirService.ListByUserID(ctx, u.ID)
	var memoirTitles []string
	for _, m := range memoirs {
		memoirTitles = append(memoirTitles, m.Title)
	}

	// 调用 LLM 生成话题
	topics, err := s.llmService.GenerateTopics(ctx, &llmService.GenerateTopicsInput{
		UserName:        u.DisplayName(),
		BirthYear:       u.BirthYear,
		Hometown:        derefString(u.Hometown),
		MainCity:        derefString(u.MainCity),
		EraMemories:     derefString(u.EraMemories),
		ExistingTopics:  existingTitles,
		ExistingMemoirs: memoirTitles,
		Count:           8,
	})
	if err != nil {
		log.Printf("[Flow] 生成话题失败: %v", err)
		return
	}

	// 创建话题
	for _, t := range topics {
		input := &topic.CreateTopicInput{
			Title:      t.Title,
			Greeting:   t.Greeting,
			Context:    t.Context,
			EraContext: t.EraContext,
			Source:     topic.SourceAI,
		}
		if _, err := s.topicService.Create(ctx, u.ID, input); err != nil {
			log.Printf("[Flow] 创建话题失败: %v", err)
		}
	}

	log.Printf("[Flow] 已生成 %d 个个性化话题", len(topics))
}

// reviewTopicPool 审查话题池
func (s *Service) reviewTopicPool(ctx context.Context, u *user.User) {
	// 获取可用话题
	existingTopics, _ := s.topicService.GetAvailableForUser(ctx, u.ID, 100)

	// 如果话题数量少于 5，补充新话题
	if len(existingTopics) < 5 {
		log.Printf("[Flow] 话题数量不足，补充新话题")
		s.generatePersonalizedTopics(ctx, u)
	}
}

// buildConversationText 构建对话文本
func buildConversationText(messages []conversation.Message) string {
	var sb strings.Builder
	for _, msg := range messages {
		role := "用户"
		if msg.Role == "assistant" {
			role = "记录师"
		}
		sb.WriteString(fmt.Sprintf("%s：%s\n\n", role, msg.Content))
	}
	return sb.String()
}

// derefString 安全解引用字符串指针
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// truncate 截断字符串
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
