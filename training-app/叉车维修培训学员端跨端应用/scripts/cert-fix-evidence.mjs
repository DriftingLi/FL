// #1671 修复后取证（最终版 v3）：devtools automator
// 链路：connect 9420 → mock request → switchTab dashboard → setData({e:false}) 关浮层 → 基线帧
//       → setData({e:true}) 开浮层 → 浮层帧。e = 编译产物 dashboard.wxml 中 cert-selector 的 wx:if 绑定键。
// 说明：automator 无法穿透自定义组件边界触发 onCertTap（setup 闭包 + virtualHost），
//       改用渲染层 setData 直写 v-if 键，与真实链路共享同一 v-if 与组件树。
import { createRequire } from 'node:module';
import fs from 'node:fs';
const req = createRequire(process.env.TEMP + '\\mp-weixin-automator\\noop.js');
const automator = req('miniprogram-automator');
const OUT = '.ci-verify/1671';
fs.mkdirSync(OUT, { recursive: true });
const save = (name, img) => {
  const buf = typeof img === 'string' ? Buffer.from(img.replace(/^data:image\/\w+;base64,/, ''), 'base64') : Buffer.from(img);
  fs.writeFileSync(OUT + '/' + name, buf);
  return buf.length;
};
const mp = await automator.connect({ wsEndpoint: 'ws://127.0.0.1:9420' });
const consoleMsgs = [];
const exceptions = [];
mp.on('console', (m) => consoleMsgs.push(`${m.type}:${String(m.args && m.args.map ? m.args.map((a) => String(a)).join(' ') : a2s(m.args)).slice(0, 160)}`));
mp.on('exception', (e) => exceptions.push(String(e && e.message).slice(0, 200)));
function a2s(x) { try { return JSON.stringify(x); } catch { return String(x); } }
const sleep = (ms) => new Promise((r) => { setTimeout(r, ms).unref?.(); });
try {
  await mp.mockWxMethod('request', () => ({ statusCode: 200, data: {} }));
  await mp.switchTab('/pages/dashboard/dashboard');
  await sleep(2500);
  const cur = await mp.currentPage();
  // 复位历史探针留下的渲染键（probe8 穷举把 19 个 v-if 键全翻 true；false = 原始默认态）
  const sweep = ['a','b','c','d','e','i','j','k','n','o','p','q','r','s','t','C','D','E','F'];
  const reset = {};
  for (const k of sweep) reset[k] = false;
  await cur.setData(reset);
  await sleep(1200);
  const closedB = save('dashboard-overlay-closed.png', await mp.screenshot());
  await cur.setData({ e: true });
  await sleep(1800);
  const openB = save('dashboard-overlay-open.png', await mp.screenshot());
  console.log(`EVIDENCE closed=${closedB}B open=${openB}B`);
  console.log(`CONSOLE count=${consoleMsgs.length}`);
  for (const c of consoleMsgs) console.log('  [console] ' + c);
  console.log(`EXCEPTIONS count=${exceptions.length}`);
  for (const e of exceptions) console.log('  [exception] ' + e);
} catch (e) {
  console.log('FAIL:', String(e && e.message || e));
}
await mp.disconnect();
process.exit(0);
