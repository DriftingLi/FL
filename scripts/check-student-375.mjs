#!/usr/bin/env node
/**
 * 375 宽溢出检查（ADR-0085 / 票 #1966）。
 *
 * 判据：`document.documentElement.scrollWidth <= window.innerWidth + 1`。
 * 为什么不是静态扫描：溢出由真实布局决定（flex 收缩、固定宽、字体度量），
 * 静态模式只能猜；本脚本借 Chrome DevTools Protocol 在真实渲染下量。
 *
 * 前置（脚本不负责起服务）：
 *   1. 前端 dev server 在跑（`npm run dev`）；
 *   2. 一个开了远程调试的 Chromium：`msedge --headless=new --remote-debugging-port=9222 about:blank`；
 *   3. 一个已登录账号的 token 与 userInfo（管理端还需 role.id 绑了能力集）。
 *
 * 用法：
 *   node scripts/check-student-375.mjs \
 *     --base http://training.localhost:5173 \
 *     --token <JWT> --user-info '<JSON>' \
 *     [--routes /training,/training/courses] [--width 375] [--cdp http://127.0.0.1:9222] [--json]
 *
 * 退出码：0 = 全部无溢出；1 = 至少一条路由溢出；2 = 用法 / 环境错误。
 *
 * 为什么没进 CI：需要真实浏览器与已登录会话，与本仓纯静态守卫的成本模型不同。
 * 接 CI 的前置条件与做法见 #1966 的评论。
 */
import { setTimeout as sleep } from 'node:timers/promises'

const DEFAULT_ROUTES = ['/training', '/training/courses', '/training/favorites', '/training/search', '/training/notebook']

function parseArgs(argv) {
  const out = { base: 'http://training.localhost:5173', cdp: 'http://127.0.0.1:9222', width: 375, routes: DEFAULT_ROUTES, json: false, token: '', userInfo: '' }
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i]
    if (a === '--base') out.base = argv[++i]
    else if (a === '--cdp') out.cdp = argv[++i]
    else if (a === '--token') out.token = argv[++i]
    else if (a === '--user-info') out.userInfo = argv[++i]
    else if (a === '--width') out.width = Number(argv[++i])
    else if (a === '--routes') out.routes = argv[++i].split(',').map(s => s.trim()).filter(Boolean)
    else if (a === '--json') out.json = true
    else if (a === '--help' || a === '-h') { console.log(usage()); process.exit(0) }
  }
  return out
}

function usage() {
  return [
    '用法: node scripts/check-student-375.mjs --token <JWT> --user-info <JSON> [--base URL] [--routes a,b] [--width 375] [--cdp URL] [--json]',
    '',
    '前置：dev server 在跑；Chromium 开了 --remote-debugging-port=9222；token 与 userInfo 来自已登录会话。'
  ].join('\n')
}

const args = parseArgs(process.argv.slice(2))
if (!args.token || !args.userInfo) {
  console.error(usage())
  process.exit(2)
}

let targets
try {
  targets = await (await fetch(args.cdp + '/json')).json()
} catch (e) {
  console.error('[check-375] 连不上 CDP: ' + args.cdp + '（Chromium 是否已带 --remote-debugging-port 启动？）')
  process.exit(2)
}
const page = targets.find(t => t.type === 'page')
if (!page) { console.error('[check-375] CDP 里没有 page target'); process.exit(2) }

const ws = new WebSocket(page.webSocketDebuggerUrl)
let seq = 0
const pending = new Map()
ws.onmessage = (e) => { const m = JSON.parse(e.data); if (m.id && pending.has(m.id)) { pending.get(m.id)(m); pending.delete(m.id) } }
await new Promise((res, rej) => { ws.onopen = res; ws.onerror = rej })
const send = (method, params = {}) => new Promise((res, rej) => {
  const id = ++seq
  pending.set(id, m => m.error ? rej(new Error(method + ' ' + JSON.stringify(m.error))) : res(m.result))
  ws.send(JSON.stringify({ id, method, params }))
})
await send('Page.enable')
await send('Runtime.enable')
const evalJs = async (expr) => (await send('Runtime.evaluate', { expression: expr, returnByValue: true, awaitPromise: true })).result?.value
const goto = async (url, wait = 1000) => {
  await send('Page.navigate', { url })
  for (let i = 0; i < 80; i++) { await sleep(150); if (await evalJs('document.readyState').catch(() => null) === 'complete') break }
  await sleep(wait)
}

await goto(args.base + '/login', 900)
await evalJs("localStorage.setItem('token'," + JSON.stringify(args.token) + ");localStorage.setItem('userInfo'," + JSON.stringify(args.userInfo) + ")")
await send('Emulation.setDeviceMetricsOverride', { width: args.width, height: 800, deviceScaleFactor: 1, mobile: false })

const results = []
for (const route of args.routes) {
  await goto(args.base + route, 1100)
  const m = await evalJs(`(() => {
    const de = document.documentElement, iw = window.innerWidth
    let worst = null
    for (const el of document.querySelectorAll('body *')) {
      const r = el.getBoundingClientRect()
      if (r.right > iw + 2 && r.width > 0 && (!worst || r.right > worst.right)) {
        worst = { right: Math.round(r.right), cls: (el.className && el.className.toString().slice(0, 50)) || el.tagName }
      }
    }
    return { path: location.pathname, iw, sw: de.scrollWidth, overflow: de.scrollWidth > iw + 1, worst }
  })()`)
  results.push({ route, ...m })
  if (!args.json) {
    console.log((m.overflow ? '✗ ' : '✓ ') + route + ' → ' + m.path + ' sw=' + m.sw + '/' + m.iw + (m.overflow ? ' 越界元素: ' + (m.worst?.cls ?? '?') : ''))
  }
}
ws.close()

if (args.json) console.log(JSON.stringify(results, null, 1))
const bad = results.filter(r => r.overflow || r.path !== r.route)
if (bad.length) {
  console.error('[check-375] ' + bad.length + ' 条路由不达标（溢出或未落到目标路由——后者通常意味着被守卫重定向，账号或证件状态不对）')
  process.exit(1)
}
console.log('[check-375] 全部 ' + results.length + ' 条路由在 ' + args.width + 'px 下无页面级横向溢出')
