// 判据收窄的定点对照：同一批判据样本，旧正则 vs 新正则。
const RE_OLD = /^\s*(?:\/{2,}|\/\*|<!--|["'])?\s*#\s*(?:ifdef|ifndef|if)\b[^\n]*\bMP-WEIXIN\b/;
const RE_NEW = /^\s*(?:\/{2,}|\/\*|<!--|["'])?\s*#\s*(?:ifdef|ifndef|if)\s+[A-Za-z0-9_\s|&()!-]*\bMP-WEIXIN\b/;
const cases = [
  ['// #ifdef MP-WEIXIN', true],
  ['// #ifndef MP-WEIXIN', true],
  ['        // #ifdef APP || MP-WEIXIN', true],
  ['// #ifndef H5 || APP || MP-WEIXIN', true],
  ['            <!-- #ifdef MP-WEIXIN -->', true],
  ['  /* #ifdef MP-WEIXIN */', true],
  ['  "#ifdef MP-WEIXIN": {}', true],
  ['#ifdef MP-WEIXIN', true],
  ['// #ifdef APP-PLUS // 与 MP-WEIXIN 分支不同', false],
  ['// 本模块不涉及 MP-WEIXIN 面，无需条件编译', false],
  ['// 见下方 #ifdef APP 与 MP-WEIXIN 分支', false],
];
let bad = 0;
for (const [line, want] of cases) {
  const o = RE_OLD.test(line);
  const n = RE_NEW.test(line);
  if (n !== want) bad += 1;
  console.log(`${n === want ? 'ok  ' : 'BAD '} 期望=${String(want).padEnd(5)} 旧=${String(o).padEnd(5)} 新=${String(n).padEnd(5)} ${line}`);
}
console.log(bad === 0 ? '\n✅ 新正则全部符合期望' : `\n❌ ${bad} 条不符`);
