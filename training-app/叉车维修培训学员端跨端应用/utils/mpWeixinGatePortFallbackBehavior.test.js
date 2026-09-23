/**
 * ② 门「残留会话 → 换端口」行为守护（2026-09-23 实测定位，ADR-0008 坑位 6 续）。
 *
 * 病根：残留的自动化服务会让 `cli.bat auto` **静默复用它** —— 请求的端口在监听、ws 也接得上，
 * 但 `Tool.getInfo` **全程无应答**（就绪闸门 `reason=sdk-version-missing`），表象酷似「工具起不来」。
 * 实测处置：换一个空闲端口重跑，同一条命令立刻恢复（2026-09-23 换到 9431 后 ② 一次通过）。
 * 本守护钉住的是「门自己会换」，而不是靠人记得加 `-Port`：
 *   - 必不红（端口被占）：`auto` 收到的 `--auto-port` **必须不等于**被占的那个端口，且日志里有 `PORT_SWITCH`；
 *   - 必红（端口空闲）：`auto` 收到的 `--auto-port` **必须等于**请求端口，且日志里**不得**出现 `PORT_SWITCH`
 *     —— 这条同时挡住「无条件换端口」这种把请求端口当摆设的写法（否则本条会红）。
 *
 * 平台边界与夹具口径见 `utils/mpWeixinGateHarness.js`（非 Windows 分支在此真断言 fail-closed）。
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

describe('② 残留自动化会话 ⇒ 换端口（auto 会静默复用死会话，只 warning 不够）', () => {
  let fixture = null;
  let fakeCli = '';
  let occupied = null;

  beforeAll(async () => {
    fixture = H.makeFixture();
    fakeCli = H.makeFakeDevTools(fixture.dir);
    occupied = await H.occupyPort();
  }, 60000);

  afterAll(async () => {
    if (occupied) await occupied.close();
    if (fixture) fs.rmSync(fixture.dir, { recursive: true, force: true });
  });

  it('非 Windows：门在拿不到证据的环境里 fail-closed（不得产出绿）', () => {
    if (IS_WIN) return; // Windows 上由下面两条真跑（本条只钉平台边界那一支）
    const r = runGate(fixture, fakeCli, occupied.port, ['-AutoAttempts', '1']);
    expect(r.status).not.toBe(0);
    expect(r.stdout).not.toMatch(/MP_WEIXIN_RESULT errors=0/);
  });

  it('必不红：请求端口被占 ⇒ auto 换到别的端口，且日志记下 PORT_SWITCH', () => {
    if (!IS_WIN) return; // 平台边界由上一条真断言
    H.resetCalls(fakeCli);
    const r = runGate(fixture, fakeCli, occupied.port, ['-AutoAttempts', '1']);
    const usedPort = H.lastFlagValue(fakeCli, 'auto', '--auto-port');
    expect(usedPort).toBeDefined();
    expect(usedPort).not.toBe(String(occupied.port)); // 没复用那个死会话
    expect(Number(usedPort)).toBeGreaterThan(occupied.port); // 沿端口号向后找
    const log = H.gateLog(fixture);
    expect(log).toMatch(/PORT_SWITCH/);
    // 换端口不是「门通过」的理由：假开发者工具永不开端口 ⇒ 仍判环境不可用
    expect(r.status).toBe(2);
    expect(log).toMatch(/MP_WEIXIN_RESULT errors=env reason=port-not-listening/);
  }, 300000);

  it('必红：请求端口空闲 ⇒ 必须沿用请求端口（不得无条件换）', async () => {
    if (!IS_WIN) return;
    H.resetCalls(fakeCli);
    const free = await H.freePort();
    const r = runGate(fixture, fakeCli, free, ['-AutoAttempts', '1']);
    expect(H.lastFlagValue(fakeCli, 'auto', '--auto-port')).toBe(String(free));
    expect(H.gateLog(fixture)).not.toMatch(/PORT_SWITCH/);
    expect(r.status).toBe(2);
  }, 300000);
});
