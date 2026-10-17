// Package langflowrelease stores immutable release metadata and atomic active pointers.
package langflowrelease

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/Tencent/WeKnora/internal/agent/langflowdef"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type Release struct {
	ID               string              `json:"id"`
	Label            string              `json:"label"`
	AgentID          string              `json:"agent_id"`
	WorkspaceID      uint64              `json:"workspace_id"`
	EditorFlowID     string              `json:"editor_flow_id"`
	Binding          langflowdef.Binding `json:"binding"`
	GraphDigest      string              `json:"graph_digest"`
	ToolsDigest      string              `json:"tools_digest"`
	ComponentsDigest string              `json:"components_digest"`
	RuntimeOwnerID   string              `json:"runtime_owner_id"`
	CreatedAt        time.Time           `json:"created_at"`
}

func (r Release) Clone() Release { r.Binding = r.Binding.Clone(); return r }

type document struct {
	Drafts   map[string]langflowdef.Binding `json:"drafts,omitempty"`
	Releases map[string]Release             `json:"releases"`
	Active   map[string]string              `json:"active"`
}
type Store struct {
	mu   sync.RWMutex
	path string
	doc  document
}

const maxStoreBytes = 16 * 1024 * 1024

func Open(path string) (*Store, error) {
	if path == "" {
		return nil, fmt.Errorf("release store path required")
	}
	s := &Store{path: path, doc: document{Releases: map[string]Release{}, Active: map[string]string{}}}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil || len(raw) > maxStoreBytes {
		return nil, fmt.Errorf("release store unavailable")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&s.doc) != nil || d.Decode(new(any)) != io.EOF || s.doc.Releases == nil || s.doc.Active == nil {
		return nil, fmt.Errorf("invalid release store")
	}
	for id, b := range s.doc.Drafts {
		if id != b.AgentID || b.ReleaseID != "" || b.GraphDigest != "" || b.RuntimeOwnerID != "" || b.Validate() != nil {
			return nil, fmt.Errorf("invalid managed draft")
		}
	}
	for id, r := range s.doc.Releases {
		if id != r.ID || validRelease(r) != nil {
			return nil, fmt.Errorf("invalid stored release")
		}
	}
	for agent, id := range s.doc.Active {
		r, ok := s.doc.Releases[id]
		if !ok || r.AgentID != agent {
			return nil, fmt.Errorf("invalid active release")
		}
	}
	return s, nil
}
func validRelease(r Release) error {
	for _, digest := range []string{r.GraphDigest, r.ToolsDigest, r.ComponentsDigest} {
		raw, err := hex.DecodeString(digest)
		if err != nil || len(raw) != 32 || digest != hex.EncodeToString(raw) {
			return fmt.Errorf("invalid release fingerprint")
		}
	}
	if r.ID == "" || len(r.ID) > 100 || r.AgentID != r.Binding.AgentID || r.WorkspaceID != r.Binding.WorkspaceID || r.EditorFlowID == "" || r.RuntimeOwnerID == "" || r.GraphDigest == "" || r.ToolsDigest == "" || r.ComponentsDigest == "" {
		return fmt.Errorf("invalid release metadata")
	}
	return r.Binding.Validate()
}
func (s *Store) copyDocument() document {
	d := document{Releases: map[string]Release{}, Active: map[string]string{}, Drafts: map[string]langflowdef.Binding{}}
	for k, v := range s.doc.Drafts {
		d.Drafts[k] = v.Clone()
	}
	for k, v := range s.doc.Releases {
		d.Releases[k] = v.Clone()
	}
	for k, v := range s.doc.Active {
		d.Active[k] = v
	}
	return d
}
func (s *Store) persist(d document) error {
	if len(d.Releases) > 1000 {
		return fmt.Errorf("release retention limit reached")
	}
	raw, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	if len(raw) > maxStoreBytes {
		return fmt.Errorf("release store size limit reached")
	}
	if err = os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return fmt.Errorf("release store write failed")
	}
	file, err := os.CreateTemp(filepath.Dir(s.path), ".release-*")
	if err != nil {
		return fmt.Errorf("release store write failed")
	}
	name := file.Name()
	defer os.Remove(name)
	if _, err = file.Write(raw); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil || os.Rename(name, s.path) != nil {
		return fmt.Errorf("release store write failed")
	}
	s.doc = d
	return nil
}
func (s *Store) Add(r Release) error {
	if err := validRelease(r); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.doc.Releases[r.ID]; ok {
		return fmt.Errorf("release is immutable")
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now().UTC()
	}
	d := s.copyDocument()
	d.Releases[r.ID] = r.Clone()
	return s.persist(d)
}
func (s *Store) Activate(agent, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.doc.Releases[id]
	if !ok || r.AgentID != agent {
		return fmt.Errorf("release does not belong to agent")
	}
	d := s.copyDocument()
	d.Active[agent] = id
	return s.persist(d)
}
func (s *Store) Active(agent string) (Release, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.doc.Releases[s.doc.Active[agent]]
	return r.Clone(), ok
}
func (s *Store) Get(agent, id string) (Release, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.doc.Releases[id]
	return r.Clone(), ok && r.AgentID == agent
}
func (s *Store) List(agent string) []Release {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows := []Release{}
	for _, r := range s.doc.Releases {
		if r.AgentID == agent {
			rows = append(rows, r.Clone())
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].CreatedAt.After(rows[j].CreatedAt) })
	return rows
}

// Digest normalizes JSON objects without losing integer precision.
func Digest(raw []byte) (string, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var value any
	if err := d.Decode(&value); err != nil {
		return "", err
	}
	if d.Decode(new(any)) != io.EOF {
		return "", fmt.Errorf("one JSON value required")
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(canonical)
	return hex.EncodeToString(hash[:]), nil
}
