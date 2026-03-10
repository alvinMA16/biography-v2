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

		s.updateStoryMemory(ctx, u, conversationText)
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

	// 等待实时会话把消息落库，避免 end-quick 抢跑。
	messages, err := s.waitForConversationMessages(ctx, conversationID)
	if err != nil {
		log.Printf("[Flow] 获取对话消息失败: %v", err)
		return
	}

	conversationText := buildConversationText(messages)
	if len(conversationText) < 50 {
		log.Printf("[Flow] 对话内容太短，跳过处理: %s", conversationID)
		return
	}

	// 生成摘要
	s.generateSummary(ctx, conv, conversationText)

	// 自动生成回忆录
	s.autoGenerateMemoir(ctx, u, conv, conversationText)

	// 更新用户长期记忆
	s.updateStoryMemory(ctx, u, conversationText)

	// 处理话题池
	s.handleTopicPool(ctx, u)

	log.Printf("[Flow] 对话结束任务完成: %s", conversationID)
}

func (s *Service) waitForConversationMessages(ctx context.Context, conversationID uuid.UUID) ([]conversation.Message, error) {
	const (
		maxWait             = 4 * time.Second
		pollInterval        = 200 * time.Millisecond
		stablePollThreshold = 2
	)

	deadline := time.Now().Add(maxWait)
	lastCount := -1
	stablePolls := 0

	for {
		messages, err := s.convService.GetAllMessages(ctx, conversationID)
		if err != nil {
			return nil, err
		}

		count := len(messages)
		if count >= 2 {
			if count == lastCount {
				stablePolls++
			} else {
				stablePolls = 0
			}
			if stablePolls >= stablePollThreshold || time.Now().After(deadline) {
				if stablePolls > 0 {
					log.Printf("[Flow] 对话消息已稳定: conversation_id=%s messages=%d", conversationID, count)
				}
				return messages, nil
			}
		}

		if time.Now().After(deadline) {
			if count < 2 {
				log.Printf("[Flow] 等待消息落库超时: conversation_id=%s messages=%d", conversationID, count)
			}
			return messages, nil
		}

		lastCount = count
		time.Sleep(pollInterval)
	}
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
	existingMemoirs, err := s.memoirService.ListByConversationID(ctx, conv.ID)
	if err == nil && len(existingMemoirs) > 0 {
		log.Printf("[Flow] 该对话已有 %d 篇回忆录，跳过自动生成", len(existingMemoirs))
		return existingMemoirs[0]
	}

	log.Printf("[Flow] 自动生成回忆录: %s", conv.ID)

	topicTitle := ""
	if conv.Topic != nil {
		topicTitle = *conv.Topic
	}

	plans, err := s.llmService.PlanMemoirs(ctx, &llmService.PlanMemoirsInput{
		UserName:     u.DisplayName(),
		BirthYear:    u.BirthYear,
		Hometown:     derefString(u.Hometown),
		StoryMemory:  derefString(u.StoryMemory),
		Topic:        topicTitle,
		Conversation: conversationText,
	})
	if err != nil {
		log.Printf("[Flow] 回忆录规划失败，回退到单篇生成: %v", err)
		plans = []llmService.PlannedMemoir{
			{
				ShouldGenerate:  true,
				TitleHint:       fallbackMemoirTitle(topicTitle, ""),
				Theme:           fallbackString(topicTitle, "本次对话中的一段重要经历"),
				CoverageSummary: "围绕这次对话中的主要经历，忠实整理用户明确讲过的重要内容",
				Confidence:      "medium",
			},
		}
	}

	var firstCreated *memoir.Memoir
	for _, plan := range plans {
		excerpt := extractConversationExcerpt(conversationText, plan.StartAnchor, plan.EndAnchor)
		if strings.TrimSpace(excerpt) == "" {
			excerpt = strings.TrimSpace(conversationText)
		}

		result, err := s.llmService.GenerateMemoir(ctx, &llmService.GenerateMemoirInput{
			UserName:            u.DisplayName(),
			BirthYear:           u.BirthYear,
			Hometown:            derefString(u.Hometown),
			StoryMemory:         derefString(u.StoryMemory),
			Topic:               topicTitle,
			MemoirTheme:         plan.Theme,
			CoverageSummary:     plan.CoverageSummary,
			ConversationExcerpt: excerpt,
		})
		if err != nil {
			log.Printf("[Flow] 生成回忆录内容失败，回退原始记录: title_hint=%s err=%v", plan.TitleHint, err)
			m := s.createFallbackMemoir(ctx, u.ID, conv.ID, fallbackMemoirTitle(topicTitle, plan.TitleHint), excerpt)
			if firstCreated == nil && m != nil {
				firstCreated = m
			}
			continue
		}

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
			log.Printf("[Flow] 创建回忆录失败，回退原始记录: title=%s err=%v", result.Title, err)
			m = s.createFallbackMemoir(ctx, u.ID, conv.ID, fallbackMemoirTitle(topicTitle, plan.TitleHint), excerpt)
		} else {
			log.Printf("[Flow] 回忆录生成成功: %s", m.Title)
		}

		if firstCreated == nil && m != nil {
			firstCreated = m
		}
	}

	if firstCreated == nil {
		return s.createFallbackMemoir(ctx, u.ID, conv.ID, fallbackMemoirTitle(topicTitle, ""), strings.TrimSpace(conversationText))
	}

	return firstCreated
}

