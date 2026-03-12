package llm

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// Manager LLM 提供者管理器
type Manager struct {
	providers map[string]Provider
	primary   string
	mu        sync.RWMutex
}

type HedgedChatConfig struct {
	HedgeAfter     time.Duration
	HedgeProviders []string
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

// ChatWithRetry 使用主要提供者进行对话，失败时有限重试
func (m *Manager) ChatWithRetry(ctx context.Context, messages []Message, attempts int) (*Response, string, error) {
	if attempts <= 0 {
		attempts = 1
	}

	provider, err := m.Primary()
	if err != nil {
		return nil, "", err
	}

	var lastErr error
	for i := 1; i <= attempts; i++ {
		resp, err := provider.Chat(ctx, messages)
		if err == nil {
			return resp, provider.Name(), nil
		}
		lastErr = err
		if i < attempts {
			log.Printf("[LLM] provider=%s chat failed, retrying (%d/%d): %v", provider.Name(), i, attempts, err)
		}
	}

	return nil, provider.Name(), lastErr
}

func (m *Manager) HedgedChat(ctx context.Context, messages []Message, cfg HedgedChatConfig) (*Response, string, error) {
	primary, err := m.Primary()
	if err != nil {
		return nil, "", err
	}

	if cfg.HedgeAfter <= 0 {
		cfg.HedgeAfter = 9 * time.Second
	}

	hedgeProviders := m.resolveProviders(cfg.HedgeProviders, primary.Name())
	if len(hedgeProviders) == 0 {
		resp, err := primary.Chat(ctx, messages)
		if err != nil {
			return nil, primary.Name(), err
		}
		return resp, primary.Name(), nil
	}

	type result struct {
		provider string
		resp     *Response
		err      error
		elapsed  time.Duration
	}

	startedAt := time.Now()
	runCtx, cancelAll := context.WithCancel(ctx)
	defer cancelAll()

	results := make(chan result, 1+len(hedgeProviders))
	cancelMu := sync.Mutex{}
	cancelFns := make([]context.CancelFunc, 0, 1+len(hedgeProviders))
	launchMu := sync.Mutex{}
	totalLaunched := 0
	hedgesLaunched := false

	launchProvider := func(provider Provider) {
		reqCtx, cancel := context.WithCancel(runCtx)
		cancelMu.Lock()
		cancelFns = append(cancelFns, cancel)
		cancelMu.Unlock()

		go func() {
			callStartedAt := time.Now()
			resp, err := provider.Chat(reqCtx, messages)
			results <- result{
				provider: provider.Name(),
				resp:     resp,
				err:      err,
				elapsed:  time.Since(callStartedAt),
			}
		}()
	}

	launchHedges := func(reason string) {
		launchMu.Lock()
		defer launchMu.Unlock()
		if hedgesLaunched {
			return
		}
		hedgesLaunched = true
		totalLaunched += len(hedgeProviders)
		log.Printf("[LLM] hedged chat launching %d hedge request(s): primary=%s reason=%s after=%s", len(hedgeProviders), primary.Name(), reason, time.Since(startedAt).Round(time.Millisecond))
		for _, provider := range hedgeProviders {
			launchProvider(provider)
		}
	}

	totalLaunched = 1
	launchProvider(primary)
	timer := time.NewTimer(cfg.HedgeAfter)
	defer timer.Stop()

	var errorMessages []string
	completed := 0

	for {
		select {
		case <-ctx.Done():
			cancelMu.Lock()
			for _, cancel := range cancelFns {
				cancel()
			}
			cancelMu.Unlock()
			return nil, "", ctx.Err()
		case <-timer.C:
			launchHedges("slow_primary")
		case res := <-results:
			completed++
			if res.err == nil && res.resp != nil {
				cancelMu.Lock()
				for _, cancel := range cancelFns {
					cancel()
				}
				cancelMu.Unlock()
				log.Printf("[LLM] hedged chat winner=%s total_elapsed=%s call_elapsed=%s", res.provider, time.Since(startedAt).Round(time.Millisecond), res.elapsed.Round(time.Millisecond))
				return res.resp, res.provider, nil
			}

			errorMessages = append(errorMessages, fmt.Sprintf("%s: %v", res.provider, res.err))
			if res.provider == primary.Name() {
				launchHedges("primary_error")
			}

			launchMu.Lock()
			done := completed >= totalLaunched
			launchMu.Unlock()
			if done {
				return nil, primary.Name(), fmt.Errorf("llm hedged chat failed after %s: %s", time.Since(startedAt).Round(time.Millisecond), strings.Join(errorMessages, "; "))
			}
		}
	}
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

func (m *Manager) resolveProviders(names []string, exclude string) []Provider {
	m.mu.RLock()
	defer m.mu.RUnlock()

	providers := make([]Provider, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		if name == "" || name == exclude {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		provider, ok := m.providers[name]
		if !ok {
			continue
		}
		seen[name] = struct{}{}
		providers = append(providers, provider)
	}
	return providers
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
