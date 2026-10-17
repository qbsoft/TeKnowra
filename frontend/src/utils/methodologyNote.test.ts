import assert from 'node:assert/strict'
import test from 'node:test'
import { renderChatMarkdown, createChatMarkdownRenderer } from './chatMarkdownRenderer.ts'

const render = (text: string) => renderChatMarkdown(text, {
  renderer: createChatMarkdownRenderer(), escapeMarkdown: x => x, sanitizeHtml: x => x,
})
test('methodology note collapses without hiding the core answer or the next heading', () => {
  const html = render('销售额100元。\n\n### 口径说明\n\n- 按认收日统计\n- 当前用户范围\n\n### 证据限制\n\n尚无交付证据。')
  assert.match(html, /<details class="methodology-note"><summary>口径说明/)
  assert.doesNotMatch(html, /<details[^>]*\bopen/)
  assert.match(html, /销售额100元/)
  assert.match(html, /按认收日统计/)
  assert.match(html, /<\/details>[\s\S]*证据限制/)
})
test('inline legacy note keeps its full text', () => {
  const html = render('结果。\n\n**口径说明：** 2026年9月，全部数据。')
  assert.match(html, /<summary>口径说明/)
  assert.match(html, /2026年9月，全部数据/)
})
test('examples and ordinary mentions of methodology remain normal content', () => {
  assert.doesNotMatch(render('```text\n口径说明：示例\n```\n\n请阅读口径说明。'), /<details/)
})
test('methodology note preserves reference links and math', () => {
  const html = render('结果。\n\n口径说明：见[业务证据][source]，$x+y$。\n\n[source]: https://example.com/evidence')
  assert.match(html, /href="https:\/\/example.com\/evidence"/)
  assert.match(html, /katex/)
})
