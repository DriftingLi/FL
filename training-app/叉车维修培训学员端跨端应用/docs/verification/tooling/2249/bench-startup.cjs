// #2249 阶段 0 · 第三件：启动拆账（三形态 × n 次）
// 形态 A：cmd → npm → jest（与 dev-finish.ps1 逐套件驱动同形：`npm run test:behavior -- --runTestsByPath <suite>`）
// 形态 B：node → jest 直调（绕开 cmd 与 npm 两层：`node node_modules/jest/bin/jest.js --config ...`）
// 形态 C：单进程双套件（一次 jest 起两个套件，把单次启动摊到两个套件上）
const { spawn } = require('node:child_process');
const path = require('node:path');

const CWD = path.resolve(__dirname, '..');
const N = Number(process.env.BENCH_N || 5);
// 两个都在 jest.config.behavior.js 的 --listTests 里（现测 106 套件），且行数最小
const SUITE_A = 'utils/credentialSwitchBehavior.test.js';
const SUITE_B = 'utils/utsHarnessDedupContract.test.js';
const FORMS = String(process.env.BENCH_FORMS || 'A,B,C').split(',');

function runOnce(form) {
  return new Promise((resolve) => {
    const jest = path.join(CWD, 'node_modules', 'jest', 'bin', 'jest.js');
    let cmd, args;
    if (form === 'A') {
      cmd = process.env.ComSpec || 'cmd.exe';
      args = ['/d', '/s', '/c', `npm run test:behavior -- --runTestsByPath ${SUITE_A}`];
    } else if (form === 'B') {
      cmd = process.execPath;
      args = [jest, '--config', 'jest.config.behavior.js', '-i', '--runTestsByPath', SUITE_A];
    } else if (form === 'C') {
      cmd = process.execPath;
      args = [jest, '--config', 'jest.config.behavior.js', '-i', '--runTestsByPath', SUITE_A, SUITE_B];
    } else {
      return resolve({ form, error: 'unknown form ' + form });
    }
    const t0 = process.hrtime.bigint();
    const p = spawn(cmd, args, { cwd: CWD, stdio: ['ignore', 'pipe', 'pipe'], windowsHide: true });
    let out = '';
    p.stdout.on('data', (d) => { out += d.toString(); });
    p.stderr.on('data', (d) => { out += d.toString(); });
    p.on('close', (code) => {
      const ms = Number((process.hrtime.bigint() - t0) / 1000000n);
      const t = out.match(/Tests:\s+(.*)/);
      const tm = out.match(/Time:\s+([\d.]+)\s*s/);
      const suites = out.match(/Test Suites:\s+(.*)/);
      resolve({
        form, code, wallMs: ms,
        suitesLine: suites ? suites[1].trim() : null,
        testsLine: t ? t[1].trim() : null,
        jestTimeSec: tm ? Number(tm[1]) : null,
      });
    });
  });
}

(async () => {
  const rows = [];
  for (const form of FORMS) {
    for (let i = 1; i <= N; i++) {
      const r = await runOnce(form);
      r.n = i;
      rows.push(r);
      console.log(`form=${form} n=${i} wallMs=${r.wallMs} jestTimeSec=${r.jestTimeSec} tests=${r.testsLine} code=${r.code}`);
    }
  }
  console.log('---JSON---');
  console.log(JSON.stringify(rows));
})();
