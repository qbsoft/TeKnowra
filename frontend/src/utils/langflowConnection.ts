export function localLangflowEditorUrl(flowId: string, editorOrigin?: string): string {
  if (!/^[A-Za-z0-9_-]+$/.test(flowId)) throw new Error('Langflow 流程标识无效');
  const configured = editorOrigin ?? (typeof window === 'undefined' ? '' : window.__RUNTIME_CONFIG__?.LANGFLOW_EDITOR_URL);
  const url = new URL(configured?.trim() || 'http://127.0.0.1:7861');
  const localHttp = url.protocol === 'http:' && ['localhost', '127.0.0.1', '[::1]'].includes(url.hostname);
  if ((!localHttp && url.protocol !== 'https:') || url.username || url.password
    || url.pathname !== '/' || url.search || url.hash) {
    throw new Error('Langflow 编排地址配置无效，请联系管理员');
  }
  return `${url.origin}/flow/${flowId}`;
}
export function canTrialLangflow(info: { bound: boolean; release_enabled?: boolean; active_release_id?: string } | null): boolean {
  return Boolean(info?.bound && (!info.release_enabled || info.active_release_id));
}
