import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import { fileURLToPath } from 'node:url'

// 浮窗启动按钮的图标：上游用 emoji（💬 / ✕），长相由操作系统字体决定，Windows 上很难看。
// fork 在 public/weknora-widget.js 里换成内联矢量图标，钩子 3 处。合并上游时若被冲掉，
// 按钮会悄悄变回 emoji——不报错，只是又难看了。这条测试盯着它。
const widget = readFileSync(fileURLToPath(new URL('../../../public/weknora-widget.js', import.meta.url)), 'utf8')

test('启动按钮不再用 emoji 当图标', () => {
  assert.doesNotMatch(widget, /launcher\.textContent\s*=/)
  assert.ok(!widget.includes("'💬'"), 'emoji 气泡又回来了')
})

test('三处钩子都在：初始图标、样式、开合切换', () => {
  assert.match(widget, /tkLauncherIcon\(launcher, false\)/)
  assert.match(widget, /tkLauncherStyle\(launcher\)/)
  assert.match(widget, /tkLauncherIcon\(launcher, panelOpen\)/)
})

test('图标是写死的静态 SVG，不拼接任何外部输入', () => {
  const fn = widget.slice(widget.indexOf('function tkLauncherIcon'), widget.indexOf('function tkLauncherStyle'))
  assert.match(fn, /launcher\.innerHTML = open \? TK_ICON_CLOSE : TK_ICON_CHAT/)
  for (const name of ['TK_ICON_CHAT', 'TK_ICON_CLOSE']) {
    const decl = widget.slice(widget.indexOf(`var ${name} =`), widget.indexOf(';', widget.indexOf(`var ${name} =`)))
    assert.doesNotMatch(decl, /opts\.|title|\bchannelId\b/, `${name} 里不该出现宿主传入的值`)
  }
})
