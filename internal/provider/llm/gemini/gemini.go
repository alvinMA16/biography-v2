package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"github.com/peizhengma/biography-v2/internal/provider/llm"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// Provider Gemini LLM 提供者
type Provider struct {
	client      *genai.Client
	rawClient   *http.Client
	clientTrans *http.Transport
	rawTrans    *http.Transport
	apiKey      string
	model       string
	proxy       string
}

type generateContentRequest struct {
	Contents          []geminiContent         `json:"contents,omitempty"`
	SystemInstruction *geminiContent          `json:"systemInstruction,omitempty"`
	Tools             []geminiTool            `json:"tools,omitempty"`
	ToolConfig        *geminiToolConfig       `json:"toolConfig,omitempty"`
	GenerationConfig  *geminiGenerationConfig `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text         string              `json:"text,omitempty"`
	FunctionCall *geminiFunctionCall `json:"functionCall,omitempty"`
}

type geminiTool struct {
	FunctionDeclarations []geminiFunctionDeclaration `json:"functionDeclarations,omitempty"`
}

type geminiFunctionDeclaration struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

type geminiToolConfig struct {
	FunctionCallingConfig *geminiFunctionCallingConfig `json:"functionCallingConfig,omitempty"`
}

type geminiFunctionCallingConfig struct {
	Mode string `json:"mode,omitempty"`
}

type geminiGenerationConfig struct {
	Temperature *float32 `json:"temperature,omitempty"`
	TopP        *float32 `json:"topP,omitempty"`
}

type generateContentResponse struct {
	Candidates []struct {
		Content      *geminiContent `json:"content"`
		FinishReason string         `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata *struct {
		TotalTokenCount int32 `json:"totalTokenCount"`
	} `json:"usageMetadata,omitempty"`
}

type geminiFunctionCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args,omitempty"`
}

// New 创建 Gemini 提供者
func New(cfg llm.ProviderConfig) (*Provider, error) {
	if cfg.APIKey == "" {
		return nil, errors.New("gemini: API key is required")
	}

	ctx := context.Background()
	clientTransport, err := buildTransport(cfg.Proxy)
	if err != nil {
		return nil, err
	}
	clientHTTP := &http.Client{
		Transport: &apiKeyTransport{base: clientTransport, apiKey: cfg.APIKey},
		Timeout:   time.Duration(cfg.Timeout) * time.Second,
	}
	opts := []option.ClientOption{
		option.WithAPIKey(cfg.APIKey),
		option.WithHTTPClient(clientHTTP),
	}

	client, err := genai.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("gemini: failed to create client: %w", err)
	}

	model := cfg.Model

	// 创建共享 HTTP 客户端（用于 RawGenerate 等直接 HTTP 调用）
	rawTransport, err := buildTransport(cfg.Proxy)
	if err != nil {
		return nil, err
	}
	rawClient := &http.Client{
		Transport: rawTransport,
		Timeout:   30 * time.Second,
	}

	return &Provider{
		client:      client,
		rawClient:   rawClient,
		clientTrans: clientTransport,
		rawTrans:    rawTransport,
		apiKey:      cfg.APIKey,
		model:       model,
		proxy:       cfg.Proxy,
	}, nil
}

// apiKeyTransport 在每个请求的 URL 中注入 API Key。
// 当使用 option.WithHTTPClient 时，SDK 不会自动添加 API Key，需要手动注入。
type apiKeyTransport struct {
	base   http.RoundTripper
	apiKey string
}

func (t *apiKeyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	q := r.URL.Query()
	q.Set("key", t.apiKey)
	r.URL.RawQuery = q.Encode()
	return t.base.RoundTrip(r)
}

func buildTransport(proxy string) (*http.Transport, error) {
	trans := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   20,
		MaxConnsPerHost:       50,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     true,
	}

	if proxy != "" {
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			return nil, fmt.Errorf("gemini: invalid proxy URL: %w", err)
		}
		trans.Proxy = http.ProxyURL(proxyURL)
		// HTTP/2 通过 HTTP 代理（CONNECT 隧道）不稳定，容易导致连接无法归还连接池，
		// 累积后触发 "Too many open connections"。走代理时退回 HTTP/1.1 并缩短空闲超时。
		trans.ForceAttemptHTTP2 = false
		trans.IdleConnTimeout = 30 * time.Second
		log.Printf("[Gemini] 检测到代理 %s，已禁用 HTTP/2 并缩短空闲超时", proxy)
	}
	return trans, nil
}

// Name 返回提供者名称
func (p *Provider) Name() string {
	return "gemini"
}

// ModelName 返回当前主模型名称（供监控展示）
func (p *Provider) ModelName() string {
	return p.model
}

// UpstreamEndpoint 返回上游服务地址（供监控展示）
func (p *Provider) UpstreamEndpoint() string {
	if p.proxy != "" {
		return p.proxy
	}
	return "https://generativelanguage.googleapis.com"
}

