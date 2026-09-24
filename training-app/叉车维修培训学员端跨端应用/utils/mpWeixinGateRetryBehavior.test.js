/**
 * ② 门「整段重试」行为守护（2026-09-23 实测定位，坑位 6）。
 *
 * 病根（本机实测，两次独立复现）：冷起点 / IDE 重启窗口下 **第一次** `cli.bat auto` 会
 * 「回显 `√ auto` 但自动化端口在预算内始终不监听」或「直接挂住 >180s」；**同一条命令**等 IDE 稳定后
 * 重跑 1–8s 即起。故执行路径把 `close → open → auto → 端口` 当**一个可重试的单元**跑 attempts 次
 * （`scripts/mp-weixin-check.ps1`，计划面判据 C21）。
 *
 * 为什么守护要**真跑门**而不是断言源码文本：文本断言在「循环被摘掉、只留一句字面量」时照样绿。
 * 这里用共享夹具的**假开发者工具**（只记账 + 回显，**永不**监听端口 = 冷起点第一枪的形态）把门真跑一遍，
 * 判据是**调用序列**与**结果行**：
 *   - 必不红（attempts=2）：`close → open → auto` 必须**整段**出现两遍（不是只重试 auto），
 *     且两次都用尽才 `exit 2` / `reason=port-not-listening`；
 *   - 必红（attempts=1）：只允许出现一遍 —— 这条同时证明「次数真的来自 `-AutoAttempts`」，
 *     而不是写死的两遍（否则本条会红）。
 *
 * 夹具与平台边界口径见 `utils/mpWeixinGateHarness.js`（非 Windows 分支在此真断言 fail-closed）。
 */
const fs = require('fs');
const path = require('path');
const { execFileSync } = require('child_process');
const H = require('./mpWeixinGateHarness');

const IS_WIN = H.IS_WIN;
// ⚠️ 载体路径写成**本文件内的字面量**：③ 门判据真源 `scripts/classify-guards.mjs` 按文件自身的
// 「代码级执行调用 + 仓内载体引用」判行为守护 ⇒ 两样都不能藏进共享夹具（藏了会被判成接线守护）。
const SCRIPT_REL = 'scripts/mp-weixin-check.ps1';

/** 真跑门（起子进程；参数由夹具的 `gateArgs` 拼，执行留在本文件里）。 */
function runGate(fixture, fakeCli, port, extraArgs) {
  const args = H.gateArgs(path.join(H.ROOT, SCRIPT_REL), fixture, fakeCli, port, extraArgs);
  try {
    const stdout = execFileSync('pwsh', args, {
      encoding: 'utf8', timeout: 300000, windowsHide: true, maxBuffer: 16 * 1024 * 1024
    });
    return { status: 0, stdout };
  } catch (e) {
    return { status: e.status, stdout: String(e.stdout || '') + String(e.stderr || '') };
  }
}

describe('② 整段重试行为（坑位 6：auto 回显 √ 但端口不起 ⇒ close → open → auto 整段再来）', () => {
  let fixture = null;
  let fakeCli = '';
  let port = 0;

  beforeAll(async () => {
    fixture = H.makeFixture();
    fakeCli = H.makeFakeDevTools(fixture.dir);
    port = await H.freePort();
  }, 60000);

  afterAll(() => {
    if (fixture) fs.rmSync(fixture.dir, { recursive: true, force: true });
  });

  it('非 Windows：门在拿不到证据的环境里 fail-closed（不得产出绿）', () => {
    if (IS_WIN) return; // Windows 上由下面两条真跑（本条只钉平台边界那一支）
    const r = runGate(fixture, fakeCli, port, ['-AutoAttempts', '1']);
    expect(r.status).not.toBe(0);
    expect(r.stdout).not.toMatch(/MP_WEIXIN_RESULT errors=0/);
  });

  it('必不红：attempts=2 ⇒ close → open → auto 整段出现两遍，用尽才 exit 2', () => {
    if (!IS_WIN) return; // 平台边界由上一条真断言
    H.resetCalls(fakeCli);
    const r = runGate(fixture, fakeCli, port, ['-AutoAttempts', '2']);
    // **整段**：不是 close/open/auto 各来两次的任意组合，而是两遍完整的 close → open → auto
    expect(H.verbsOf(fakeCli)).toEqual(['close', 'open', 'auto', 'close', 'open', 'auto']);
    // 全部用尽才判环境不可用（不是第一枪就红）
    expect(r.status).toBe(2);
    const log = H.gateLog(fixture);
    expect(log).toMatch(/整段重试第 2\/2 次/);
    expect(log).toMatch(/MP_WEIXIN_RESULT errors=env reason=port-not-listening/);
  }, 300000);

  it('必红：attempts=1 ⇒ 只跑一遍（证明次数真的来自 -AutoAttempts，不是写死的）', () => {
    if (!IS_WIN) return;
    H.resetCalls(fakeCli);
    const r = runGate(fixture, fakeCli, port, ['-AutoAttempts', '1']);
    expect(H.verbsOf(fakeCli)).toEqual(['close', 'open', 'auto']);
    expect(r.status).toBe(2);
    const log = H.gateLog(fixture);
    expect(log).not.toMatch(/整段重试/);
    expect(log).toMatch(/MP_WEIXIN_RESULT errors=env reason=port-not-listening/);
  }, 300000);
});
