// #2249 · 复刻「上一页的分离监听器还活着」形态下 Wait-MpWeixinAutoPortReleased 的真实成本
// 场景：监听器 60 s 后自灭（与 makeListeningDevTools 的 fake-listener.js 同形），
//       门在每页冷启动前调 Wait-MpWeixinAutoPortReleased（预算 60 轮，每轮一次 Get-NetTCPConnection + 1 s sleep）。
// 记录：每轮 queryMs、直到端口释放的轮数与总墙钟。
const net = require('node:net');
const { execFileSync } = require('node:child_process');

const PORT = Number(process.argv[2] || 47781);
const LIFE_MS = Number(process.argv[3] || 60000);

const script = (port) => `
$sw = [System.Diagnostics.Stopwatch]::StartNew()
$held = 0
while ($held -lt 60) {
    $t0 = $sw.ElapsedMilliseconds
    $n = @(Get-NetTCPConnection -State Listen -LocalPort ${port} -ErrorAction SilentlyContinue).Count
    $t1 = $sw.ElapsedMilliseconds
    "ROUND $held count=$n queryMs=$($t1 - $t0) totalMs=$($t1)"
    if ($n -eq 0) { "RELEASED at round $held" ; break }
    Start-Sleep -Seconds 1
    $held++
}
"LOOP_TOTAL_MS=$($sw.ElapsedMilliseconds) rounds=$held"
`;

(async () => {
  const srv = net.createServer(() => {});
  await new Promise((r) => srv.listen(PORT, '127.0.0.1', r));
  console.log(`监听 ${PORT}，${LIFE_MS} ms 后自灭（复刻 fake-listener.js 的 60s 生命期）`);
  setTimeout(() => srv.close(() => {}), LIFE_MS);
  const out = execFileSync('pwsh', ['-NoProfile', '-NonInteractive', '-Command', script(PORT)], { encoding: 'utf8', windowsHide: true });
  console.log(out.trim());
})().catch((e) => { console.error('ERR', e.message); process.exit(1); });
