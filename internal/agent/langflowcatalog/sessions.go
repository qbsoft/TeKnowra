// Package langflowcatalog exposes only design-time resource metadata.
package langflowcatalog

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type Resource struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}
type Service struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Tools []Tool `json:"tools"`
}
type SkillResource struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}
type Catalog struct {
	AllowSessionAttachments bool            `json:"allow_session_attachments,omitempty"`
	AgentID                 string          `json:"agent_id"`
	FlowID                  string          `json:"flow_id"`
	Models                  []Resource      `json:"models"`
	KnowledgeBases          []Resource      `json:"knowledge_bases"`
	Services                []Service       `json:"services"`
	Skills                  []SkillResource `json:"skills,omitempty"`
}
type directorySession struct {
	ctx     context.Context
	agentID string
	expires time.Time
}
type Sessions struct {
	mu    sync.Mutex
	items map[string]directorySession
	load  func(context.Context, string) (Catalog, error)
	now   func() time.Time
}

func NewSessions(load func(context.Context, string) (Catalog, error)) *Sessions {
	return &Sessions{items: map[string]directorySession{}, load: load, now: time.Now}
}
func (s *Sessions) Create(ctx context.Context, agent string) (string, time.Time, error) {
	if _, err := s.load(ctx, agent); err != nil {
		return "", time.Time{}, err
	}
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return "", time.Time{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for key, item := range s.items {
		if !item.expires.After(now) {
			delete(s.items, key)
		}
	}
	if len(s.items) >= 256 {
		return "", time.Time{}, fmt.Errorf("目录连接过多，请等待旧连接过期")
	}
	token := hex.EncodeToString(nonce)
	expires := now.Add(15 * time.Minute)
	s.items[token] = directorySession{ctx: context.WithoutCancel(ctx), agentID: agent, expires: expires}
	return token, expires, nil
}
func (s *Sessions) Read(ctx context.Context, token string) (Catalog, error) {
	s.mu.Lock()
	item, ok := s.items[token]
	if ok && !item.expires.After(s.now()) {
		delete(s.items, token)
		ok = false
	}
	s.mu.Unlock()
	if !ok || ctx.Err() != nil {
		return Catalog{}, fmt.Errorf("目录连接已失效，请从TeKnowra重新连接")
	}
	// Keep identity values, but honor cancellation of this directory request.
	call, cancel := context.WithCancel(item.ctx)
	defer cancel()
	stop := context.AfterFunc(ctx, cancel)
	defer stop()
	return s.load(call, item.agentID)
}
