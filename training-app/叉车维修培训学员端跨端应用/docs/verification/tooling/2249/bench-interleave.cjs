// #2249 · A/B 交替穿插对照（抗机器漂移）：每次 A、B 相邻跑，差值才是 npm 层成本
const { spawn } = require('node:child_process');
const path = require('node:path');
const CWD = path.resolve(__dirname, '..');
const N = Number(process.env.BENCH_N || 5);
const SUITE = 'utils/credentialSwitchBehavior.test.js';

function run(cmd, args) {
  return new Promise((resolve) => {
    const t0 = process.hrtime.bigint();
    const p = spawn(cmd, args, { cwd: CWD, stdio: ['ignore', 'pipe', 'pipe'], windowsHide: true });
    let out = '';
    p.stdout.on('data', (d) => (out += d));
    p.stderr.on('data', (d) => (out += d));
    p.on('close', () => {
      const m = out.match(/Time:\s+([\d.]+)\s*s/);
      resolve({ wallMs: Number((process.hrtime.bigint() - t0) / 1000000n), jest: m ? Number(m[1]) : null });
    });
  });
}

const A = () => run(process.env.ComSpec || 'cmd.exe', ['/d', '/s', '/c', `npm run test:behavior -- --runTestsByPath ${SUITE}`]);
const B = () => run(process.execPath, [path.join(CWD, 'node_modules', 'jest', 'bin', 'jest.js'), '--config', 'jest.config.behavior.js', '-i', '--runTestsByPath', SUITE]);

(async () => {
  const rows = [];
  for (let i = 1; i <= N; i++) {
    const a = await A();
    const b = await B();
    rows.push({ n: i, aWall: a.wallMs, aJest: a.jest, bWall: b.wallMs, bJest: b.jest, npmMs: a.wallMs - b.wallMs });
    console.log(`n=${i} A=${a.wallMs}ms(jest ${a.jest}s) B=${b.wallMs}ms(jest ${b.jest}s) npm层=${a.wallMs - b.wallMs}ms`);
  }
  console.log('---JSON---');
  console.log(JSON.stringify(rows));
})();
