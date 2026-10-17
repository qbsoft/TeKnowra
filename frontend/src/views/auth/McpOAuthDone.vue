<template>
  <main class="mcp-oauth-done">
    <p>{{ message }}</p>
  </main>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { mcpOAuthLanding, signalMcpOAuthLanding } from '@/api/mcpOAuthDone'

const message = ref('正在确认授权…')

onMounted(() => {
  const landing = mcpOAuthLanding(window.location.hash)
  if (landing) signalMcpOAuthLanding(landing)
  message.value = landing === 'error'
    ? '授权没有完成，请关闭此窗口后重试'
    : '授权成功，可以关闭此窗口'
  if (landing !== 'error') {
    window.setTimeout(() => {
      try { window.close() } catch { /* 不是脚本打开的窗口就留着这句提示 */ }
    }, 400)
  }
})
</script>

<style scoped>
.mcp-oauth-done {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  font: var(--app-text-xl)/1.5 system-ui, sans-serif;
  color: #1f2329;
}
</style>
