package dashscope

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/peizhengma/biography-v2/internal/provider/llm"
)

// Provider DashScope LLM 提供者 (阿里通义千问，使用 OpenAI 兼容 API)
type Provider struct {
	apiKey    string
	baseURL   string
	model     string
	modelFast string
	client    *http.Client
}

// New 创建 DashScope 提供者
func New(cfg llm.ProviderConfig) (*Provider, error) {
	if cfg.APIKey == "" {
		return nil, errors.New("dashscope: API key is required")
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	}

	model := cfg.Model
	if model == "" {
		model = "qwen-plus"
	}

	modelFast := cfg.ModelFast
	if modelFast == "" {
		modelFast = "qwen-turbo"
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 60
	}

	return &Provider{
		apiKey:    cfg.APIKey,
		baseURL:   strings.TrimSuffix(baseURL, "/"),
		model:     model,
		modelFast: modelFast,
		client: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
	}, nil
}

// Name 返回提供者名称
func (p *Provider) Name() string {
	return "dashscope"
}

// ModelName 返回当前主模型名称（供监控展示）
func (p *Provider) ModelName() string {
	return p.model
}

// UpstreamEndpoint 返回上游服务地址（供监控展示）
func (p *Provider) UpstreamEndpoint() string {
	return p.baseURL + "/chat/completions"
}

// RawGenerate 执行一次原始请求，返回原始请求体、原始响应体和状态码（用于监控诊断）。
func (p *Provider) RawGenerate(ctx context.Context, prompt string) (string, string, int, error) {
	reqBody := chatRequest{
		Model: p.model,
		Messages: []chatMessage{
			{Role: "user", Content: prompt},
		},
		Stream: false,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", "", 0, fmt.Errorf("dashscope: failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return string(jsonBody), "", 0, fmt.Errorf("dashscope: failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return string(jsonBody), "", 0, fmt.Errorf("dashscope: request failed: %w", err)
	}
	defer resp.Body.Close()

	rawResp, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return string(jsonBody), string(rawResp), resp.StatusCode, fmt.Errorf("dashscope: API error (status %d): %s", resp.StatusCode, string(rawResp))
	}

	return string(jsonBody), string(rawResp), resp.StatusCode, nil
}

// chatRequest OpenAI 兼容的请求格式
type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatResponse OpenAI 兼容的响应格式
type chatResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Index        int `json:"index"`
		Message      chatMessage
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// streamChunk 流式响应块
type streamChunk struct {
	ID      string `json:"id"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

// Chat 同步对话
func (p *Provider) Chat(ctx context.Context, messages []llm.Message) (*llm.Response, error) {
	// 转换消息格式
	chatMessages := make([]chatMessage, len(messages))
	for i, msg := range messages {
		chatMessages[i] = chatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	reqBody := chatRequest{
		Model:    p.model,
		Messages: chatMessages,
		Stream:   false,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("dashscope: failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("dashscope: failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("dashscope: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("dashscope: API error (status %d): %s", resp.StatusCode, string(body))
	}

	var chatResp chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("dashscope: failed to decode response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, errors.New("dashscope: no choices in response")
	}

	return &llm.Response{
		Content:      chatResp.Choices[0].Message.Content,
		FinishReason: chatResp.Choices[0].FinishReason,
		TokensUsed:   chatResp.Usage.TotalTokens,
	}, nil
}

// ChatStream 流式对话
func (p *Provider) ChatStream(ctx context.Context, messages []llm.Message) (<-chan llm.Chunk, error) {
	// 转换消息格式
	chatMessages := make([]chatMessage, len(messages))
	for i, msg := range messages {
		chatMessages[i] = chatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	reqBody := chatRequest{
		Model:    p.model,
		Messages: chatMessages,
		Stream:   true,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("dashscope: failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("dashscope: failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("dashscope: request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("dashscope: API error (status %d): %s", resp.StatusCode, string(body))
	}

	ch := make(chan llm.Chunk, 100)

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()

			// 跳过空行和注释
			if line == "" || strings.HasPrefix(line, ":") {
				continue
			}

			// 解析 SSE 数据
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				ch <- llm.Chunk{Done: true}
				return
			}

			var chunk streamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
				ch <- llm.Chunk{Content: chunk.Choices[0].Delta.Content}
			}
		}

		if err := scanner.Err(); err != nil {
			ch <- llm.Chunk{Error: err}
		}
	}()

	return ch, nil
}

// HealthCheck 健康检查
func (p *Provider) HealthCheck(ctx context.Context) error {
	messages := []llm.Message{
		{Role: "user", Content: "ping"},
	}

	// 使用快速模型进行健康检查
	chatMessages := []chatMessage{
		{Role: "user", Content: "ping"},
	}

	reqBody := chatRequest{
		Model:    p.modelFast,
		Messages: chatMessages,
		Stream:   false,
	}

	jsonBody, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("dashscope: health check failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("dashscope: health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("dashscope: health check failed with status %d", resp.StatusCode)
	}

	_ = messages // suppress unused warning
	return nil
}
