package langflowrelease

import (
	"fmt"
	"github.com/Tencent/WeKnora/internal/agent/langflowdef"
)

func (s *Store) Draft(agent string) (langflowdef.Binding, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.doc.Drafts[agent]
	return b.Clone(), ok
}
func (s *Store) Drafts() []langflowdef.Binding {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []langflowdef.Binding{}
	for _, b := range s.doc.Drafts {
		out = append(out, b.Clone())
	}
	return out
}
func (s *Store) PutDraft(b langflowdef.Binding) error {
	if err := b.Validate(); err != nil {
		return err
	}
	if b.ReleaseID != "" || b.GraphDigest != "" || b.RuntimeOwnerID != "" {
		return fmt.Errorf("draft cannot contain runtime credentials")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if prior, ok := s.doc.Drafts[b.AgentID]; ok && prior.WorkspaceID != b.WorkspaceID {
		return fmt.Errorf("draft workspace cannot change")
	}
	for _, r := range s.doc.Releases {
		if r.AgentID == b.AgentID && r.WorkspaceID != b.WorkspaceID {
			return fmt.Errorf("release workspace cannot change")
		}
	}
	d := s.copyDocument()
	d.Drafts[b.AgentID] = b.Clone()
	if len(d.Drafts) > 1000 {
		return fmt.Errorf("draft retention limit reached")
	}
	return s.persist(d)
}
