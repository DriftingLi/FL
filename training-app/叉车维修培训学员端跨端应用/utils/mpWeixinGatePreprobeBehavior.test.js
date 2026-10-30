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
 * 真死端口）钉住否定性闸门的四态语义（第四态 = #2147 补的进程维度）：
 *   - 必不红：端口文件不存在（冷起点）⇒ 放行；文件在 + 端口真监听 ⇒ 放行；
 *   - 必红：文件在 + 端口真死 + **IDE 进程数 ≥1** ⇒ Wedged=true（且 DeadPorts 点名文件与端口）；
 *   - 必不红（#2147）：文件在 + 端口真死 + **IDE 进程数 = 0** ⇒ stale-port-file ⇒ 放行
 *     （只剩陈旧文件 = 冷起点的另一痕迹；2026-10-30 实测这一支把门永久堵死且不自愈）；
 *   - 边界：内容不可解析 ⇒ 放行（最窄签名，不误杀可跑环境）；多根下一个活一个死 ⇒ 放行。
 *
 * 对抗式自查（#2147 起**成对跑真变异**，不再只靠「换一个夹具」）：把判据库副本里那行进程数判据
 * 摘掉 / 反向，两组断言必须**当场翻红**；只会「换真值不改代码」的自查证明不了判别力（guards.md 判据①）。
 * 平台边界：判据库是纯 .NET（TcpClient / Get-ChildItem），POSIX pwsh 同样可执行 —— 用例平台无关，
 * 门在 POSIX 上不跑 ② 不影响本守护的意义（判据真源不随平台分叉）。
 */

const net = require('net');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { execFileSync } = require('child_process');
// 读取层归一（ADR-0019 / #1177）：变异腿要读判据库源码 —— 裸 readFileSync 在 core.autocrlf=true 的
// Windows 检出上会让多行锚点撞 \r 匹配 0 次（本仓 scripts/check-contract-read.mjs 守这一条）。
const { readText } = require('./utsHarness');

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
 * `procs` = 进程数夹具（#2147）：判据库里那一次 Get-Process 是**可注入的一维** —— 不注入的话
 * 「0 个 / ≥1 个」两态在 CI（本机无 IDE）上根本造不出来，守护会退化成「只测放行那一侧」。
 * `libPath` = 判据库载体（缺省 = 仓内核；变异腿指向被改坏的临时副本）。
 * @param {{key: string, roots: string[], procs?: number}[]} cases 用例清单
 */
