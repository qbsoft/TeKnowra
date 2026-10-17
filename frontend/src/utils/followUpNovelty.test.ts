import assert from 'node:assert/strict'
import test from 'node:test'
import { excludeAskedQuestions } from './followUpNovelty.ts'
test('cached cards omit current and previous user questions, retain drill-down IDs', () => {
  const items = [{id:'old',text:' 九月变化多少? '},{id:'next',text:'这个客户有哪些订单？'}]
  assert.deepEqual(excludeAskedQuestions(items,['九月变化多少？']),[items[1]])
})
