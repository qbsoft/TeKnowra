import type { DeploymentCapabilityKey } from './deploymentCapabilities'

export const CHROME_EXTENSION_URL =
  'https://chromewebstore.google.com/detail/jpemjbopikggjlmikmclgbmkhhopjdgd?utm_source=item-share-cb'

export const CLAWHUB_SKILL_URL = 'https://clawhub.ai/lyingbug/weknora'

export type IntegrationTab = 'im' | 'embed' | 'api' | 'cli' | 'chrome' | 'claw'

export const INTEGRATION_TABS: IntegrationTab[] = ['im', 'embed', 'api', 'cli', 'chrome', 'claw']

/** Aligns with Settings.vue SECTION_MIN_ROLE.api and router.go g.Owner() on /api-principal-config. */
export type IntegrationTabRole = 'viewer' | 'contributor' | 'admin' | 'owner'

export const INTEGRATION_TAB_MIN_ROLE: Partial<Record<IntegrationTab, IntegrationTabRole>> = {
  api: 'owner',
}

export const INTEGRATION_TAB_CAPABILITY: Partial<Record<IntegrationTab, DeploymentCapabilityKey>> = {
  im: 'integrations.im',
  embed: 'integrations.embed',
  api: 'integrations.api',
}

/**
 * TeKnowra：从导航里隐藏的集成页。CLI 与 Claw Skill 介绍的是上游发布的外部产物
 * （命令就叫 `weknora`、安装步骤是克隆上游仓库、Skill 发布在上游作者名下），
 * 本产品没有对应物，只改标题会前后对不上，所以整页不进导航。
 * INTEGRATION_TABS 不动：路由仍可解析，页面仍可用 ?section= 直接打开。见 BRANDING.md。
 */
const HIDDEN_INTEGRATION_TABS: ReadonlySet<IntegrationTab> = new Set(['cli', 'claw'])

export type IntegrationPreviewIcon =
  | { type: 'icon'; name: string }
  | { type: 'emoji'; value: string }

/** Sidebar hover preview + Integrations modal nav — add new entries here. */
export const INTEGRATION_PREVIEW_ITEMS: Array<{
  key: IntegrationTab
  icon: IntegrationPreviewIcon
}> = [
  { key: 'im', icon: { type: 'icon', name: 'chat-message' } },
  { key: 'embed', icon: { type: 'icon', name: 'code' } },
  { key: 'api', icon: { type: 'icon', name: 'secured' } },
  { key: 'cli', icon: { type: 'icon', name: 'code' } },
  { key: 'chrome', icon: { type: 'icon', name: 'extension' } },
  { key: 'claw', icon: { type: 'emoji', value: '🦞' } },
].filter((item) => !HIDDEN_INTEGRATION_TABS.has(item.key))
