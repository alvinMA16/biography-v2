package llm

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeProvider struct {
	name      string
	delay     time.Duration
	resp      *Response
	err       error
	callCount int
	mu        sync.Mutex
}

func (p *fakeProvider) Name() string {
	return p.name
}

func (p *fakeProvider) Chat(ctx context.Context, messages []Message) (*Response, error) {
	p.mu.Lock()
	p.callCount++
	p.mu.Unlock()

	if p.delay > 0 {
		timer := time.NewTimer(p.delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
		}
	}

	if p.err != nil {
		return nil, p.err
	}
	return p.resp, nil
}

func (p *fakeProvider) ChatWithTools(ctx context.Context, messages []Message, tools []Tool) (*Response, error) {
	return p.Chat(ctx, messages)
}

func (p *fakeProvider) ChatStream(ctx context.Context, messages []Message) (<-chan Chunk, error) {
	ch := make(chan Chunk, 1)
	close(ch)
	return ch, nil
}

func (p *fakeProvider) HealthCheck(ctx context.Context) error {
	return nil
}

func (p *fakeProvider) Calls() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.callCount
}

func TestHedgedChatReturnsPrimaryBeforeHedge(t *testing.T) {
	manager := NewManager("primary")
	primary := &fakeProvider{
		name:  "primary",
		delay: 5 * time.Millisecond,
		resp:  &Response{Content: "primary"},
	}
	hedge := &fakeProvider{
		name:  "hedge",
		delay: 1 * time.Millisecond,
		resp:  &Response{Content: "hedge"},
	}
	manager.Register(primary)
	manager.Register(hedge)

	resp, provider, err := manager.HedgedChat(context.Background(), []Message{{Role: "user", Content: "hi"}}, HedgedChatConfig{
		HedgeAfter:     20 * time.Millisecond,
		HedgeProviders: []string{"hedge"},
	})
	if err != nil {
		t.Fatalf("HedgedChat returned error: %v", err)
	}
	if provider != "primary" {
		t.Fatalf("expected primary winner, got %s", provider)
	}
	if resp == nil || resp.Content != "primary" {
		t.Fatalf("expected primary response, got %+v", resp)
	}
	if hedge.Calls() != 0 {
		t.Fatalf("expected hedge not to launch, got %d call(s)", hedge.Calls())
	}
}

func TestHedgedChatLaunchesHedgeAfterDelay(t *testing.T) {
	manager := NewManager("primary")
	primary := &fakeProvider{
		name:  "primary",
		delay: 40 * time.Millisecond,
		resp:  &Response{Content: "primary"},
	}
	hedge := &fakeProvider{
		name:  "hedge",
		delay: 5 * time.Millisecond,
		resp:  &Response{Content: "hedge"},
	}
	manager.Register(primary)
	manager.Register(hedge)

	startedAt := time.Now()
	resp, provider, err := manager.HedgedChat(context.Background(), []Message{{Role: "user", Content: "hi"}}, HedgedChatConfig{
		HedgeAfter:     10 * time.Millisecond,
		HedgeProviders: []string{"hedge"},
	})
	if err != nil {
		t.Fatalf("HedgedChat returned error: %v", err)
	}
	if provider != "hedge" {
		t.Fatalf("expected hedge winner, got %s", provider)
	}
	if resp == nil || resp.Content != "hedge" {
		t.Fatalf("expected hedge response, got %+v", resp)
	}
	if hedge.Calls() != 1 {
		t.Fatalf("expected hedge to launch once, got %d call(s)", hedge.Calls())
	}
	if elapsed := time.Since(startedAt); elapsed >= primary.delay {
		t.Fatalf("expected hedge to finish before primary, elapsed=%s primary_delay=%s", elapsed, primary.delay)
	}
}

func TestHedgedChatLaunchesHedgeImmediatelyOnPrimaryError(t *testing.T) {
	manager := NewManager("primary")
	primary := &fakeProvider{
		name:  "primary",
		delay: 2 * time.Millisecond,
		err:   errors.New("boom"),
	}
	hedge := &fakeProvider{
		name:  "hedge",
		delay: 4 * time.Millisecond,
		resp:  &Response{Content: "hedge"},
	}
	manager.Register(primary)
	manager.Register(hedge)

	startedAt := time.Now()
	resp, provider, err := manager.HedgedChat(context.Background(), []Message{{Role: "user", Content: "hi"}}, HedgedChatConfig{
		HedgeAfter:     50 * time.Millisecond,
		HedgeProviders: []string{"hedge"},
	})
	if err != nil {
		t.Fatalf("HedgedChat returned error: %v", err)
	}
	if provider != "hedge" {
		t.Fatalf("expected hedge winner, got %s", provider)
	}
	if resp == nil || resp.Content != "hedge" {
		t.Fatalf("expected hedge response, got %+v", resp)
	}
	if elapsed := time.Since(startedAt); elapsed >= 50*time.Millisecond {
		t.Fatalf("expected hedge to start before timer, elapsed=%s", elapsed)
	}
}
