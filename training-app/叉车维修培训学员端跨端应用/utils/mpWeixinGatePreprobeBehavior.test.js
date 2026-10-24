/**
 * ② 门「publish 前秒级环境预探」行为守护（#1860）。
 *
 * 判据真源 = `scripts/lib/mp-weixin-preprobe.ps1`（门脚本 dot-source 同一份）。
 * 病根：② 的失败轮 9–26 min/次白烧在「开发者工具会话不就绪」族 —— 环境被楔死时，门把
 * publish（≈2–3 min）+ close→open→auto 整段重试全烧完才暴露。预探把楔死态的暴露点提到
 * publish **之前**（秒级），楔死签名 = **.ide 端口文件在但其服务端口试连不上**（微信官方
 * HTTP 文档的检测法：「检查用户目录下是否有端口文件及尝试连接」）。
 *
 * 为什么判据是**真执行库**而不是断言门脚本源码文本：文本断言在「判据被摘掉、只留一句字面量」时
 * 照样绿（C20 教训）。这里用**夹具用户目录**注入 .ide 端口文件 + **真 TCP**（node 起真监听器 /
 * 真死端口）钉住否定性闸门的三态语义：
 *   - 必不红：端口文件不存在（冷起点）⇒ 放行；文件在 + 端口真监听 ⇒ 放行；
 *   - 必红：文件在 + 端口真死 ⇒ Wedged=true（且 DeadPorts 点名文件与端口）；
 *   - 边界：内容不可解析 ⇒ 放行（最窄签名，不误杀可跑环境）；多根下一个活一个死 ⇒ 放行。
 *
 * 对抗式自查：把判据改坏（改成「文件不存在也拦」/「内容不可解析也拦」）时，必红/必不红夹具当场翻红。
 * 平台边界：判据库是纯 .NET（TcpClient / Get-ChildItem），POSIX pwsh 同样可执行 —— 用例平台无关，
 * 门在 POSIX 上不跑 ② 不影响本守护的意义（判据真源不随平台分叉）。
 */

const net = require('net');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { execFileSync } = require('child_process');

const ROOT = path.join(__dirname, '..');
// ⚠️ 载体路径写成**本文件内的字面量**（③ 门分类器按文件自身的「执行调用 + 仓内载体引用」判类）
const LIB_REL = 'scripts/lib/mp-weixin-preprobe.ps1';

function listenOn(port) {
  return new Promise((resolve, reject) => {
    const srv = net.createServer();
    srv.on('error', reject);
    srv.listen(port, '127.0.0.1', () => resolve(srv));
  });
}

/** 取一个当前空闲的端口（先占住再放掉 = 试连必死的端口）。 */
function deadPort() {
  return new Promise((resolve, reject) => {
    const srv = net.createServer();
    srv.on('error', reject);
    srv.listen(0, '127.0.0.1', () => {
      const p = srv.address().port;
      srv.close(() => resolve(p));
    });
  });
}

/**
 * 起 pwsh：dot-source 判据库 → 对注入的夹具根跑预探 → 回传机读 JSON。
 * 用例经 JSON 文件交接（路径里的反斜杠/引号不经过 shell 拼串，免得转义层出岔）。
 * @param {{key: string, roots: string[]}[]} cases 用例清单
 */
function probeLib(cases, timeoutMs) {
  const libPath = path.join(ROOT, LIB_REL.split('/').join(path.sep));
  const handoff = path.join(fs.mkdtempSync(path.join(os.tmpdir(), 'mp-preprobe-io-')), 'cases.json');
  fs.writeFileSync(handoff, JSON.stringify(cases), 'utf8');
  const script = [
    "[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)",
    "$ErrorActionPreference = 'Stop'",
    'Set-StrictMode -Version Latest',
    `. '${libPath}'`,
    `$cases = (Get-Content -LiteralPath '${handoff.replace(/\\/g, '\\\\')}' -Raw | ConvertFrom-Json)`,
    '$out = [ordered]@{}',
    `foreach ($c in $cases) { $out[$c.key] = Test-MpWeixinDevToolsSessionReachable -UserDataRoots ([string[]]@($c.roots)) -ConnectTimeoutMs ${timeoutMs} }`,
    '$out | ConvertTo-Json -Depth 6 -Compress'
  ].join('\r\n');
  try {
    return execFileSync('pwsh', ['-NoProfile', '-NonInteractive', '-Command', script], {
      encoding: 'utf8', timeout: 120000, windowsHide: true, maxBuffer: 4 * 1024 * 1024
    });
  } finally {
    fs.rmSync(path.dirname(handoff), { recursive: true, force: true });
  }
}

