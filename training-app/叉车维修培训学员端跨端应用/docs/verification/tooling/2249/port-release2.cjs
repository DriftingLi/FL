// #2249 · 修正版：监听器在**独立进程**里 60 s 后自灭（不阻塞主线程），复刻 fake-listener.js 的生命期
// 目的：量化「上一页的分离监听器还活着」时 Wait-MpWeixinAutoPortReleased 要付多少轮、多少秒
const { spawn, execFileSync } = require('node:child_process');
const path = require('node:path');
const fs = require('node:fs');
const os = require('node:os');

const PORT = Number(process.argv[2] || 47791);
const LIFE_MS = Number(process.argv[3] || 60000);

const listenerPath = path.join(os.tmpdir(), `m2249-listener-${PORT}.js`);
fs.writeFileSync(listenerPath, `
const net = require('net');
const srv = net.createServer(() => {});
srv.listen(${PORT}, '127.0.0.1', () => console.log('LISTENING'));
setTimeout(() => { srv.close(() => process.exit(0)); }, ${LIFE_MS});
`, 'utf8');

const listener = spawn(process.execPath, [listenerPath], { stdio: ['ignore', 'pipe', 'inherit'] });
listener.stdout.once('data', () => {
  console.log(`listener 已监听 ${PORT}，${LIFE_MS} ms 后自灭`);
  const script = `
$sw = [System.Diagnostics.Stopwatch]::StartNew()
$held = 0
while ($held -lt 60) {
    $t0 = $sw.ElapsedMilliseconds
    $n = @(Get-NetTCPConnection -State Listen -LocalPort ${PORT} -ErrorAction SilentlyContinue).Count
    $t1 = $sw.ElapsedMilliseconds
    "ROUND $held count=$n queryMs=$($t1 - $t0) totalMs=$($t1)"
    if ($n -eq 0) { "RELEASED at round $held"; break }
    Start-Sleep -Seconds 1
    $held++
}
"LOOP_TOTAL_MS=$($sw.ElapsedMilliseconds) rounds=$held"
`;
  const out = execFileSync('pwsh', ['-NoProfile', '-NonInteractive', '-Command', script], { encoding: 'utf8', windowsHide: true });
  console.log(out.trim());
  try { listener.kill(); } catch (e) {}
});
