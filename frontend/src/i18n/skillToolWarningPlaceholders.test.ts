import assert from 'node:assert/strict'
import test from 'node:test'

import { LOCALE_BUNDLES, getLocaleValueAtPath, type LocaleName } from './localeKeyAudit.ts'

/**
 * 这几条提示是带参数的，模板里传的名字和译文里写的占位符必须对上。对不上
 * 的话不会报错，只会把 {tools} 原样显示出来——而这类提示本来就少见，很可能
 * 到用户看见了才发现。这里把两边钉死。
 *
 * 参数名以模板里实际传的为准：
 *   agentEditor.tools.mcpLoadFailed        { message }
 *   agent.editor.skillMissingTools         { tools }
 *   agent.editor.skillMissingToolsNamed    { skill, tools }
 */
const EXPECTED_PLACEHOLDERS: Record<string, string[]> = {
  'agentEditor.tools.mcpLoadFailed': ['message'],
  'agent.editor.skillMissingTools': ['tools'],
  'agent.editor.skillMissingToolsNamed': ['skill', 'tools'],
}

// 不带参数但同属这批提示的，反过来钉：不该出现任何占位符，
// 否则模板不传值，用户会看到一个花括号。
const EXPECTED_NO_PLACEHOLDERS = [
  'agentEditor.tools.mcpLoading',
  'agentEditor.tools.mcpNameDegraded',
]

function placeholdersIn(text: string): string[] {
  return [...text.matchAll(/\{\s*([A-Za-z0-9_]+)\s*\}/g)].map(m => m[1]).sort()
}

test('每种语言的技能/MCP 提示都用同一组占位符', () => {
  const problems: string[] = []

  for (const [localeName, bundle] of Object.entries(LOCALE_BUNDLES) as Array<[LocaleName, unknown]>) {
    for (const [path, expected] of Object.entries(EXPECTED_PLACEHOLDERS)) {
      const value = getLocaleValueAtPath(bundle, path)
      if (typeof value !== 'string') {
        problems.push(`${localeName} 缺少 ${path}`)
        continue
      }
      const got = placeholdersIn(value)
      if (got.join(',') !== [...expected].sort().join(',')) {
        problems.push(`${localeName} ${path} 的占位符是 [${got}]，模板传的是 [${expected}]`)
      }
    }

    for (const path of EXPECTED_NO_PLACEHOLDERS) {
      const value = getLocaleValueAtPath(bundle, path)
      if (typeof value !== 'string') {
        problems.push(`${localeName} 缺少 ${path}`)
        continue
      }
      const got = placeholdersIn(value)
      if (got.length) {
        problems.push(`${localeName} ${path} 有占位符 [${got}]，但模板不传参数`)
      }
    }
  }

  assert.deepEqual(problems, [])
})