// handleTopicPool 根据回忆录数量处理话题池
func (s *Service) handleTopicPool(ctx context.Context, u *user.User) {
	// 获取已完成回忆录数量
	count, err := s.memoirService.Count(ctx, u.ID)
	if err != nil {
		log.Printf("[Flow] 获取回忆录数量失败: %v", err)
		return
	}

	if count <= 0 {
		return
	}

	existingTopics, _ := s.topicService.GetAvailableForUser(ctx, u.ID, 100)
	if len(existingTopics) == 0 {
		log.Printf("[Flow] 用户已有回忆录但暂无个性化话题，生成首批话题")
		s.generatePersonalizedTopics(ctx, u, 5)
	} else if len(existingTopics) < 4 {
		log.Printf("[Flow] 用户已有 %d 篇回忆录，审查话题池", count)
		s.reviewTopicPool(ctx, u)
	}
}

// generatePersonalizedTopics 生成个性化话题
func (s *Service) generatePersonalizedTopics(ctx context.Context, u *user.User, count int) {
	if count <= 0 {
		count = 4
	}

	// 获取已有话题标题
	existingTopics, _ := s.topicService.GetAvailableForUser(ctx, u.ID, 100)
	var existingTitles []string
	for _, t := range existingTopics {
		existingTitles = append(existingTitles, t.Title)
	}

	memoirSummaries := s.buildTopicMemoirSummaries(ctx, u.ID)

	// 调用 LLM 生成话题
	topics, err := s.llmService.GenerateTopics(ctx, &llmService.GenerateTopicsInput{
		UserName:        u.DisplayName(),
		BirthYear:       u.BirthYear,
		Hometown:        derefString(u.Hometown),
		MainCity:        derefString(u.MainCity),
		UserMemory:      derefString(u.StoryMemory),
		MemoirSummaries: memoirSummaries,
		ExistingTopics:  existingTitles,
		Count:           count,
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

	// 如果话题数量少于 4，补充到 5 个左右
	if len(existingTopics) < 4 {
		needed := 5 - len(existingTopics)
		if needed < 1 {
			needed = 1
		}
		log.Printf("[Flow] 话题数量不足，补充新话题: need=%d", needed)
		s.generatePersonalizedTopics(ctx, u, needed)
	}
}

// RegenerateTopicPoolForUser 管理端触发：清理 AI 话题并重新生成
func (s *Service) RegenerateTopicPoolForUser(ctx context.Context, userID uuid.UUID, count int) error {
	u, err := s.userService.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	memoirCount, err := s.memoirService.Count(ctx, userID)
	if err != nil {
		return err
	}
	if memoirCount <= 0 {
		return errors.New("user has no memoirs to generate topics from")
	}

	if count <= 0 {
		count = 5
	}

	if err := s.topicService.DeleteByUserAndSource(ctx, userID, topic.SourceAI); err != nil {
		return err
	}

	s.generatePersonalizedTopics(ctx, u, count)
	return nil
}

func (s *Service) updateStoryMemory(ctx context.Context, u *user.User, conversationText string) {
	memoirSummaries := s.buildTopicMemoirSummaries(ctx, u.ID)

	storyMemory, err := s.llmService.GenerateStoryMemory(ctx, &llmService.GenerateStoryMemoryInput{
		UserName:        u.DisplayName(),
		BirthYear:       u.BirthYear,
		Hometown:        derefString(u.Hometown),
		MainCity:        derefString(u.MainCity),
		ExistingMemory:  derefString(u.StoryMemory),
		MemoirSummaries: memoirSummaries,
		Conversation:    conversationText,
	})
	if err != nil {
		log.Printf("[Flow] 更新用户长期记忆失败: %v", err)
		return
	}

	if err := s.userService.UpdateStoryMemory(ctx, u.ID, storyMemory); err != nil {
		log.Printf("[Flow] 保存用户长期记忆失败: %v", err)
		return
	}

	u.StoryMemory = &storyMemory
	log.Printf("[Flow] 用户长期记忆已更新: user_id=%s len=%d", u.ID, len([]rune(storyMemory)))
}

func (s *Service) buildTopicMemoirSummaries(ctx context.Context, userID uuid.UUID) string {
	memoirs, err := s.memoirService.ListByUserID(ctx, userID)
	if err != nil {
		log.Printf("[Flow] 获取回忆录列表失败，无法构建摘要: %v", err)
		return ""
	}

	var parts []string
	for _, m := range memoirs {
		if m.ConversationID == nil {
			continue
		}

		conv, err := s.convService.GetByID(ctx, *m.ConversationID)
		if err != nil || conv.Summary == nil {
			continue
		}

		summary := strings.TrimSpace(*conv.Summary)
		if summary == "" {
			continue
		}

		parts = append(parts, fmt.Sprintf("%d. %s", len(parts)+1, summary))
		if len(parts) >= 5 {
			break
		}
	}

	return strings.Join(parts, "\n")
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

func extractConversationExcerpt(conversationText, startAnchor, endAnchor string) string {
	text := strings.TrimSpace(conversationText)
	if text == "" {
		return ""
	}

	startIdx := 0
	if anchor := strings.TrimSpace(startAnchor); anchor != "" {
		if idx := strings.Index(text, anchor); idx >= 0 {
			startIdx = idx
		}
	}

	endIdx := len(text)
	if anchor := strings.TrimSpace(endAnchor); anchor != "" {
		if idx := strings.Index(text[startIdx:], anchor); idx >= 0 {
			endIdx = startIdx + idx + len(anchor)
		}
	}

	if endIdx <= startIdx {
		return text
	}

	excerpt := strings.TrimSpace(text[startIdx:endIdx])
	if excerpt == "" || len([]rune(excerpt)) < 80 {
		return text
	}
	return excerpt
}

func fallbackMemoirTitle(topicTitle, titleHint string) string {
	if strings.TrimSpace(titleHint) != "" {
		return strings.TrimSpace(titleHint)
	}
	if strings.TrimSpace(topicTitle) != "" {
		return strings.TrimSpace(topicTitle)
	}
	return "一段对话记录"
}

func fallbackString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func (s *Service) createFallbackMemoir(ctx context.Context, userID, conversationID uuid.UUID, title, content string) *memoir.Memoir {
	fallbackInput := &memoir.CreateMemoirInput{
		ConversationID: &conversationID,
		Title:          fallbackMemoirTitle("", title),
		Content:        strings.TrimSpace(content),
	}

	m, err := s.memoirService.Create(ctx, userID, fallbackInput)
	if err != nil {
		log.Printf("[Flow] 创建原始对话记录失败: %v", err)
		return nil
	}

	log.Printf("[Flow] 已回退创建原始对话记录: %s", m.Title)
	return m
}
