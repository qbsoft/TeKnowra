import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { spawnSync } from 'node:child_process';

const entrypoint = readFileSync(new URL('../../docker-entrypoint.sh', import.meta.url), 'utf8');
const validation = entrypoint.split('# 生成运行时配置文件')[0];
const shell = process.env.TEST_POSIX_SHELL || 'sh';

test('container configuration accepts a public origin and an unset local default', () => {
  for (const value of ['', 'https://flow.yihe.work:9222', 'http://127.0.0.1:7861']) {
    const result = spawnSync(shell, ['-c', validation], {
      env: { ...process.env, LANGFLOW_EDITOR_URL: value }, encoding: 'utf8',
    });
    assert.equal(result.status, 0, result.error?.message || result.stderr);
  }
});

test('container configuration rejects script injection, credentials and multiline values', () => {
  for (const value of ['https://user:password@flow.yihe.work', 'javascript:alert(1)',
    'https://flow.yihe.work/path', 'https://flow.yihe.work";alert(1);//',
    'https://flow.yihe.work\n";alert(1);//', 'https://flow.yihe.work\r\n']) {
    const result = spawnSync(shell, ['-c', validation], {
      env: { ...process.env, LANGFLOW_EDITOR_URL: value }, encoding: 'utf8',
    });
    assert.equal(result.status, 1, `unexpected acceptance: ${JSON.stringify(value)}`);
    assert.match(result.stderr, /Invalid LANGFLOW_EDITOR_URL/);
  }
});
