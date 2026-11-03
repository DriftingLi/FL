// #2249 阶段 0 · 第一件：尺子改动集分档表（只读，不执行任何测试）
// 对每一类单文件改动，现算行为腿受影响子集大小（含/不含 b′ 常驻名单）。
import { execFileSync } from 'node:child_process';

const CASES = [
  ['纯 .uts 逻辑层', 'utils/forumBody.uts'],
  ['纯 .uts 逻辑层（另一支）', 'utils/dashboardNews.uts'],
  ['纯 .uts 逻辑层（第三支）', 'utils/examInfoTabs.uts'],
  ['纯 .uvue 页面', 'pages/markdown/markdown.uvue'],
  ['运行时面 .uts（api 层）', 'api/envelope.uts'],
  ['门脚本 .ps1', 'scripts/mp-weixin-check.ps1'],
  ['套件自身 .test.js', 'utils/credentialSwitchBehavior.test.js'],
  ['页面集合 pages.json', 'pages.json'],
  ['仓根文档 .md', 'docs/agents/dev-loop.md'],
];

function changed(files, extra = []) {
  const out = execFileSync(
    process.execPath,
    ['scripts/behavior-suite-deps.mjs', '--changed-json', ...extra, '--changed', ...files],
    { encoding: 'utf8' },
  );
  const j = JSON.parse(out);
  return { total: j.total, n: j.subset.length, pct: (100 * j.subset.length) / j.total, uncovered: j.uncovered.length };
}

console.log('| 改动类别 | 代表文件 | 含 b′ 常驻 | 不含 b′ | 未覆盖 |');
console.log('| --- | --- | ---: | ---: | ---: |');
for (const [label, file] of CASES) {
  let a, b;
  try {
    a = changed([file]);
    b = changed([file], ['--no-bprime-always']);
  } catch (e) {
    console.log(`| ${label} | \`${file}\` | ERR ${String(e.message).slice(0, 60)} | | |`);
    continue;
  }
  console.log(`| ${label} | \`${file}\` | ${a.n}/${a.total} (${a.pct.toFixed(1)}%) | ${b.n}/${b.total} (${b.pct.toFixed(1)}%) | ${a.uncovered} |`);
}
