import { get, post } from '@/utils/request';
export interface LangflowInfo { bound: boolean; setup_enabled?: boolean; managed?: boolean; flow_id?: string; release_enabled?: boolean; active_release_id?: string; active_release_label?: string; structured_scripts?: StructuredScript[] }
export interface StructuredScript { skill: string; path: string; input_schema: Record<string,unknown> }
export interface LangflowDraftOption { agent_id: string; flow_id: string; name: string }
export function listLangflowDraftOptions(agentId: string) {return get<{data:LangflowDraftOption[]}>(`/api/v1/agents/${encodeURIComponent(agentId)}/langflow/draft-options`);}
export function setupLangflowDraft(agentId: string, mode: 'create'|'associate'|'sync',flowId:string, scripts:StructuredScript[]) {return post(`/api/v1/agents/${encodeURIComponent(agentId)}/langflow/draft`,{mode,flow_id:flowId,structured_scripts:scripts});}
export interface LangflowRelease { id: string; label: string; flow_id: string; created_at: string; graph_digest: string }
export function listLangflowReleases(agentId: string) {
  return get<{ data: { releases: LangflowRelease[]; active_release_id: string } }>(`/api/v1/agents/${encodeURIComponent(agentId)}/langflow/releases`);
}
export function publishLangflow(agentId: string, label: string) {
  return post<{ data: LangflowRelease }>(`/api/v1/agents/${encodeURIComponent(agentId)}/langflow/releases`, { label });
}
export function activateLangflowRelease(agentId: string, releaseId: string) {
  return post(`/api/v1/agents/${encodeURIComponent(agentId)}/langflow/releases/${encodeURIComponent(releaseId)}/activate`);
}
export interface DesignConnection { directory_url: string; directory_ticket: string; expires_at: string; scope: string }
export function getLangflowInfo(agentId: string) {
  return get<{ data: LangflowInfo }>(`/api/v1/agents/${encodeURIComponent(agentId)}/langflow`);
}
export function createLangflowDesignConnection(agentId: string) {
  return post<{ data: DesignConnection }>(`/api/v1/agents/${encodeURIComponent(agentId)}/langflow/design-session`);
}
