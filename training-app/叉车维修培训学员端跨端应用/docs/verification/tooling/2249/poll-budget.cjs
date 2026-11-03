// #2249 阶段 0 · 假设②机制拆账：端口轮询预算的「每轮真实成本」
// 复刻门脚本两处预算的循环形状，逐轮记录毫秒：
//   A) Wait-MpWeixinAutoPortReleased（:1353-1362）：端口**被占**时付满 60 轮
//      while ($held -lt 60) { if (count -eq 0) return; Start-Sleep 1; $held++ }
//   B) port-listening 轮询（:2246-2248 / :1534-1536）：端口**永不监听**时付满 waitSeconds 轮
//      for ($i=0; $i -lt N; $i++) { if (count -gt 0) break; Start-Sleep 1 }
const net = require('node:net');
const { execFileSync } = require('node:child_process');

const PORT = Number(process.argv[2] || 47771);
const ROUNDS = Number(process.argv[3] || 60);

const PS = (script) => execFileSync('pwsh', ['-NoProfile', '-NonInteractive', '-Command', script], { encoding: 'utf8', windowsHide: true });

const script = (port, rounds, expectHeld) => `
$sw = [System.Diagnostics.Stopwatch]::StartNew()
for ($i = 0; $i -lt ${rounds}; $i++) {
    $t0 = $sw.ElapsedMilliseconds
    $n = @(Get-NetTCPConnection -State Listen -LocalPort ${port} -ErrorAction SilentlyContinue).Count
    $t1 = $sw.ElapsedMilliseconds
    ${expectHeld
      ? 'if ($n -eq 0) { "ROUND $i count=0 STOP"; break }'
      : 'if ($n -gt 0) { "ROUND $i count=$n STOP"; break }'}
    "ROUND $i count=$n queryMs=$($t1 - $t0) totalMs=$($t1)"
    Start-Sleep -Seconds 1
}
"LOOP_TOTAL_MS=$($sw.ElapsedMilliseconds)"
`;

(async () => {
  // ---- A：端口被占（复刻 makeListeningDevTools 的分离监听器形态）----
  const srv = net.createServer(() => {});
  await new Promise((r) => srv.listen(PORT, '127.0.0.1', r));
  console.log(`=== A 端口被占（监听 ${PORT}）· 复刻 Wait-MpWeixinAutoPortReleased · 预算 ${ROUNDS} 轮 ===`);
  const a = PS(script(PORT, ROUNDS, true));
  console.log(a);

  // ---- B：端口永不监听（无服务）----
  await new Promise((r) => srv.close(r));
  await new Promise((r) => setTimeout(r, 500));
  console.log(`=== B 端口永不监听（${PORT} 空闲）· 复刻 port-listening 轮询 · 预算 5 轮（套件实测值）===`);
  const b = PS(script(PORT, 5, false));
  console.log(b);
})().catch((e) => { console.error('ERR', e.message); process.exit(1); });
