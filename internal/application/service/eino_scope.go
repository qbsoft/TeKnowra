package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Tencent/WeKnora/internal/types"
	"slices"
)

func cloneEinoTargets(in types.SearchTargets) types.SearchTargets {
	b, _ := json.Marshal(in)
	var out types.SearchTargets
	_ = json.Unmarshal(b, &out)
	return out
}

// Intersections retain the initially authorized upper bound, including tags.
func intersectEinoTargets(original, current types.SearchTargets) types.SearchTargets {
	out := types.SearchTargets{}
	for _, a := range original {
		if a == nil {
			continue
		}
		for _, b := range current {
			if b == nil || a.KnowledgeBaseID != b.KnowledgeBaseID || a.TenantID != b.TenantID {
				continue
			}
			v := *a
			v.KnowledgeIDs = slices.Clone(a.KnowledgeIDs)
			v.TagIDs = slices.Clone(a.TagIDs)
			v.ScopeTagIDs = slices.Clone(a.ScopeTagIDs)
			if a.Type == types.SearchTargetTypeKnowledge || b.Type == types.SearchTargetTypeKnowledge {
				v.Type = types.SearchTargetTypeKnowledge
				if a.Type == types.SearchTargetTypeKnowledgeBase {
					v.KnowledgeIDs = slices.Clone(b.KnowledgeIDs)
				} else if b.Type == types.SearchTargetTypeKnowledge {
					v.KnowledgeIDs = nil
					for _, id := range a.KnowledgeIDs {
						if slices.Contains(b.KnowledgeIDs, id) {
							v.KnowledgeIDs = append(v.KnowledgeIDs, id)
						}
					}
				}
				if len(v.KnowledgeIDs) == 0 {
					continue
				}
			}
			// Non-document index tag scopes are outside the first pilot.
			if len(v.TagIDs) > 0 || len(b.TagIDs) > 0 {
				continue
			}
			out = append(out, &v)
			break
		}
	}
	return out
}

func (s *sessionService) attachEinoScope(ctx context.Context, cfg *types.AgentConfig, tenant uint64, tags []types.TagScope) error {
	if err := s.attachAuthorizedKnowledgeScope(ctx, cfg, tenant, tags); err != nil {
		return err
	}
	cfg.EinoRevalidate = cfg.RuntimeRevalidate
	cfg.EinoFilterEvidence = cfg.RuntimeFilterEvidence
	return nil
}
func (s *sessionService) attachAuthorizedKnowledgeScope(ctx context.Context, cfg *types.AgentConfig, tenant uint64, tags []types.TagScope) error {
	original := cloneEinoTargets(cfg.SearchTargets)
	caller := types.CallerFromContext(ctx)
	if caller.TenantID == 0 || caller.UserID == "" {
		return fmt.Errorf("合同审查 V2 试点需要已登录的用户身份")
	}
	if len(original) == 0 {
		return fmt.Errorf("合同审查 V2 需要选择合同或政策知识库")
	}
	kbIDs := slices.Clone(cfg.KnowledgeBases)
	docIDs := slices.Clone(cfg.KnowledgeIDs)
	b, _ := json.Marshal(tags)
	var savedTags []types.TagScope
	_ = json.Unmarshal(b, &savedTags)
	cfg.RuntimeRevalidate = func(runCtx context.Context) (types.SearchTargets, error) {
		if types.CallerFromContext(runCtx) != caller {
			return nil, fmt.Errorf("review caller changed")
		}
		// Drop cached grants; organization sharing is read again on each boundary.
		fresh := context.WithValue(runCtx, types.KBGrantsContextKey, struct{}{})
		fresh = context.WithValue(fresh, types.SharedAgentGrantContextKey, struct{}{})
		if cfg.SharedAgentReadOnly {
			var err error
			fresh, err = s.refreshLangflowSharedKnowledgeContext(fresh, cfg)
			if err != nil {
				return nil, err
			}
		}
		current, e := s.buildSearchTargets(fresh, tenant, kbIDs, docIDs, savedTags)
		if e != nil {
			return nil, e
		}
		out := intersectEinoTargets(original, current)
		if len(out) == 0 {
			return nil, fmt.Errorf("合同审查 V2 的知识授权已失效")
		}
		return out, nil
	}
	cfg.RuntimeFilterEvidence = func(runCtx context.Context, scope types.SearchTargets, refs []*types.SearchResult) ([]*types.SearchResult, error) {
		if types.CallerFromContext(runCtx) != caller {
			return nil, fmt.Errorf("review caller changed")
		}
		out := []*types.SearchResult{}
		seen := map[string]bool{}
		for _, r := range refs {
			if r == nil || r.ID == "" || r.KnowledgeID == "" || r.KnowledgeBaseID == "" {
				return nil, fmt.Errorf("retrieval result missing provenance")
			}
			var target *types.SearchTarget
			for _, v := range scope {
				if v != nil && v.KnowledgeBaseID == r.KnowledgeBaseID && (v.Type == types.SearchTargetTypeKnowledgeBase || slices.Contains(v.KnowledgeIDs, r.KnowledgeID)) {
					target = v
					break
				}
			}
			if target == nil {
				return nil, fmt.Errorf("retrieval result outside authorized scope")
			}
			// Scope is checked before looking up source metadata. Verify ownership,
			// then fetch canonical chunk content in the scoped tenant.
			k, e := s.knowledgeService.GetKnowledgeByIDOnly(runCtx, r.KnowledgeID)
			if e != nil || k == nil || k.TenantID != target.TenantID || k.KnowledgeBaseID != target.KnowledgeBaseID || k.EnableStatus != "enabled" || k.DeletedAt.Valid {
				return nil, fmt.Errorf("knowledge source no longer valid")
			}
			c, e := s.chunkService.GetChunkByID(types.WithExecutionTenant(runCtx, target.TenantID), r.ID)
			if e != nil || c == nil || c.TenantID != target.TenantID || c.KnowledgeID != r.KnowledgeID || !c.IsEnabled || c.DeletedAt.Valid {
				return nil, fmt.Errorf("chunk source no longer valid")
			}
			if !seen[r.ID] {
				copyResult := *r
				copyResult.Content = c.Content
				copyResult.KnowledgeTitle = k.Title
				out = append(out, &copyResult)
				seen[r.ID] = true
			}
		}
		return out, nil
	}
	return nil
}
