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
