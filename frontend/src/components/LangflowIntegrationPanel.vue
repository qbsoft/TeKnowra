<template>
  <div v-if="info?.bound || executionEngine === 'langflow'" class="settings-group langflow-integration">
    <p v-if="!info?.setup_enabled">Langflow接入暂不可用，请检查当前编辑权限与服务配置。智能体可保存，完成接入与发布前不能执行。</p>
    <div v-if="info?.setup_enabled" class="draft-setup">
      <h3>Langflow 流程接入</h3>
      <p>使用本页已保存的模型、指定知识库、选定 MCP 工具和技能。下方操作会先保存当前资源配置，再生成或校验流程绑定，无须改后台文件。</p>
      <p>新建流程会用本页提示词初始化 Agent Instructions；后续执行以画布中发布的指令为准。</p>
      <t-select v-model="setupMode" aria-label="流程接入方式" :options="[{label:'创建通用流程',value:'create'},{label:'关联已有流程的独立副本',value:'associate'}]" />
      <t-select v-if="setupMode === 'associate'" v-model="sourceFlow" aria-label="关联流程" :options="flowOptions" placeholder="选择当前空间有权配置的编辑流程" />
      <details><summary>计算脚本授权（高级，可选）</summary><p>仅填写审核过的固定技能入口与 JSON 输入 schema，格式为数组，默认 []。沿用已安装技能的版本和权限，不提供任意 shell 或原件文件读取。</p><t-textarea v-model="scriptsText" aria-label="计算脚本授权" :autosize="{minRows:3,maxRows:10}" /></details>
      <div class="draft-actions"><t-button :loading="settingUp" @click="setupDraft(setupMode)">{{ setupMode === 'create' ? (info.bound ? '生成新的编辑草稿' : '创建并接入流程') : '关联独立副本' }}</t-button><t-button v-if="info.bound" variant="outline" :loading="settingUp" @click="setupDraft('sync')">同步并校验资源配置</t-button></div>
      <p v-if="info.bound">新草稿和资源绑定不会覆盖已发布运行图。完成编排后发布新版本；不需要时保留旧版本。</p>
    </div>
    <div v-if="info?.bound">
    <div class="setting-row">
      <div class="setting-info">
        <label>Langflow 研发编排</label>
        <p class="desc">在画布中配置模型、知识库、MCP 和专家。试运行使用当前员工的 TeKnowra 权限。</p>
        <p v-if="connectedUntil" class="desc">目录连接有效至 {{ connectedUntil }}；仅可读取资源元数据。</p>
      </div>
      <div class="setting-control">
        <t-button variant="outline" @click="openEditor">打开编排</t-button>
        <t-button variant="outline" :loading="connecting" @click="copyDirectory">复制目录连接</t-button>
        <t-button theme="primary" :disabled="!canTrialLangflow(info)" @click="trial">在聊天中试运行</t-button>
      </div>
    </div>
    <p class="desc">将目录连接粘贴到 TeKnowra 组件的连接字段，刷新后字段清空。目录失效时重新连接；不要使用该连接执行业务工具。</p>
    <div v-if="info.release_enabled" class="release-controls">
      <p>当前发布版本：{{ info.active_release_label || '尚未发布' }} <small>{{ info.active_release_id }}</small></p>
      <p class="desc">打开编排修改草稿；发布后新任务使用新版本，已启动的任务保持原版本。选择旧版本并激活可回退。</p>
      <t-input v-model="releaseLabel" placeholder="发布名称，例如 V1" :maxlength="64" aria-label="发布名称" />
      <t-button :loading="publishing" :disabled="!releaseLabel.trim()" @click="publish">发布当前草稿</t-button>
      <t-select v-model="selectedRelease" :options="releaseOptions" placeholder="选择已发布版本" aria-label="已发布版本" />
      <t-button variant="outline" :loading="activating" :disabled="!selectedRelease || selectedRelease === info.active_release_id" @click="activate">激活所选版本</t-button>
    </div>
    </div>
  </div>
