export function localLangflowEditorUrl(flowId: string): string {
  if (!/^[A-Za-z0-9_-]+$/.test(flowId)) throw new Error('Langflow 流程标识无效');
  return `http://127.0.0.1:7861/flow/${flowId}`;
}
export function canTrialLangflow(info: { bound: boolean; release_enabled?: boolean; active_release_id?: string } | null): boolean {
  return Boolean(info?.bound && (!info.release_enabled || info.active_release_id));
}
