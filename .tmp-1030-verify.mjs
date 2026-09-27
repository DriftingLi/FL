// #1030 真实链路复演：把 **PR #1032 的真实改动集**喂给「修复前 / 修复后」两份校验器，看 ② 门的判据各自怎么判。
//
// 为什么不是「看 PR 的 pr-evidence check 绿不绿」：校验器对**不含运行时面的改动集直接放行**
// （`runtime.length === 0` 提前 return），而本 PR 只改 CI 配置 + 测试 + 文档 ⇒ 走不到 ② 这道门，
// 拿它当判据等于空跑。故这里补一个**合成的 *.uvue**（只为跨过那次提前 return），
// 于是被比较的维度收敛成一条：**② 门是否被点亮**（① ④ 的「缺行」是注入运行时面的必然产物，两边都会报，不作判据）。
//
// 输入（由 pwsh 侧物化，Node 侧只读文件、不 spawn 子进程）：
//   .tmp-1030-wf-old.yml    origin/master 的 pr-evidence.yml
//   .tmp-1030-wf-new.yml    fix/1030-gate2-trigger 的 pr-evidence.yml
//   .tmp-1030-pr-files.json gh api repos/…/pulls/1032/files 的原样输出
//   .tmp-1030-pr-body.md    PR #1032 的真实正文
//
// 运行：node .tmp-1030-verify.mjs

import { readFileSync } from 'node:fs';

const loadValidator = (path) => {
  const src = readFileSync(path, 'utf8');
  const m = src.match(/\/\/ ==== PR-EVIDENCE-VALIDATOR-START ====([\s\S]*?)\/\/ ==== PR-EVIDENCE-VALIDATOR-END ====/);
  if (!m) throw new Error(`找不到校验器标记块：${path}`);
  return new Function(`${m[1]}\nreturn validatePrEvidence;`)();
};

const prFiles = JSON.parse(readFileSync('.tmp-1030-pr-files.json', 'utf8'));
const body = readFileSync('.tmp-1030-pr-body.md', 'utf8');

// 合成探针：只为让校验器走到门校验（② 的判据面与它无关，它自己不携带该术语）
const PROBE = {
  filename: 'training-app/叉车维修培训学员端跨端应用/pages/exam/exam.uvue',
  status: 'modified',
  patch: '@@ -1,2 +1,3 @@\n <view class="page">\n+  <text>probe</text>\n </view>\n',
};

const files = [...prFiles.map((f) => ({ filename: f.filename, status: f.status, patch: f.patch })), PROBE];
console.log(`PR #1032 真实改动集 ${prFiles.length} 个文件 + 1 个合成 .uvue 探针\n`);
for (const f of prFiles) {
  const hit = /MP-WEIXIN/.test(f.patch || '');
  console.log(`  ${hit ? '含该术语' : '不含术语'}  ${f.filename}`);
}

const REPORT = [];
for (const [label, path] of [
  ['修复前（origin/master）', '.tmp-1030-wf-old.yml'],
  ['修复后（本 PR）', '.tmp-1030-wf-new.yml'],
]) {
  const r = await loadValidator(path)({ files, body, author: 'zhengcookie' });
  const gate2Err = r.errors.filter((e) => e.includes('②'));
  const gate2Free = r.notes.some((n) => n.includes('第②门免'));
  const gate2Hit = r.notes.some((n) => n.includes('② 触发面命中'));
  REPORT.push({ label, gate2Err, gate2Free, gate2Hit, errors: r.errors });
  console.log(`\n===== ${label} =====`);
  console.log(`② 相关报错：${gate2Err.length ? gate2Err.join(' | ') : '（无）'}`);
  console.log(`notes 里「第②门免」：${gate2Free}；「② 触发面命中」：${gate2Hit}`);
  console.log(`全部报错：\n  - ${r.errors.join('\n  - ')}`);
}

const [oldR, newR] = REPORT;
console.log('\n===== 结论 =====');
const oldLit = oldR.gate2Err.length > 0;
const newLit = newR.gate2Err.length > 0;
console.log(`旧校验器把 ② 点亮（复现 BUG）：${oldLit}`);
console.log(`新校验器把 ② 点亮：${newLit}`);
console.log(`新校验器说明「第②门免」：${newR.gate2Free}`);
console.log(
  oldLit && !newLit
    ? '✅ 同一份真实改动集：旧判据红在 ②，新判据不要求 ② —— 本修复在真实 PR 的改动集上成立'
    : '❌ 未呈现「旧红新绿」的差异，需复查',
);
