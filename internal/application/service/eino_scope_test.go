package service

import (
	"context"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"testing"
)

type einoKnowledgeStub struct {
	interfaces.KnowledgeService
	value *types.Knowledge
	reads int
}

func (s *einoKnowledgeStub) GetKnowledgeByIDOnly(context.Context, string) (*types.Knowledge, error) {
	s.reads++
	return s.value, nil
}

type einoChunkStub struct {
	interfaces.ChunkService
	value *types.Chunk
	reads int
}

func (s *einoChunkStub) GetChunkByID(context.Context, string) (*types.Chunk, error) {
	s.reads++
	return s.value, nil
}
func TestEinoEvidenceRejectsDisabledSource(t *testing.T) {
	for _, disabled := range []string{"knowledge", "chunk"} {
		t.Run(disabled, func(t *testing.T) {
			k := &einoKnowledgeStub{value: &types.Knowledge{ID: "doc", TenantID: 1, KnowledgeBaseID: "kb", EnableStatus: "enabled"}}
			c := &einoChunkStub{value: &types.Chunk{ID: "chunk", TenantID: 1, KnowledgeID: "doc", IsEnabled: true, Content: "canonical"}}
			if disabled == "knowledge" {
				k.value.EnableStatus = "disabled"
			} else {
				c.value.IsEnabled = false
			}
			s := &sessionService{knowledgeService: k, chunkService: c}
			cfg := &types.AgentConfig{SearchTargets: types.SearchTargets{&types.SearchTarget{Type: types.SearchTargetTypeKnowledgeBase, KnowledgeBaseID: "kb", TenantID: 1}}}
			if err := s.attachEinoScope(types.WithCaller(context.Background(), types.Caller{TenantID: 1, UserID: "user"}), cfg, 1, nil); err != nil {
				t.Fatal(err)
			}
			_, err := cfg.EinoFilterEvidence(types.WithCaller(context.Background(), types.Caller{TenantID: 1, UserID: "user"}), cfg.SearchTargets, []*types.SearchResult{&types.SearchResult{ID: "chunk", KnowledgeID: "doc", KnowledgeBaseID: "kb"}})
			if err == nil {
				t.Fatal("disabled evidence accepted")
			}
		})
	}
}
func TestEinoEvidenceRejectsOverbroadBeforeContentLookup(t *testing.T) {
	k := &einoKnowledgeStub{}
	c := &einoChunkStub{}
	s := &sessionService{knowledgeService: k, chunkService: c}
	cfg := &types.AgentConfig{SearchTargets: types.SearchTargets{&types.SearchTarget{Type: types.SearchTargetTypeKnowledge, KnowledgeIDs: []string{"doc"}, KnowledgeBaseID: "kb", TenantID: 1}}}
	if err := s.attachEinoScope(types.WithCaller(context.Background(), types.Caller{TenantID: 1, UserID: "user"}), cfg, 1, nil); err != nil {
		t.Fatal(err)
	}
	for _, r := range []*types.SearchResult{{ID: "chunk", KnowledgeID: "foreign", KnowledgeBaseID: "kb"}, {ID: "chunk", KnowledgeID: "doc", KnowledgeBaseID: "foreign"}, {ID: "chunk", KnowledgeBaseID: "kb"}} {
		if _, err := cfg.EinoFilterEvidence(types.WithCaller(context.Background(), types.Caller{TenantID: 1, UserID: "user"}), cfg.SearchTargets, []*types.SearchResult{r}); err == nil {
			t.Fatal("unauthorized source accepted")
		}
	}
	if k.reads != 0 || c.reads != 0 {
		t.Fatal("unauthorized source reached content lookup")
	}
}

func TestEinoScopeIntersectionNeverWidens(t *testing.T) {
	original := types.SearchTargets{&types.SearchTarget{KnowledgeBaseID: "kb", TenantID: 1, Type: types.SearchTargetTypeKnowledge, KnowledgeIDs: []string{"a"}}}
	now := types.SearchTargets{&types.SearchTarget{KnowledgeBaseID: "kb", TenantID: 1, Type: types.SearchTargetTypeKnowledgeBase}, &types.SearchTarget{KnowledgeBaseID: "foreign", TenantID: 2, Type: types.SearchTargetTypeKnowledgeBase}}
	out := intersectEinoTargets(original, now)
	if len(out) != 1 || out[0].Type != types.SearchTargetTypeKnowledge || len(out[0].KnowledgeIDs) != 1 || out[0].KnowledgeIDs[0] != "a" {
		t.Fatalf("scope widened: %+v", out)
	}
}
func TestEinoScopeRevoked(t *testing.T) {
	if len(intersectEinoTargets(types.SearchTargets{&types.SearchTarget{KnowledgeBaseID: "kb", TenantID: 1}}, nil)) != 0 {
		t.Fatal("revoked scope survived")
	}
}

func TestEinoScopeRequiresAuthenticatedHumanCaller(t *testing.T) {
	cfg := &types.AgentConfig{SearchTargets: types.SearchTargets{{KnowledgeBaseID: "kb", TenantID: 1, Type: types.SearchTargetTypeKnowledgeBase}}}
	if err := (&sessionService{}).attachEinoScope(context.Background(), cfg, 1, nil); err == nil {
		t.Fatal("missing caller accepted")
	}
}

func TestEinoScopeRejectsChangedCallerBeforeLookup(t *testing.T) {
	k := &einoKnowledgeStub{}
	c := &einoChunkStub{}
	s := &sessionService{knowledgeService: k, chunkService: c}
	cfg := &types.AgentConfig{SearchTargets: types.SearchTargets{{KnowledgeBaseID: "kb", TenantID: 1, Type: types.SearchTargetTypeKnowledgeBase}}}
	ctx := types.WithCaller(context.Background(), types.Caller{TenantID: 1, UserID: "original"})
	if err := s.attachEinoScope(ctx, cfg, 1, nil); err != nil {
		t.Fatal(err)
	}
	changed := types.WithCaller(context.Background(), types.Caller{TenantID: 1, UserID: "other"})
	if _, err := cfg.EinoRevalidate(changed); err == nil {
		t.Fatal("changed caller reauthorized")
	}
	if _, err := cfg.EinoFilterEvidence(changed, cfg.SearchTargets, []*types.SearchResult{{ID: "chunk", KnowledgeID: "doc", KnowledgeBaseID: "kb"}}); err == nil {
		t.Fatal("changed caller read evidence")
	}
	if k.reads != 0 || c.reads != 0 {
		t.Fatal("changed caller reached lookup")
	}
}
