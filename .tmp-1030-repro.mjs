// #1030 最小复现：② 门触发判据扫 diff 文本里的 MP-WEIXIN 字面量。
// 场景 = PR #1028 的真实形态：改动集里有 .uvue（运行时面）+ 一份 ADR（文档），
// ADR 的 diff 里写了一句「② 免（未命中 MP-WEIXIN 面）」——没有任何条件编译段、没碰两份 json。
// 期望：不点亮 ②（正文缺 ② 行也应绿）。实际（修复前）：② 被点亮 ⇒ PR 判红。
//
// 运行：node .tmp-1030-repro.mjs

import { readFileSync } from 'node:fs';

const src = readFileSync('.github/workflows/pr-evidence.yml', 'utf8');
const m = src.match(/\/\/ ==== PR-EVIDENCE-VALIDATOR-START ====([\s\S]*?)\/\/ ==== PR-EVIDENCE-VALIDATOR-END ====/);
const validate = new Function(`${m[1]}\nreturn validatePrEvidence;`)();

const ADR = 'training-app/叉车维修培训学员端跨端应用/docs/adr/0014-移动端全局搜索落点与投影口径.md';
const files = [
  {
    filename: 'training-app/叉车维修培训学员端跨端应用/pages/search/search.uvue',
    status: 'modified',
    patch: '@@ -10,3 +10,4 @@\n <view class="search-header">\n+  <text class="title">搜索</text>\n </view>\n',
  },
  {
    filename: ADR,
    status: 'modified',
    // 就是 #1028 踩到的那句（真实换行构成的多行 patch）
    patch: '@@ -86,3 +86,4 @@\n+> **二轮门证据**：③ CI run 全绿；**②** 免（未命中 MP-WEIXIN 面：无条件编译段）。\n',
  },
  {
    filename: 'training-app/叉车维修培训学员端跨端应用/utils/useBiometric.test.js',
    status: 'modified',
    patch: "@@ -53,3 +53,4 @@\n+    const app = () => branch('// #ifdef APP || MP-WEIXIN');\n",
  },
];
// 正文：四门证据里**刻意不写 ② 行**（② 未命中 ⇒ 按 ADR 可整行不写）
const body = `## 改了什么

搜索页样式。

## 验收证据

- ① Android 真机逐页截图对比 — 执行人：@DriftingLi · 日期：2026-09-15 · 复测对象：搜索页 · 结论（含产物）：逐页截图 docs/verification/search/1028/search-after.jpg
- ③ \`npm run test:unit\` 全绿 — 结论（含产物）：https://github.com/DriftingLi/FL/actions/runs/123456789
- ④ 本地编译门 — 执行人：@DriftingLi · 日期：2026-09-15 · 复测对象：整模块 · 结论（含产物）：KOTLIN_ALL_RESULT errors=0，日志 .ci-verify/kotlin-all.log
`;

const r = await validate({ files, body, author: 'zhengcookie' });
console.log('ok        =', r.ok);
console.log('errors    =', r.errors);
console.log('notes     =', r.notes);
const litGate2 = r.errors.some((e) => e.includes('②'));
console.log(
  litGate2
    ? '❌ 复现成功（BUG）：② 门被文档/测试文件里的「MP-WEIXIN」字样点亮 —— PR 判红'
    : '✅ ② 门未被点亮（期望行为）：文档/测试里提到该术语不再触发',
);
