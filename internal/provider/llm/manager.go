package llm

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Manager LLM 提供者管理器
type Manager struct {
	providers map[string]Provider
	primary   string
	mu        sync.RWMutex
}

// NewManager 创建管理器
func NewManager(primary string) *Manager {
	return &Manager{
		providers: make(map[string]Provider),
		primary:   primary,
	}
}

// Register 注册提供者
func (m *Manager) Register(provider Provider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers[provider.Name()] = provider
}

// Get 获取指定提供者
func (m *Manager) Get(name string) (Provider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	provider, ok := m.providers[name]
	if !ok {
		return nil, fmt.Errorf("llm: provider %s not found", name)
	}
	return provider, nil
}

// Primary 获取主要提供者
func (m *Manager) Primary() (Provider, error) {
	return m.Get(m.primary)
}

// SetPrimary 设置主要提供者
func (m *Manager) SetPrimary(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.providers[name]; !ok {
		return fmt.Errorf("llm: provider %s not registered", name)
	}
	m.primary = name
	return nil
}

// Chat 使用主要提供者进行对话
func (m *Manager) Chat(ctx context.Context, messages []Message) (*Response, error) {
	provider, err := m.Primary()
	if err != nil {
		return nil, err
	}
	return provider.Chat(ctx, messages)
}

// ChatStream 使用主要提供者进行流式对话
func (m *Manager) ChatStream(ctx context.Context, messages []Message) (<-chan Chunk, error) {
	provider, err := m.Primary()
	if err != nil {
		return nil, err
	}
	return provider.ChatStream(ctx, messages)
}

// ChatWith 使用指定提供者进行对话
func (m *Manager) ChatWith(ctx context.Context, providerName string, messages []Message) (*Response, error) {
	provider, err := m.Get(providerName)
	if err != nil {
		return nil, err
	}
	return provider.Chat(ctx, messages)
}

// ChatStreamWith 使用指定提供者进行流式对话
func (m *Manager) ChatStreamWith(ctx context.Context, providerName string, messages []Message) (<-chan Chunk, error) {
	provider, err := m.Get(providerName)
	if err != nil {
		return nil, err
	}
	return provider.ChatStream(ctx, messages)
}

// HealthCheck 检查所有提供者健康状态
func (m *Manager) HealthCheck(ctx context.Context) map[string]error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make(map[string]error)
	var wg sync.WaitGroup

	for name, provider := range m.providers {
		wg.Add(1)
		go func(n string, p Provider) {
			defer wg.Done()
			results[n] = p.HealthCheck(ctx)
		}(name, provider)
	}

	wg.Wait()
	return results
}

// HealthCheckPrimary 检查主要提供者健康状态
func (m *Manager) HealthCheckPrimary(ctx context.Context) error {
	provider, err := m.Primary()
	if err != nil {
		return err
	}
	return provider.HealthCheck(ctx)
}

// Available 返回可用的提供者列表
func (m *Manager) Available() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var names []string
	for name := range m.providers {
		names = append(names, name)
	}
	return names
}

// ChatWithFallback 带降级的对话，主提供者失败时尝试其他提供者
func (m *Manager) ChatWithFallback(ctx context.Context, messages []Message) (*Response, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 先尝试主提供者
	if primary, ok := m.providers[m.primary]; ok {
		resp, err := primary.Chat(ctx, messages)
		if err == nil {
			return resp, nil
		}
	}

	// 尝试其他提供者
	var lastErr error
	for name, provider := range m.providers {
		if name == m.primary {
			continue
		}
		resp, err := provider.Chat(ctx, messages)
		if err == nil {
			return resp, nil
		}
		lastErr = err
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("llm: no available providers")
}
