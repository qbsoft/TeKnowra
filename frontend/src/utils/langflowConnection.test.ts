import test from 'node:test';
import assert from 'node:assert/strict';
import { localLangflowEditorUrl, canTrialLangflow } from './langflowConnection';

test('published mode never offers draft trial and preserves explicit legacy test mode', () => {
  assert.equal(canTrialLangflow({ bound: true, release_enabled: true }), false);
  assert.equal(canTrialLangflow({ bound: true, release_enabled: true, active_release_id: 'v1' }), true);
  assert.equal(canTrialLangflow({ bound: true, release_enabled: false }), true);
  assert.equal(canTrialLangflow({ bound: false, release_enabled: true, active_release_id: 'v1' }), false);
});

test('editor URL contains only the fixed local origin and flow ID', () => {
  assert.equal(localLangflowEditorUrl('flow-one'), 'http://127.0.0.1:7861/flow/flow-one');
  for (const id of ['../credentials', 'flow?token=secret', 'https://foreign', '']) {
    assert.throws(() => localLangflowEditorUrl(id));
  }
});

test('uses the configured public editor while keeping local development unchanged', () => {
  assert.equal(localLangflowEditorUrl('flow-one', 'https://flow.yihe.work:9222'),
    'https://flow.yihe.work:9222/flow/flow-one');
  assert.equal(localLangflowEditorUrl('flow-one', 'https://flow.yihe.work:9222/'),
    'https://flow.yihe.work:9222/flow/flow-one');
  assert.equal(localLangflowEditorUrl('flow-one', ''), 'http://127.0.0.1:7861/flow/flow-one');
});

test('rejects credentials, executable schemes and non-origin editor addresses', () => {
  for (const base of ['javascript:alert(1)', 'https://user:password@flow.yihe.work',
    'https://flow.yihe.work/flow/other', 'https://flow.yihe.work?token=x',
    'https://flow.yihe.work/#fragment', 'http://foreign.example', '//foreign.example']) {
    assert.throws(() => localLangflowEditorUrl('flow-one', base));
  }
  assert.equal(localLangflowEditorUrl('flow-one','http://localhost:7861'),
    'http://localhost:7861/flow/flow-one');
});

test('the existing button call reads deployment runtime configuration without changing dev defaults', () => {
  const previous = Object.getOwnPropertyDescriptor(globalThis, 'window');
  try {
    Object.defineProperty(globalThis, 'window', { configurable: true,
      value: { __RUNTIME_CONFIG__: { LANGFLOW_EDITOR_URL: 'https://flow.yihe.work:9222' } } });
    assert.equal(localLangflowEditorUrl('flow-one'), 'https://flow.yihe.work:9222/flow/flow-one');
    Object.defineProperty(globalThis, 'window', { configurable: true, value: {} });
    assert.equal(localLangflowEditorUrl('flow-one'), 'http://127.0.0.1:7861/flow/flow-one');
  } finally {
    if (previous) Object.defineProperty(globalThis, 'window', previous);
    else Reflect.deleteProperty(globalThis, 'window');
  }
});