</template>
<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { useRouter } from 'vue-router';
import { MessagePlugin } from 'tdesign-vue-next';
import { getLangflowInfo, createLangflowDesignConnection, listLangflowReleases, publishLangflow, activateLangflowRelease, listLangflowDraftOptions, setupLangflowDraft, type StructuredScript, type LangflowInfo, type LangflowRelease, type LangflowDraftOption } from '@/api/langflow';
import { localLangflowEditorUrl, canTrialLangflow } from '@/utils/langflowConnection';
import { copyWithToast } from '@/utils/clipboard';
import { useSettingsStore } from '@/stores/settings';
const props = defineProps<{ agentId: string; executionEngine?: string; saveAgent?: () => Promise<void> }>();
const emit = defineEmits<{ (e: 'bound',value:boolean):void }>();
const settingUp=ref(false);
const setupMode=ref<'create'|'associate'>('create');
const sourceFlow=ref('');
const draftOptions=ref<LangflowDraftOption[]>([]);
const scriptsText=ref('[]');
const flowOptions=computed(()=>draftOptions.value.map(o=>({label:o.name+' · '+o.flow_id.slice(0,8),value:o.flow_id})));
const info = ref<LangflowInfo | null>(null);
const connecting = ref(false);
const connectedUntil = ref('');
const releases = ref<LangflowRelease[]>([]);
const releaseLabel = ref('');
const selectedRelease = ref('');
const publishing = ref(false);
const activating = ref(false);
const releaseOptions = computed(() => releases.value.map(r => ({ value: r.id, label: `${r.label} · ${r.id.slice(0, 8)}${r.id === info.value?.active_release_id ? '（当前）' : ''}` })));
const router = useRouter();
async function refreshInfo(id:string) {
 const response=await getLangflowInfo(id);
 if(props.agentId!==id)return;
 info.value=response.data;emit('bound',info.value.bound);
 scriptsText.value=JSON.stringify(info.value.structured_scripts || [],null,2);
 if(info.value.setup_enabled){const result=await listLangflowDraftOptions(id);draftOptions.value=result.data;}
 if(info.value.bound && info.value.release_enabled)await refreshReleases(id);
}
watch(() => [props.agentId, props.executionEngine], async () => {
  const id=props.agentId;
  info.value = null; connectedUntil.value = ''; releases.value = []; selectedRelease.value = ''; releaseLabel.value = '';
  if (!id) return;
  try { await refreshInfo(id); } catch { /* No usable binding. */ }
}, { immediate: true });
function scriptPolicies():StructuredScript[]{const parsed=JSON.parse(scriptsText.value);if(!Array.isArray(parsed))throw new Error('脚本授权须为JSON数组');return parsed;}
async function setupDraft(mode:'create'|'associate'|'sync'){
 settingUp.value=true;
 try{const policies=scriptPolicies();if(mode==='associate'&&!sourceFlow.value)throw new Error('请选择要关联的编辑流程');await props.saveAgent?.();await setupLangflowDraft(props.agentId,mode,sourceFlow.value,policies);await refreshInfo(props.agentId);MessagePlugin.success('流程与资源绑定已保存；请编排后发布新版本');}
 catch(error:any){MessagePlugin.error(error?.message || '流程配置失败，请核对已保存资源与画布节点');}
 finally{settingUp.value=false;}
}
function openEditor() {
  if (info.value?.flow_id) window.open(localLangflowEditorUrl(info.value.flow_id), '_blank', 'noopener,noreferrer');
}
async function copyDirectory() {
  connecting.value = true;
  try {
    const response = await createLangflowDesignConnection(props.agentId);
    await copyWithToast(JSON.stringify(response.data), '目录连接已复制，有效期15分钟');
    connectedUntil.value = new Date(response.data.expires_at).toLocaleTimeString();
  } catch { MessagePlugin.error('目录连接失败，请检查编辑权限后重试'); }
  finally { connecting.value = false; }
}
function trial() {
  if (!canTrialLangflow(info.value)) return;
  const settings = useSettingsStore(); settings.selectAgent(props.agentId); settings.toggleAgent(true);
  router.push('/platform/creatChat');
}
async function refreshReleases(id: string) {
  const response = await listLangflowReleases(id);
  if (props.agentId !== id) return;
  releases.value = response.data.releases;
  const current = releases.value.find(r => r.id === response.data.active_release_id);
  if (info.value) { info.value.active_release_id = response.data.active_release_id; info.value.active_release_label = current?.label; }
  selectedRelease.value = response.data.active_release_id;
  if (!releaseLabel.value) releaseLabel.value = `V${releases.value.length + 1}`;
}
async function publish() {
  publishing.value = true;
  const id = props.agentId;
  try { if(info.value?.managed){const policies=scriptPolicies();await props.saveAgent?.();await setupLangflowDraft(id,'sync','',policies);} await publishLangflow(id, releaseLabel.value.trim()); if (props.agentId === id) { releaseLabel.value = ''; await refreshReleases(id); MessagePlugin.success('版本已发布并激活'); } }
  catch (error: any) { MessagePlugin.error(error?.message || '发布失败，请检查流程、资源权限和运行服务'); }
  finally { publishing.value = false; }
}
async function activate() {
  activating.value = true;
  const id = props.agentId;
  try { await activateLangflowRelease(id, selectedRelease.value); if (props.agentId === id) { await refreshReleases(id); MessagePlugin.success('版本已激活，后续任务使用该版本'); } }
  catch (error: any) { MessagePlugin.error(error?.message || '激活失败，请检查版本依赖与资源权限'); }
  finally { activating.value = false; }
}
</script>
<style scoped>
.release-controls { display: flex; flex-wrap: wrap; gap: 10px; margin-top: 12px; }
.release-controls p { width: 100%; margin: 0; }
.release-controls small { color: var(--td-text-color-secondary); }
.release-controls .t-input__wrap, .release-controls .t-select__wrap { max-width: 360px; }
.draft-setup{display:flex;flex-direction:column;gap:10px;margin-bottom:18px}.draft-setup h3,.draft-setup p{margin:0}.draft-setup p{color:var(--td-text-color-secondary)}.draft-actions{display:flex;gap:10px}.draft-setup .t-select__wrap{max-width:460px}
</style>
