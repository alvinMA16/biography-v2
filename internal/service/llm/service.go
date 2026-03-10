package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"text/template"
	"unicode/utf8"

	"github.com/peizhengma/biography-v2/internal/domain/memoir"
	"github.com/peizhengma/biography-v2/internal/domain/topic"
	"github.com/peizhengma/biography-v2/internal/prompt"
	"github.com/peizhengma/biography-v2/internal/provider/llm"
)

var (
	ErrLLMNotAvailable = errors.New("LLM provider not available")
	ErrInvalidResponse = errors.New("invalid LLM response")
	ErrEmptyInput      = errors.New("empty input")
)

// Service LLM 业务服务
type Service struct {
	manager *llm.Manager
}

// New 创建 LLM 服务
func New(manager *llm.Manager) *Service {
	return &Service{manager: manager}
}

// GenerateMemoirInput 生成回忆录输入
type GenerateMemoirInput struct {
	UserName     string
	BirthYear    *int
	Hometown     string
	Topic        string
	Conversation string
}

// GenerateMemoirOutput 生成回忆录输出
type GenerateMemoirOutput struct {
	Title      string `json:"title"`
	Content    string `json:"content"`
	TimePeriod string `json:"time_period"`
	StartYear  *int   `json:"start_year"`
	EndYear    *int   `json:"end_year"`
}

// GenerateMemoir 生成回忆录
func (s *Service) GenerateMemoir(ctx context.Context, input *GenerateMemoirInput) (*memoir.GeneratedMemoir, error) {
	if input.Conversation == "" {
		return nil, ErrEmptyInput
	}

	if _, err := s.manager.Primary(); err != nil {
		return nil, ErrLLMNotAvailable
	}

	// 构建 prompt
	birthYear := 0
	if input.BirthYear != nil {
		birthYear = *input.BirthYear
	}

	promptText, err := renderTemplate(prompt.MemoirPrompt, map[string]interface{}{
		"UserName":     input.UserName,
		"BirthYear":    birthYear,
		"Hometown":     input.Hometown,
		"Topic":        input.Topic,
		"Conversation": input.Conversation,
	})
	if err != nil {
		return nil, err
	}

	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		resp, providerName, err := s.manager.ChatWithRetry(ctx, []llm.Message{
			{Role: "user", Content: promptText},
		}, 1)
		if err != nil {
			lastErr = err
			log.Printf("[LLM] GenerateMemoir failed (attempt %d/3 provider=%s): %v", attempt, providerName, err)
			continue
		}

		var output GenerateMemoirOutput
		if err := parseJSONResponse(resp.Content, &output); err != nil {
			lastErr = err
			log.Printf("[LLM] GenerateMemoir invalid response (attempt %d/3 provider=%s): %s", attempt, providerName, truncateForLog(resp.Content, 1000))
			continue
		}

		return &memoir.GeneratedMemoir{
			Title:      output.Title,
			Content:    output.Content,
			TimePeriod: output.TimePeriod,
			StartYear:  output.StartYear,
			EndYear:    output.EndYear,
		}, nil
	}

	return nil, lastErr
}

// GenerateTopicsInput 生成话题输入
type GenerateTopicsInput struct {
	UserName        string
	BirthYear       *int
	Hometown        string
	MainCity        string
	UserMemory      string
	MemoirSummaries string
	ExistingTopics  []string
	Count           int
}

// GenerateTopics 生成话题
func (s *Service) GenerateTopics(ctx context.Context, input *GenerateTopicsInput) ([]topic.GeneratedTopic, error) {
	if _, err := s.manager.Primary(); err != nil {
		return nil, ErrLLMNotAvailable
	}

	count := input.Count
	if count <= 0 {
		count = 4
	}

	birthYear := 0
	if input.BirthYear != nil {
		birthYear = *input.BirthYear
	}

	existingTopics := "无"
	if len(input.ExistingTopics) > 0 {
		existingTopics = strings.Join(input.ExistingTopics, "、")
	}

	userMemory := strings.TrimSpace(input.UserMemory)
	if userMemory == "" {
		userMemory = "暂无"
	}

	memoirSummaries := strings.TrimSpace(input.MemoirSummaries)
	if memoirSummaries == "" {
		memoirSummaries = "暂无"
	}

	promptText, err := renderTemplate(prompt.TopicPrompt, map[string]interface{}{
		"UserName":        input.UserName,
		"BirthYear":       birthYear,
		"Hometown":        input.Hometown,
		"MainCity":        input.MainCity,
		"UserMemory":      userMemory,
		"MemoirSummaries": memoirSummaries,
		"ExistingTopics":  existingTopics,
		"Count":           count,
	})
	if err != nil {
		return nil, err
	}

	resp, providerName, err := s.manager.ChatWithRetry(ctx, []llm.Message{
		{Role: "user", Content: promptText},
	}, 3)
	if err != nil {
		return nil, err
	}

	var topics []topic.GeneratedTopic
	if err := parseJSONResponse(resp.Content, &topics); err != nil {
		log.Printf("[LLM] GenerateTopics invalid response (provider=%s): %s", providerName, truncateForLog(resp.Content, 1000))
		return nil, err
	}

	return topics, nil
}

// GenerateStoryMemoryInput 生成用户长期记忆输入
type GenerateStoryMemoryInput struct {
	UserName        string
	BirthYear       *int
	Hometown        string
	MainCity        string
	ExistingMemory  string
	MemoirSummaries string
	Conversation    string
}