function probeLib(cases, timeoutMs, libPath) {
  const lib = libPath || path.join(ROOT, LIB_REL.split('/').join(path.sep));
  const handoff = path.join(fs.mkdtempSync(path.join(os.tmpdir(), 'mp-preprobe-io-')), 'cases.json');
  fs.writeFileSync(handoff, JSON.stringify(cases), 'utf8');
  const script = [
    "[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)",
    "$ErrorActionPreference = 'Stop'",
    'Set-StrictMode -Version Latest',
    `. '${lib}'`,
    `$cases = (Get-Content -LiteralPath '${handoff.replace(/\\/g, '\\\\')}' -Raw | ConvertFrom-Json)`,
    '$out = [ordered]@{}',
    // 取属性一律走 PSObject.Properties：harness 是 Set-StrictMode -Version Latest，缺省字段直接点取会抛
    `foreach ($c in $cases) { $pc = $c.PSObject.Properties['procs']; $probe = if ($pc) { [scriptblock]::Create("$($pc.Value)") } else { $null }; $out[$c.key] = Test-MpWeixinDevToolsSessionReachable -UserDataRoots ([string[]]@($c.roots)) -ConnectTimeoutMs ${timeoutMs} -IdeProcessProbe $probe }`,
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

/**
 * 变异腿：把判据库源码改坏后落到临时副本（**不碰仓内文件**），返回 { file, dir } 供清理。
 * 判据库是 dot-source 的真载体，所以「改坏它」是把判别力测出来的唯一办法（C20 教训）。
 */
function mutatedLib(src, from, to) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'mp-preprobe-lib-'));
  const file = path.join(dir, 'mp-weixin-preprobe.ps1');
  fs.writeFileSync(file, String(src).replace(from, to), 'utf8');
  return { file, dir };
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

  it('必红：文件在 + 端口真死 + IDE 进程数 ≥1 ⇒ Wedged=true，DeadPorts 点名文件与端口', async () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'mp-preprobe-dead-'));
    const port = await deadPort();
    // 真实 Windows 形态（2026-10-24 实测）：<root>\User Data\<MD5>\Default\.ide —— 嵌套两层，
    // 枚举器的有界递归必须够到它（此前一级通配只够到 <root>\<一级>\Default\.ide，真机静默漏拦）
    const nested = path.join(dir, 'User Data', '389e542e484c9a41faec27e03bd50a9d', 'Default');
    fs.mkdirSync(nested, { recursive: true });
    fs.writeFileSync(path.join(nested, '.ide'), String(port), 'utf8');
    try {
      // #2147：进程数必须显式注入 —— 本机/CI 都没有 IDE 在跑时，真实进程数恒 0，
      // 不注入就变成了「测 stale 支」而不是「测楔死支」（两侧都要有夹具，缺一侧即假绿）
      const out = parseOut(probeLib([{ key: 'dead', roots: [dir], procs: 1 }], 2000));
      expect(out.dead.Wedged).toBe(true);
      expect(out.dead.StalePortFile).toBe(false);
      expect(out.dead.IdeProcesses).toBe(1);
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

  it('必不红（#2147）：文件在 + 试连失败 + IDE 进程数 0 ⇒ stale-port-file，按冷起点放行', async () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'mp-preprobe-stale-'));
    const port = await deadPort();
    fs.writeFileSync(path.join(dir, '.ide'), String(port), 'utf8');
    try {
      const out = parseOut(probeLib([{ key: 'stale', roots: [dir], procs: 0 }], 2000));
      // 放行 = 「文件在但连不上」这一支被进程维度劈开后**新长出来的那半边**（病根：被拦死不退、不自愈）
      expect(out.stale.Wedged).toBe(false);
      expect(out.stale.StalePortFile).toBe(true);
      expect(out.stale.IdeProcesses).toBe(0);
      // 观察串必须点名这一支：门是「按冷起点放行」，不是「假装 IDE 在跑」
      expect(out.stale.Observed).toContain('stale-port-file');
      // 陈旧痕迹仍然如实点名（放行不等于把证据吞掉）
      expect(out.stale.DeadPorts.length).toBe(1);
      expect(out.stale.DeadPorts[0]).toContain(String(port));
    } finally {
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });

  it('变异必红（#2147）：把进程数判据摘掉 / 反向，两侧断言必须当场翻红', async () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'mp-preprobe-mutant-'));
    const port = await deadPort();
    fs.writeFileSync(path.join(dir, '.ide'), String(port), 'utf8');
    const SRC = readText(path.join(ROOT, LIB_REL.split('/').join(path.sep)));
    // 判据的载体就是这一行：进程数只在这一处改变结论
    const JUDGE = 'if ($ideCount -eq 0) {';
    let a = null;
    let b = null;
    try {
      expect(SRC).toContain(JUDGE);
      // ① 摘掉进程维度（没人再走 stale 支）⇒ 进程 0 的夹具被拦死 —— 与 #2147 的病根同形
      a = mutatedLib(SRC, JUDGE, 'if ($ideCount -eq -1) {');
      const outA = parseOut(probeLib([{ key: 'stale', roots: [dir], procs: 0 }], 2000, a.file));
      expect(outA.stale.Wedged).toBe(true);
      // ② 反向（≥1 也当陈旧）⇒ 真楔死的夹具被放行 —— 假绿那一侧
      b = mutatedLib(SRC, JUDGE, 'if ($ideCount -ge 0) {');
      const outB = parseOut(probeLib([{ key: 'dead', roots: [dir], procs: 2 }], 2000, b.file));
      expect(outB.dead.Wedged).toBe(false);
      expect(outB.dead.StalePortFile).toBe(true);
    } finally {
      [a, b].forEach((m) => { if (m) fs.rmSync(m.dir, { recursive: true, force: true }); });
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });
});