function parseOut(stdout) {
  const line = String(stdout).split(/\r?\n/).filter((l) => l.trim().startsWith('{')).pop();
  if (!line) throw new Error(`预探库无可解析 JSON 输出：\n${stdout}`);
  return JSON.parse(line);
}

describe('② 环境预探行为（#1860：否定性闸门 —— 只拦「.ide 端口文件在但试连不上」）', () => {
  it('必不红：端口文件不存在（冷起点）⇒ 放行；文件在 + 端口真监听 ⇒ 放行', async () => {
    const dirCold = fs.mkdtempSync(path.join(os.tmpdir(), 'mp-preprobe-cold-'));
    const dirLive = fs.mkdtempSync(path.join(os.tmpdir(), 'mp-preprobe-live-'));
    const srv = await listenOn(0);
    const livePort = srv.address().port;
    fs.writeFileSync(path.join(dirLive, '.ide'), String(livePort), 'utf8');
    try {
      const out = parseOut(probeLib([
        { key: 'cold', roots: [dirCold] },
        { key: 'live', roots: [dirLive] }
      ], 2000));
      expect(out.cold.Wedged).toBe(false);
      expect(out.cold.Observed).toContain('no-port-file');
      expect(out.cold.DeadPorts).toEqual([]);
      expect(out.live.Wedged).toBe(false);
      expect(out.live.Observed).toContain('试连可达');
    } finally {
      srv.close();
      fs.rmSync(dirCold, { recursive: true, force: true });
      fs.rmSync(dirLive, { recursive: true, force: true });
    }
  });

  it('必红：文件在 + 端口真死 ⇒ Wedged=true，DeadPorts 点名文件与端口', async () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'mp-preprobe-dead-'));
    const port = await deadPort();
    // 真实 Windows 形态（2026-10-24 实测）：<root>\User Data\<MD5>\Default\.ide —— 嵌套两层，
    // 枚举器的有界递归必须够到它（此前一级通配只够到 <root>\<一级>\Default\.ide，真机静默漏拦）
    const nested = path.join(dir, 'User Data', '389e542e484c9a41faec27e03bd50a9d', 'Default');
    fs.mkdirSync(nested, { recursive: true });
    fs.writeFileSync(path.join(nested, '.ide'), String(port), 'utf8');
    try {
      const out = parseOut(probeLib([{ key: 'dead', roots: [dir] }], 2000));
      expect(out.dead.Wedged).toBe(true);
      expect(out.dead.Observed).toContain('port-file-dead');
      expect(out.dead.DeadPorts.length).toBe(1);
      expect(out.dead.DeadPorts[0]).toContain(String(port));
      expect(out.dead.DeadPorts[0]).toContain('User Data');
    } finally {
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });

  it('边界：内容不可解析 ⇒ 放行（不误杀）；多根一活一死 ⇒ 放行（任一可达即不拦）', async () => {
    const dirGarbage = fs.mkdtempSync(path.join(os.tmpdir(), 'mp-preprobe-garbage-'));
    const dirDead = fs.mkdtempSync(path.join(os.tmpdir(), 'mp-preprobe-mix-dead-'));
    const dirLive = fs.mkdtempSync(path.join(os.tmpdir(), 'mp-preprobe-mix-live-'));
    fs.writeFileSync(path.join(dirGarbage, '.ide'), 'not-a-port', 'utf8');
    const port = await deadPort();
    fs.writeFileSync(path.join(dirDead, '.ide'), String(port), 'utf8');
    const srv = await listenOn(0);
    const livePort = srv.address().port;
    fs.writeFileSync(path.join(dirLive, '.ide'), String(livePort), 'utf8');
    try {
      const out = parseOut(probeLib([
        { key: 'garbage', roots: [dirGarbage] },
        { key: 'mixed', roots: [dirDead, dirLive] }
      ], 2000));
      expect(out.garbage.Wedged).toBe(false);
      expect(out.garbage.Observed).toContain('不可解析');
      expect(out.mixed.Wedged).toBe(false);
      expect(out.mixed.Observed).toContain('试连可达');
    } finally {
      srv.close();
      fs.rmSync(dirGarbage, { recursive: true, force: true });
      fs.rmSync(dirDead, { recursive: true, force: true });
      fs.rmSync(dirLive, { recursive: true, force: true });
    }
  });
});