// GenerateStoryMemory 生成用户长期记忆
func (s *Service) GenerateStoryMemory(ctx context.Context, input *GenerateStoryMemoryInput) (string, error) {
	if strings.TrimSpace(input.Conversation) == "" {
		return "", ErrEmptyInput
	}

	if _, err := s.manager.Primary(); err != nil {
		return "", ErrLLMNotAvailable
	}

	birthYear := 0
	if input.BirthYear != nil {
		birthYear = *input.BirthYear
	}

	existingMemory := strings.TrimSpace(input.ExistingMemory)
	if existingMemory == "" {
		existingMemory = "暂无"
	}

	memoirSummaries := strings.TrimSpace(input.MemoirSummaries)
	if memoirSummaries == "" {
		memoirSummaries = "暂无"
	}

	promptText, err := renderTemplate(prompt.StoryMemoryPrompt, map[string]interface{}{
		"UserName":        input.UserName,
		"BirthYear":       birthYear,
		"Hometown":        input.Hometown,
		"MainCity":        input.MainCity,
		"ExistingMemory":  existingMemory,
		"MemoirSummaries": memoirSummaries,
		"Conversation":    input.Conversation,
	})
	if err != nil {
		return "", err
	}

	resp, providerName, err := s.manager.ChatWithRetry(ctx, []llm.Message{
		{Role: "user", Content: promptText},
	}, 3)
	if err != nil {
		return "", err
	}

	memory := strings.TrimSpace(resp.Content)
	if memory == "" {
		return "", ErrInvalidResponse
	}

	if utf8.RuneCountInString(memory) > 800 {
		memory = truncateRunes(memory, 800)
	}

	log.Printf("[LLM] GenerateStoryMemory success (provider=%s len=%d)", providerName, utf8.RuneCountInString(memory))
	return memory, nil
}

// GenerateSummary 生成对话摘要
func (s *Service) GenerateSummary(ctx context.Context, topicTitle, conversation string) (string, error) {
	if conversation == "" {
		return "", ErrEmptyInput
	}

	provider, err := s.manager.Primary()
	if err != nil {
		return "", ErrLLMNotAvailable
	}

	promptText, err := renderTemplate(prompt.SummaryPrompt, map[string]interface{}{
		"Topic":        topicTitle,
		"Conversation": conversation,
	})
	if err != nil {
		return "", err
	}

	resp, err := provider.Chat(ctx, []llm.Message{
		{Role: "user", Content: promptText},
	})
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(resp.Content), nil
}

// ExtractedProfile 提取的用户信息
type ExtractedProfile struct {
	Nickname      *string `json:"nickname"`
	PreferredName *string `json:"preferred_name"`
	Gender        *string `json:"gender"`
	BirthYear     *int    `json:"birth_year"`
	Hometown      *string `json:"hometown"`
	MainCity      *string `json:"main_city"`
}

// ExtractProfile 从对话提取用户信息
func (s *Service) ExtractProfile(ctx context.Context, conversation string) (*ExtractedProfile, error) {
	if conversation == "" {
		return nil, ErrEmptyInput
	}

	provider, err := s.manager.Primary()
	if err != nil {
		return nil, ErrLLMNotAvailable
	}

	promptText, err := renderTemplate(prompt.ProfileExtractionPrompt, map[string]interface{}{
		"Conversation": conversation,
	})
	if err != nil {
		return nil, err
	}

	resp, err := provider.Chat(ctx, []llm.Message{
		{Role: "user", Content: promptText},
	})
	if err != nil {
		return nil, err
	}

	var profile ExtractedProfile
	if err := parseJSONResponse(resp.Content, &profile); err != nil {
		return nil, err
	}

	return &profile, nil
}

// GenerateEraMemories 生成时代记忆
func (s *Service) GenerateEraMemories(ctx context.Context, birthYear int, hometown, mainCity string) (string, error) {
	provider, err := s.manager.Primary()
	if err != nil {
		return "", ErrLLMNotAvailable
	}

	promptText, err := renderTemplate(prompt.EraMemoriesPrompt, map[string]interface{}{
		"BirthYear": birthYear,
		"Hometown":  hometown,
		"MainCity":  mainCity,
	})
	if err != nil {
		return "", err
	}

	resp, err := provider.Chat(ctx, []llm.Message{
		{Role: "user", Content: promptText},
	})
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(resp.Content), nil
}

// renderTemplate 渲染模板
func renderTemplate(tmplStr string, data map[string]interface{}) (string, error) {
	tmpl, err := template.New("prompt").Parse(tmplStr)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// parseJSONResponse 解析 JSON 响应
func parseJSONResponse(content string, v interface{}) error {
	// 尝试提取 JSON 部分（LLM 可能会在 JSON 前后加入其他文本）
	content = strings.TrimSpace(content)

	// 查找 JSON 开始位置
	start := strings.Index(content, "[")
	if start == -1 {
		start = strings.Index(content, "{")
	}
	if start == -1 {
		return ErrInvalidResponse
	}

	// 查找 JSON 结束位置
	end := strings.LastIndex(content, "]")
	if end == -1 {
		end = strings.LastIndex(content, "}")
	}
	if end == -1 || end < start {
		return ErrInvalidResponse
	}

	jsonStr := content[start : end+1]

	if err := json.Unmarshal([]byte(jsonStr), v); err != nil {
		return ErrInvalidResponse
	}

	return nil
}

func truncateForLog(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func truncateRunes(s string, maxRunes int) string {
	if maxRunes <= 0 || utf8.RuneCountInString(s) <= maxRunes {
		return s
	}

	var b strings.Builder
	count := 0
	for _, r := range s {
		if count >= maxRunes {
			break
		}
		b.WriteRune(r)
		count++
	}
	return strings.TrimSpace(b.String())
}
