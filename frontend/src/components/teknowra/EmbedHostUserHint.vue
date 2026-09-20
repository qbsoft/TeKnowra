<!--
  「设置 → 网页嵌入 → 复制嵌入代码」下面那段说明 —— TeKnowra fork 自己的组件，上游没有。

  平台生成的嵌入代码里多了一个 hostUser（data-host-user / host_user）。它是可选的，而且占位符必须由
  接入方换成自己系统里的当前用户 ID——不说清楚的话，对方要么原样贴上去（等于没传），要么不知道
  这一行是干什么的。单页应用的宿主换人时页面不刷新，写死在标签上的属性跟不上，要用编程方式。
  钩子：AgentEmbedChannelPanel.vue 里代码框下面一行。
-->
<template>
  <div class="host-user-hint" role="note">
    <p class="host-user-hint__title">{{ text.title }}</p>
    <p>{{ text.what }} <code>{{ field }}</code> {{ text.how }}</p>
    <p>{{ text.none }}</p>
    <template v-if="mode !== 'iframe'">
      <p>{{ text.spa }}</p>
      <pre class="host-user-hint__pre">{{ spaExample }}</pre>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { EMBED_HOST_USER_PLACEHOLDER } from '@/api/embed/hostUser'

const props = defineProps<{ mode: 'iframe' | 'widget' | 'secure' }>()
const { locale } = useI18n()

const field = computed(() =>
  props.mode === 'iframe' ? `host_user=${EMBED_HOST_USER_PLACEHOLDER}` : `data-host-user="${EMBED_HOST_USER_PLACEHOLDER}"`,
)

const spaExample = computed(() =>
  props.mode === 'secure'
    ? `WeKnora.init({ channel: '…', tokenEndpoint: '…', hostUser: currentUser.id })`
    : `WeKnora.init({ channel: '…', token: '…', hostUser: currentUser.id })`,
)

const zh = {
  title: '你的系统有登录吗？',
  what: '有的话，把代码里的',
  how: `换成当前登录用户的 ID（用不会变的 ID，不要用姓名、邮箱）。这样同一个浏览器里换了人登录，新的人需要自己授权、看不到上一个人的对话；换回原来的人，沿用他之前的授权和对话。`,
  none: `没有登录（如官网、帮助中心）：删掉这一项即可，一个浏览器算一个访客。占位符 ${EMBED_HOST_USER_PLACEHOLDER} 原样留着也等同于没填。`,
  spa: '单页应用换人登录时页面不会刷新，写在标签上的值跟不上——改用编程方式，用户变化后重新调用一次：',
}
const en = {
  title: 'Does your site have its own login?',
  what: 'If so, replace',
  how: `with the ID of the signed-in user (a stable ID, not a name or email). When a different person signs in on the same browser they authorize for themselves and cannot see the previous person's conversation; switching back restores the original person's authorization and conversation.`,
  none: `No login (marketing site, help center): remove it — one browser is one visitor. Leaving the ${EMBED_HOST_USER_PLACEHOLDER} placeholder as-is is treated as not set.`,
  spa: 'Single-page apps do not reload when the user changes, so a value written on the tag goes stale. Initialize programmatically and call it again after the user changes:',
}
const text = computed(() => (String(locale.value || '').toLowerCase().startsWith('zh') ? zh : en))
</script>

<style scoped>
.host-user-hint {
  margin-top: 12px;
  padding: 12px 14px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 8px;
  background: var(--td-bg-color-container-hover);
  font-size: 13px;
  line-height: 1.7;
  color: var(--td-text-color-secondary);
}
.host-user-hint p {
  margin: 0 0 6px;
}
.host-user-hint__title {
  font-weight: 600;
  color: var(--td-text-color-primary);
}
.host-user-hint code {
  padding: 1px 5px;
  border-radius: 4px;
  background: var(--td-bg-color-component);
  font-size: 12px;
  word-break: break-all;
}
.host-user-hint__pre {
  margin: 4px 0 0;
  padding: 8px 10px;
  border-radius: 6px;
  background: var(--td-bg-color-component);
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
