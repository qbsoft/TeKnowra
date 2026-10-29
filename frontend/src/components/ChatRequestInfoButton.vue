<template>
  <t-popup
    v-model:visible="visible"
    trigger="click"
    placement="top"
    :show-arrow="true"
    destroy-on-close
    overlay-class-name="chat-request-info-popup"
    :overlay-inner-style="{ padding: 0 }"
  >
    <t-button
      size="small"
      variant="outline"
      shape="round"
      :title="$t('chat.requestInfoTitle')"
    >
      <ChatActionIcon name="info-circle" />
    </t-button>
    <template #content>
      <div class="chat-request-card" @click.stop>
        <div class="chat-request-card-header">
          <span class="chat-request-card-title">{{ $t('chat.requestInfoTitle') }}</span>
          <t-button
            v-if="rows.length > 0"
            size="small"
            variant="text"
            shape="square"
            :title="$t('common.copy')"
            @click="copyAll"
          >
            <t-icon name="copy" />
          </t-button>
        </div>
        <div v-if="rows.length === 0" class="chat-request-empty">
          {{ $t('chat.requestInfoEmpty') }}
        </div>
        <div v-else class="chat-request-card-body">
          <div v-for="row in rows" :key="row.key" class="chat-request-row">
            <span class="chat-request-label">{{ row.label }}</span>
            <span class="chat-request-value">{{ row.value }}</span>
          </div>
        </div>
      </div>
    </template>
  </t-popup>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { get } from '@/utils/request';
import { useAuthStore } from '@/stores/auth';
import ChatActionIcon from './ChatActionIcon.vue';

import { useI18n } from 'vue-i18n';
import {
  buildChatRequestDebugPayload,
  type ChatRequestDebugInfo,
} from '@/utils/chatRequestDebug';
import { copyWithToast } from '@/utils/clipboard';

const props = defineProps<{
  session: Record<string, unknown>;
  sessionId?: string;
}>();

const { t } = useI18n();
const visible = ref(false);
const authStore=useAuthStore();
const executionInfo = ref<{diagnostics?: Array<{code:string; node:string; reasons?:string[]}>; versions?:Array<{definition:string; implementation:string}>; historical_implementation_unknown?:boolean} | null>(null);
watch([visible,()=>props.session.id,()=>props.sessionId], async ([open]) => {
  executionInfo.value=null;
  if (!open || !props.sessionId || !props.session.id || !authStore.hasRole('contributor')) return;
  const identity=`${props.sessionId}/${props.session.id}`;
  try {
    const result=await get<{data:NonNullable<typeof executionInfo.value>}>(`/api/v1/messages/${encodeURIComponent(props.sessionId)}/${encodeURIComponent(String(props.session.id))}/execution-diagnostics`,{silent:true});
    if (visible.value && identity===`${props.sessionId}/${props.session.id}`) executionInfo.value=result.data;
  } catch { /* Unsupported role and historic servers disclose no private details. */ }
});

const debugInfo = computed((): ChatRequestDebugInfo => {
  const s = props.session;
  const dr = s.debugRequest as ChatRequestDebugInfo | undefined;
  return {
    requestId: (s.request_id as string) || dr?.requestId,
    messageId: (s.id as string) || undefined,
    sessionId: props.sessionId || dr?.sessionId,
    url: dr?.url,
    method: dr?.method,
    body: dr?.body ?? null,
    sentAt: dr?.sentAt,
  };
});

const rows = computed(() => {
  const info = debugInfo.value;
  const list: { key: string; label: string; value: string }[] = [];
  const add = (key: string, labelKey: string, val?: string) => {
    if (!val) return;
    list.push({ key, label: t(labelKey), value: val });
  };
  add('requestId', 'chat.requestInfoRequestId', info.requestId);
  add('messageId', 'chat.requestInfoMessageId', info.messageId);
  add('sessionId', 'chat.requestInfoSessionId', info.sessionId);
  if (info.method && info.url) {
    list.push({ key: 'url', label: t('chat.requestInfoUrl'), value: `${info.method} ${info.url}` });
  }
  if (info.sentAt) {
    add('sentAt', 'chat.requestInfoSentAt', new Date(info.sentAt).toLocaleString());
  }
  const versions=executionInfo.value?.versions || [];
  if (versions.length) {
    list.push({key:'definition',label:'场景 / 发布版本',value:versions[0].definition || '未知'});
    list.push({key:'implementation',label:'实现版本',value:versions[0].implementation || '历史任务未记录，不补写'});
  }
  for (const [index,d] of (executionInfo.value?.diagnostics || []).entries()) {
    list.push({key:`diagnostic-${index}`,label:'执行诊断',value:[d.code,d.node,...(d.reasons || [])].join(' · ')});
  }
  return list;
});

const copyAll = async () => {
  const ok = await copyWithToast(buildChatRequestDebugPayload(debugInfo.value), 'common.copied');
  if (ok) visible.value = false;
};
</script>

<style lang="less">
.chat-request-info-popup {
  .t-popup__content {
    padding: 0;
    border-radius: var(--app-radius-md);
    box-shadow: var(--td-shadow-2);
  }
}
</style>

<style scoped lang="less">
.chat-request-card {
  min-width: 260px;
  max-width: 360px;
  padding: 10px 12px;
  font-size: var(--app-text-sm);
  color: var(--td-text-color-primary);
}

.chat-request-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
  padding-bottom: 6px;
  border-bottom: 1px solid var(--td-component-stroke);
}

.chat-request-card-title {
  font-size: var(--app-text-sm);
  font-weight: 600;
}

.chat-request-card-body {
  display: flex;
  flex-direction: column;
  gap: 2px;
  max-height: 360px;
  overflow-y: auto;
}

.chat-request-row {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 3px 0;
  line-height: 1.5;
}

.chat-request-label {
  flex: 0 0 72px;
  color: var(--td-text-color-secondary);
}

.chat-request-value {
  flex: 1;
  font-family: var(--td-font-family-mono);
  font-size: var(--app-text-xs);
  color: var(--td-text-color-primary);
  word-break: break-all;
}

.chat-request-empty {
  color: var(--td-text-color-placeholder);
  padding: 4px 0 8px;
}

</style>