// RawGenerate 执行一次原始请求，返回原始请求体、原始响应体和状态码（用于监控诊断）。
func (p *Provider) RawGenerate(ctx context.Context, prompt string) (string, string, int, error) {
	requestObj := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"role": "user",
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature": 0.7,
			"topP":        0.9,
		},
	}

	jsonBody, err := json.Marshal(requestObj)
	if err != nil {
		return "", "", 0, fmt.Errorf("gemini: failed to marshal request: %w", err)
	}

	endpoint := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", p.model, url.QueryEscape(p.apiKey))
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return string(jsonBody), "", 0, fmt.Errorf("gemini: failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.rawClient.Do(req)
	if err != nil {
		return string(jsonBody), "", 0, fmt.Errorf("gemini: request failed: %w", err)
	}
	defer resp.Body.Close()

	rawResp, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return string(jsonBody), string(rawResp), resp.StatusCode, fmt.Errorf("gemini: API error (status %d): %s", resp.StatusCode, string(rawResp))
	}

	return string(jsonBody), string(rawResp), resp.StatusCode, nil
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

// ChatWithTools 带工具的同步对话
func (p *Provider) ChatWithTools(ctx context.Context, messages []llm.Message, tools []llm.Tool) (*llm.Response, error) {
	contents, systemInstruction := convertMessagesToGeminiContents(messages)

	temperature := float32(0.7)
	topP := float32(0.9)
	reqBody := generateContentRequest{
		Contents: contents,
		Tools:    convertToolsToGeminiTools(tools),
		ToolConfig: &geminiToolConfig{
			FunctionCallingConfig: &geminiFunctionCallingConfig{
				Mode: "AUTO",
			},
		},
		GenerationConfig: &geminiGenerationConfig{
			Temperature: &temperature,
			TopP:        &topP,
		},
	}
	if systemInstruction != nil {
		reqBody.SystemInstruction = systemInstruction
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("gemini: failed to marshal tool request: %w", err)
	}

	endpoint := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", p.model, url.QueryEscape(p.apiKey))
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("gemini: failed to create tool request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.rawClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini: tool request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gemini: tool API error (status %d): %s", resp.StatusCode, string(body))
	}

	var geminiResp generateContentResponse
	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return nil, fmt.Errorf("gemini: failed to decode tool response: %w", err)
	}

	return parseToolResponse(&geminiResp)
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
	model := p.client.GenerativeModel(p.model)

	_, err := model.GenerateContent(ctx, genai.Text("ping"))
	if err != nil {
		return fmt.Errorf("gemini: health check failed: %w", err)
	}

	return nil
}

// Close 关闭客户端
func (p *Provider) Close() error {
	if p.clientTrans != nil {
		p.clientTrans.CloseIdleConnections()
	}
	if p.rawTrans != nil {
		p.rawTrans.CloseIdleConnections()
	}
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

func convertMessagesToGeminiContents(messages []llm.Message) ([]geminiContent, *geminiContent) {
	contents := make([]geminiContent, 0, len(messages))
	var systemInstruction *geminiContent

	for _, msg := range messages {
		text := strings.TrimSpace(msg.Content)
		if text == "" {
			continue
		}

		switch msg.Role {
		case "system":
			systemInstruction = &geminiContent{
				Parts: []geminiPart{{Text: text}},
			}
		case "user":
			contents = append(contents, geminiContent{
				Role:  "user",
				Parts: []geminiPart{{Text: text}},
			})
		case "assistant":
			contents = append(contents, geminiContent{
				Role:  "model",
				Parts: []geminiPart{{Text: text}},
			})
		}
	}

	return contents, systemInstruction
}

func convertToolsToGeminiTools(tools []llm.Tool) []geminiTool {
	if len(tools) == 0 {
		return nil
	}

	declarations := make([]geminiFunctionDeclaration, 0, len(tools))
	for _, tool := range tools {
		if tool.Type != "function" {
			continue
		}
		declarations = append(declarations, geminiFunctionDeclaration{
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
			Parameters:  tool.Function.Parameters,
		})
	}
	if len(declarations) == 0 {
		return nil
	}

	return []geminiTool{
		{FunctionDeclarations: declarations},
	}
}

func parseToolResponse(resp *generateContentResponse) (*llm.Response, error) {
	if resp == nil || len(resp.Candidates) == 0 {
		return nil, errors.New("gemini: no candidates in tool response")
	}

	candidate := resp.Candidates[0]
	if candidate.Content == nil {
		return &llm.Response{}, nil
	}

	var textParts []string
	toolCalls := make([]llm.ToolCall, 0)

	for idx, part := range candidate.Content.Parts {
		if strings.TrimSpace(part.Text) != "" {
			textParts = append(textParts, part.Text)
		}
		if part.FunctionCall != nil {
			args := "{}"
			if len(part.FunctionCall.Args) > 0 {
				if encoded, err := json.Marshal(part.FunctionCall.Args); err == nil {
					args = string(encoded)
				}
			}
			toolCalls = append(toolCalls, llm.ToolCall{
				ID:   fmt.Sprintf("gemini-call-%d", idx),
				Type: "function",
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{
					Name:      part.FunctionCall.Name,
					Arguments: args,
				},
			})
		}
	}

	finishReason := strings.ToLower(strings.TrimSpace(candidate.FinishReason))
	if finishReason == "" {
		finishReason = "stop"
	}

	tokensUsed := 0
	if resp.UsageMetadata != nil {
		tokensUsed = int(resp.UsageMetadata.TotalTokenCount)
	}

	return &llm.Response{
		Content:      strings.TrimSpace(strings.Join(textParts, "")),
		FinishReason: finishReason,
		TokensUsed:   tokensUsed,
		ToolCalls:    toolCalls,
	}, nil
}
