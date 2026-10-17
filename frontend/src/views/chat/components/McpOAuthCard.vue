<template>
  <div v-if="interactive" class="prompt-panel" @click.stop>
    <div class="prompt-question">{{ $t('agentStream.mcpOAuth.dockQuestion', { target: targetLabel }) }}</div>
    <div class="prompt-desc">{{ $t('agentStream.mcpOAuth.desc') }}</div>

    <div class="prompt-footer">
      <div class="prompt-footer-meta">
        <span v-if="secondsLeft >= 0" class="action-timer" :class="timerClass">{{ formatCountdown(secondsLeft) }}</span>
      </div>
      <div class="prompt-footer-actions">
        <button type="button" class="prompt-btn" :disabled="canceling || authorizing" @click="skip">
          {{ $t('agentStream.mcpOAuth.skip') }}
        </button>
        <button type="button" class="prompt-btn is-primary" :disabled="authorizing || canceling" @click="authorize">
          {{ $t('agentStream.mcpOAuth.authorize') }}<span class="prompt-btn-key" aria-hidden="true">↵</span>
        </button>
      </div>
    </div>
  </div>

  <div v-else class="action-card interaction-inline" :class="{ 'action-pending': !resolved }">
    <div class="action-header no-results">
      <div class="action-title">
        <t-icon class="action-title-icon" :name="headerIcon" />
        <span class="action-name">{{ mainTitle }}</span>
      </div>
    </div>
    <div class="interaction-status-summary">
      <div class="results-summary-text">
        <template v-if="awaitingInDock">
          <span class="status-label">{{ $t('agentStream.promptDock.waiting') }}</span>
          <span class="inline-dot">·</span>
          <button type="button" class="inline-action is-primary" @click.stop="dock?.focus(pendingId)">
            {{ $t('agentStream.promptDock.handle') }}
          </button>
        </template>
        <span v-else class="status-label">{{ statusLine }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { useI18n } from 'vue-i18n'
import { useSettingsStore } from '@/stores/settings'
import {
  cancelMCPOAuth,
  getMCPOAuthAuthorizeURL,
  getMCPOAuthStatus,
  resolveMCPOAuth,
  refreshMCPMetadata,
  MCP_OAUTH_CALLBACK_PATH,
} from '@/api/mcp-service'
import {
  MCP_OAUTH_SIGNAL_KEY,
  claimMcpOAuthResume,
  mcpOAuthDoneUrl,
  releaseMcpOAuthResume,
  watchMcpOAuthPopup,
} from '@/api/mcpOAuthDone'
import {
  cancelEmbedMCPOAuth,
  getEmbedMCPOAuthAuthorizeURL,
  getEmbedMCPOAuthStatus,
  resolveEmbedMCPOAuth,
} from '@/api/embed'
import { markPromptSubmitted, promptSubmission, usePromptDock } from '@/composables/usePromptDock'
import { isSubmitKey } from '@/utils/promptKeys'

const props = defineProps<{
  pendingId: string
  // dock: the authorize prompt, shown above the composer. inline: the timeline row.
  variant?: 'dock' | 'inline'
  serviceId: string
  serviceName: string
  mcpToolName?: string
  timeoutSeconds?: number
  requestedAt?: number
  resolved?: boolean
  authorized?: boolean
  resolveReason?: string
  timedOut?: boolean
  canceled?: boolean
  embeddedMode?: boolean
  embedChannelId?: string
  embedToken?: string
  embedSessionId?: string
  embedSessionSig?: string
  embedVisitorId?: string
}>()

const useEmbedOAuth = () =>
  props.embeddedMode
  && props.embedChannelId
  && props.embedToken
  && props.embedSessionId
  && props.embedSessionSig
  && props.embedVisitorId

const { t } = useI18n()
const dock = usePromptDock()

const submission = computed(() => promptSubmission(props.pendingId))
const open = computed(() => !props.resolved && !submission.value)
// Without a dock (a view that does not provide one) the row stays answerable.
const interactive = computed(() => open.value && (props.variant === 'dock' || !dock))
const awaitingInDock = computed(() => open.value && props.variant !== 'dock' && Boolean(dock?.isDocked(props.pendingId)))

const authorizing = ref(false)
const canceling = ref(false)
const now = ref(Date.now())
let clock: number | null = null
let stopPopupWatch: (() => void) | null = null
let stopResumePoll: (() => void) | null = null

const deadline = computed(() => {
  const base = (props.requestedAt || 0) * 1000
  const add = (props.timeoutSeconds || 600) * 1000
  return base + add
})

const secondsLeft = computed(() => {
  if (props.resolved) return -1
  return Math.max(0, Math.floor((deadline.value - now.value) / 1000))
})

const timerClass = computed(() => {
  if (secondsLeft.value <= 30) return 'timer-critical'
  if (secondsLeft.value <= 120) return 'timer-warning'
  return ''
})

const headerIcon = computed(() => {
  if (!props.resolved) return 'lock-on'
  if (props.authorized) return 'check-circle'
  return 'close-circle'
})

const targetLabel = computed(() => (
  props.mcpToolName
    ? t('agentStream.mcpOAuth.targetWithTool', { service: props.serviceName, tool: props.mcpToolName })
    : props.serviceName
))

const mainTitle = computed(() => {
  if (!props.resolved) {
    return t('agentStream.mcpOAuth.waiting', { target: targetLabel.value })
  }
  return props.mcpToolName
    ? t('agentStream.mcpOAuth.titleWithTool', { service: props.serviceName, tool: props.mcpToolName })
    : t('agentStream.mcpOAuth.titleWithService', { service: props.serviceName })
})

const statusLine = computed(() => {
  if (!props.resolved) {
    if (submission.value?.authorized) return t('agentStream.mcpOAuth.authorizedTag')
    if (submission.value) return t('agentStream.mcpOAuth.canceledTag')
    return t('agentStream.mcpOAuth.waitingStatus')
  }
  if (props.authorized) return t('agentStream.mcpOAuth.authorizedTag')
  if (props.timedOut) return t('agentStream.mcpOAuth.timedOutTag')
  return t('agentStream.mcpOAuth.canceledTag')
})

function formatCountdown(s: number): string {
  if (s < 60) return t('agentStream.mcpOAuth.countdownShort', { seconds: s })
  const m = Math.floor(s / 60)
  const r = s % 60
  return `${m}:${r.toString().padStart(2, '0')}`
}

function stopPoll() {
  if (stopPopupWatch) {
    stopPopupWatch()
    stopPopupWatch = null
  }
  if (stopResumePoll) {
    stopResumePoll()
    stopResumePoll = null
  }
}

// 令牌记上之后就把暂停的对话放开。不看某一次弹窗还在不在：
// 弹窗跨站时经常像已经关掉，卡片也可能被重新挂上，原来的那次询问就丢了。
async function resumeIfTokenReady() {
  if (props.resolved || canceling.value || useEmbedOAuth()) return
  let ok = false
  try {
    ok = await getMCPOAuthStatus(props.serviceId, undefined, sharedAgentScope.value)
  } catch {
    return
  }
  if (!ok || props.resolved || canceling.value) return
  if (!claimMcpOAuthResume(props.pendingId)) return
  try {
    await resolveMCPOAuth(
      props.pendingId,
      { service_id: props.serviceId, decision: 'authorize' },
      sharedAgentScope.value,
    )
    try {
      await refreshMCPMetadata(props.serviceId)
    } catch {
      /* next describe/list will live-fill this user's directory */
    }
    markPromptSubmitted(props.pendingId, { authorized: true })
    MessagePlugin.success(t('agentStream.mcpOAuth.authorizedToast'))
    authorizing.value = false
  } catch (e: any) {
    const msg = e?.response?.data?.error?.message || e?.message || t('agentStream.mcpOAuth.resumeFailed')
    if (!/already completed|already resolved/i.test(String(msg))) {
      MessagePlugin.error(msg)
    }
    releaseMcpOAuthResume(props.pendingId)
  }
}

const skip = async () => {
  if (props.resolved || canceling.value) return
  canceling.value = true
  try {
    if (useEmbedOAuth()) {
      await cancelEmbedMCPOAuth(
        props.embedChannelId!,
        props.embedToken!,
        props.embedSessionId!,
        props.embedSessionSig!,
        props.embedVisitorId!,
        props.pendingId,
      )
    } else {
      await cancelMCPOAuth(props.pendingId, sharedAgentScope.value)
    }
    markPromptSubmitted(props.pendingId, { authorized: false })
  } catch (e: any) {
    const msg = e?.response?.data?.error?.message || e?.message || t('agentStream.mcpOAuth.skipFailed')
    MessagePlugin.error(msg)
  } finally {
    stopPoll()
    authorizing.value = false
    canceling.value = false
  }
}

// TeKnowra: 当前对话用的是别的空间共享来的智能体时，授权要落到那个源空间（MCP 服务在那边），
// 后端据此换空间并校验共享关系。用自己空间的智能体时为 undefined，行为与原来一致。
const settingsStore = useSettingsStore()
const sharedAgentScope = computed(() => {
  const sourceTenantId = settingsStore.selectedAgentSourceTenantId
  const agentId = settingsStore.settings.selectedAgentId
  return sourceTenantId && agentId ? { agentId, sourceTenantId: String(sourceTenantId) } : undefined
})

const authorize = async () => {
  if (props.resolved || authorizing.value) return
  authorizing.value = true
  try {
    const redirectUri = window.location.origin + MCP_OAUTH_CALLBACK_PATH
    // 嵌入页仍回到嵌入地址（那边有自己的落脚提示）。平台对话不能把弹窗送回对话本身，
    // 否则会再开一份对话，原来正在回答的那一页停止往下走。
    const frontendRedirect = useEmbedOAuth()
      ? window.location.pathname + window.location.search
      : mcpOAuthDoneUrl()
    const authorization = useEmbedOAuth()
      ? await getEmbedMCPOAuthAuthorizeURL(
        props.embedChannelId!,
        props.embedToken!,
        props.embedSessionId!,
        props.embedSessionSig!,
        props.embedVisitorId!,
        props.serviceId,
        { redirect_uri: redirectUri, frontend_redirect: frontendRedirect },
      )
      : await getMCPOAuthAuthorizeURL(props.serviceId, {
        redirect_uri: redirectUri,
        frontend_redirect: frontendRedirect,
      }, sharedAgentScope.value)
    if (!authorization.authorizationUrl || !authorization.authorizationAttempt) {
      MessagePlugin.error(t('agentStream.mcpOAuth.startFailed'))
      authorizing.value = false
      return
    }
    const popup = window.open(authorization.authorizationUrl, 'mcp_oauth', 'width=600,height=720')
    if (!popup) {
      MessagePlugin.error(t('agentStream.mcpOAuth.startFailed'))
      authorizing.value = false
      return
    }
    if (stopPopupWatch) {
      stopPopupWatch()
      stopPopupWatch = null
    }
    stopPopupWatch = watchMcpOAuthPopup(
      popup,
      () => useEmbedOAuth()
        ? getEmbedMCPOAuthStatus(
          props.embedChannelId!,
          props.embedToken!,
          props.embedSessionId!,
          props.embedSessionSig!,
          props.embedVisitorId!,
          props.serviceId,
          authorization.authorizationAttempt,
        )
        : getMCPOAuthStatus(props.serviceId, authorization.authorizationAttempt, sharedAgentScope.value),
      async () => {
        if (useEmbedOAuth()) {
          try {
            await resolveEmbedMCPOAuth(
              props.embedChannelId!,
              props.embedToken!,
              props.embedSessionId!,
              props.embedSessionSig!,
              props.embedVisitorId!,
              props.pendingId,
              { service_id: props.serviceId, decision: 'authorize' },
            )
            markPromptSubmitted(props.pendingId, { authorized: true })
            MessagePlugin.success(t('agentStream.mcpOAuth.authorizedToast'))
          } catch (e: any) {
            const msg = e?.response?.data?.error?.message || e?.message || t('agentStream.mcpOAuth.resumeFailed')
            MessagePlugin.error(msg)
          }
          authorizing.value = false
          return
        }
        await resumeIfTokenReady()
      },
      () => {
        authorizing.value = false
      },
    )
  } catch (e: any) {
    const msg = e?.response?.data?.error?.message || e?.message || t('agentStream.mcpOAuth.startFailed')
    MessagePlugin.error(msg)
    authorizing.value = false
  }
}

onMounted(() => {
  clock = window.setInterval(() => {
    now.value = Date.now()
  }, 1000)
  if (props.resolved || useEmbedOAuth()) return
  const onStorage = (event: StorageEvent) => {
    if (event.key === MCP_OAUTH_SIGNAL_KEY) void resumeIfTokenReady()
  }
  window.addEventListener('storage', onStorage)
  const timer = window.setInterval(() => { void resumeIfTokenReady() }, 1500)
  stopResumePoll = () => {
    window.clearInterval(timer)
    window.removeEventListener('storage', onStorage)
  }
  void resumeIfTokenReady()
})

onBeforeUnmount(() => {
  if (clock) window.clearInterval(clock)
  stopPoll()
})

/** Enter starts authorization while docked. */
const handleKey = (e: KeyboardEvent): boolean => {
  if (!interactive.value || !isSubmitKey(e) || authorizing.value || canceling.value) return false
  void authorize()
  return true
}

defineExpose({ handleKey })
</script>

<style scoped lang="less">
@import './agent-interaction-card.less';
</style>
