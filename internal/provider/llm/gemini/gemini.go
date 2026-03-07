package gemini

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/google/generative-ai-go/genai"
	"github.com/peizhengma/biography-v2/internal/provider/llm"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// Provider Gemini LLM 提供者
type Provider struct {
	client    *genai.Client
	model     string
	modelFast string
	proxy     string
}

// New 创建 Gemini 提供者
func New(cfg llm.ProviderConfig) (*Provider, error) {
	if cfg.APIKey == "" {
		return nil, errors.New("gemini: API key is required")
	}

	ctx := context.Background()
	opts := []option.ClientOption{
		option.WithAPIKey(cfg.APIKey),
	}

	// 配置代理
	if cfg.Proxy != "" {
		proxyURL, err := url.Parse(cfg.Proxy)
		if err != nil {
			return nil, fmt.Errorf("gemini: invalid proxy URL: %w", err)
		}

		transport := &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}

		httpClient := &http.Client{
			Transport: transport,
			Timeout:   time.Duration(cfg.Timeout) * time.Second,
		}

		opts = append(opts, option.WithHTTPClient(httpClient))
	}

	client, err := genai.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("gemini: failed to create client: %w", err)
	}

	model := cfg.Model
	if model == "" {
		model = "gemini-2.5-flash"
	}

	modelFast := cfg.ModelFast
	if modelFast == "" {
		modelFast = "gemini-2.0-flash-lite"
	}

	return &Provider{
		client:    client,
		model:     model,
		modelFast: modelFast,
		proxy:     cfg.Proxy,
	}, nil
}

// Name 返回提供者名称
func (p *Provider) Name() string {
	return "gemini"
}

// UpstreamEndpoint 返回上游服务地址（供监控展示）
func (p *Provider) UpstreamEndpoint() string {
	if p.proxy != "" {
		return p.proxy
	}
	return "https://generativelanguage.googleapis.com"
}

// Chat 同步对话
func (p *Provider) Chat(ctx context.Context, messages []llm.Message) (*llm.Response, error) {
	model := p.client.GenerativeModel(p.model)

	// 配置模型参数
	model.SetTemperature(0.7)
	model.SetTopP(0.9)

	// 转换消息格式
	contents, systemInstruction := p.convertMessages(messages)

	// 设置系统指令
	if systemInstruction != "" {
		model.SystemInstruction = genai.NewUserContent(genai.Text(systemInstruction))
	}

	// 发送请求
	resp, err := model.GenerateContent(ctx, contents...)
	if err != nil {
		return nil, fmt.Errorf("gemini: generate content failed: %w", err)
	}

	// 解析响应
	content, finishReason := p.parseResponse(resp)

	return &llm.Response{
		Content:      content,
		FinishReason: finishReason,
		TokensUsed:   p.getTokensUsed(resp),
	}, nil
}

// ChatStream 流式对话
func (p *Provider) ChatStream(ctx context.Context, messages []llm.Message) (<-chan llm.Chunk, error) {
	model := p.client.GenerativeModel(p.model)

	// 配置模型参数
	model.SetTemperature(0.7)
	model.SetTopP(0.9)

	// 转换消息格式
	contents, systemInstruction := p.convertMessages(messages)

	// 设置系统指令
	if systemInstruction != "" {
		model.SystemInstruction = genai.NewUserContent(genai.Text(systemInstruction))
	}

	// 创建流式迭代器
	iter := model.GenerateContentStream(ctx, contents...)

	// 创建输出通道
	ch := make(chan llm.Chunk, 100)

	go func() {
		defer close(ch)

		for {
			resp, err := iter.Next()
			if errors.Is(err, iterator.Done) {
				ch <- llm.Chunk{Done: true}
				return
			}
			if err != nil {
				ch <- llm.Chunk{Error: err}
				return
			}

			// 提取文本
			text := p.extractText(resp)
			if text != "" {
				ch <- llm.Chunk{Content: text}
			}
		}
	}()

	return ch, nil
}

// HealthCheck 健康检查
func (p *Provider) HealthCheck(ctx context.Context) error {
	model := p.client.GenerativeModel(p.modelFast)

	_, err := model.GenerateContent(ctx, genai.Text("ping"))
	if err != nil {
		return fmt.Errorf("gemini: health check failed: %w", err)
	}

	return nil
}

// Close 关闭客户端
func (p *Provider) Close() error {
	return p.client.Close()
}

// convertMessages 转换消息格式
func (p *Provider) convertMessages(messages []llm.Message) ([]genai.Part, string) {
	var contents []genai.Part
	var systemInstruction string

	for _, msg := range messages {
		switch msg.Role {
		case "system":
			// Gemini 使用 SystemInstruction 来处理系统消息
			systemInstruction = msg.Content
		case "user":
			contents = append(contents, genai.Text(msg.Content))
		case "assistant":
			// 对于多轮对话，需要构建 chat session
			// 这里简化处理，将历史消息拼接到用户消息中
			contents = append(contents, genai.Text("[Assistant]: "+msg.Content))
		}
	}

	return contents, systemInstruction
}

// parseResponse 解析响应
func (p *Provider) parseResponse(resp *genai.GenerateContentResponse) (string, string) {
	if resp == nil || len(resp.Candidates) == 0 {
		return "", "error"
	}

	candidate := resp.Candidates[0]
	content := p.extractText(resp)

	finishReason := "stop"
	if candidate.FinishReason != genai.FinishReasonStop {
		finishReason = candidate.FinishReason.String()
	}

	return content, finishReason
}

// extractText 从响应中提取文本
func (p *Provider) extractText(resp *genai.GenerateContentResponse) string {
	if resp == nil || len(resp.Candidates) == 0 {
		return ""
	}

	candidate := resp.Candidates[0]
	if candidate.Content == nil {
		return ""
	}

	var text string
	for _, part := range candidate.Content.Parts {
		if t, ok := part.(genai.Text); ok {
			text += string(t)
		}
	}

	return text
}

// getTokensUsed 获取使用的 token 数量
func (p *Provider) getTokensUsed(resp *genai.GenerateContentResponse) int {
	if resp == nil || resp.UsageMetadata == nil {
		return 0
	}

	return int(resp.UsageMetadata.TotalTokenCount)
}
