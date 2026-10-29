// ============================================================================
// pr-evidence 校验器 —— 判据的**单一来源**（#1770）
//
// **A1 形态（2026-10-29，#2100 裁定）**：证据主键 = PR 正文里的**一行**。
//   行里写 `跑于 <sha7>` + 门自带的机读读数行（`MP_WEIXIN_RESULT` / `KOTLIN_ALL_RESULT` / `COMPILE_RESULT`）。
//   退役的四样（本文件不再有任何代码路径）：sha 绑定门评论、三候选集（PR / commit / 票评论）、
//   marker 与展示副本、①③④b 的字段签核。
//
// 为什么降到这里：整套取证仪式是从「多个陌生 agent 并发推同一仓、必须互相取证」长出来的；
// 单账号串行开发的形态不持有那个前提（#2099 实读：pr-evidence / mobile-gates 都不在 ruleset 必检，
// 这套门本来就是荣誉制）。留着它只是让人照着一页执行单写 A1、却撞上旧校验器判红。
//
// 为什么只剩这两道门：真正挡住 bug 的是「测试绿 + 编译过」（#2099 盘点）。
//   ② 是投放主渠道（微信小程序）的运行时抖动唯一机判；④ 是编译面。① 已在 ADR-0043 降为发版前一次、
//   ①b 已整体退役（#2103）；③ 与 ④b 已判纯税（两腿 / 打包链路都在 CI 与发版流程里，正文再抄一遍零信息）。
//
// 纯函数 + 单一注入点：输入 files / body / headSha / fetchCompare，输出
// { ok, errors, notes, runtime, needs4b, gates }。fetchCompare 是**注入的**取数回调
// （测试注桩、不打网络）：入参 { base, head }，返回 { status, mergeBaseSha, files, filesTruncated }；
// 取不到即返回 null（fail-closed）。因注入了异步回调，本函数返回 Promise，调用方必须 await。
//
// 消费者三处，读的都是这一份源码：
//   ① .github/workflows/pr-evidence.yml —— 不 checkout，用 repos.getContent 取本文件 → new Function 求值
//   ② .github/scripts/preflight-pr.mjs —— 直接 import（本地预检，与 CI 同一份判据）
//   ③ .github/scripts/pr-evidence-check.test.mjs —— 直接 import
// 注意 ① 的求值方式要求**本文件自包含**（不得 import 别的模块），改结构时别拆文件。
// ============================================================================

// 「验收证据行该长什么样」的**唯一真源**（#1920 起；A1 下缩到两门 + 行首形态 +
// 「跑于」锚点）。node 侧当对象字面量读，PowerShell 侧（证据生成器）抽 marker 之间的**同一段字节**
// `ConvertFrom-Json` —— 两个消费者读的字节完全相同。
// 块内规则（PowerShell 的 JSON 解析器不认别的东西）：**不加注释、不加尾逗号、正则只给 source 与 flags**。
// 加一道**走已有触发面**的门 = 往 gates 里加一条；加**新触发面**的门还要同时扩触发判定
// （`isRuntimeFile` / `hitsMpWeixinFace` 是硬编码清单，不是配置）——别以为加条 gates 就够了。
// ===== GATE_EVIDENCE_DESCRIPTOR BEGIN =====
export const GATE_EVIDENCE = {
  "heading": "## 验收证据",
  "listItem": {
    "source": "^\\s*[-*]\\s",
    "prefixStrip": "^\\s*[-*]\\s+",
    "prefix": "- "
  },
  "ranAt": {
    "label": "跑于",
    "source": "跑于\\s*`?([0-9a-fA-F]{7,40})`?"
  },
  "fieldSeparator": {
    "strip": "[·|]\\s*$",
    "display": " · "
  },
  "readingValue": {
    "source": "[A-Za-z_][\\w.-]*\\s*=\\s*\\S+",
    "hint": "token 后面必须跟着门真打出来的键值对（如 `errors=0` / `pages=3`），只写 token 或写 `...` 都是占位"
  },
  "coverage": {
    "field": {
      "source": "(?:^|\\s)coverage=([^\\s\\x60]+)"
    },
    "value": {
      "source": "^(?:changed|smoke):\\d+$|^none$"
    },
    "hint": "`coverage=` 只认 `changed:<N>` / `smoke:<N>` / `none` 三值（门在 `-Pages auto` 下产出，见 ADR-0008 ② 段）"
  },
  "gates": [
    {
      "key": "②",
      "label": "微信开发者工具无报错",
      "requires": "mpweixin",
      "tokens": ["MP_WEIXIN_RESULT"],
      "tokenHint": "门自带的 `MP_WEIXIN_RESULT` 读数行"
    },
    {
      "key": "④",
      "label": "本地编译门",
      "requires": "runtime",
      "tokens": ["KOTLIN_ALL_RESULT", "COMPILE_RESULT"],
      "tokenHint": "门自带的 `KOTLIN_ALL_RESULT`（④c）或 `COMPILE_RESULT`（④a）读数行"
    }
  ]
};
// ===== GATE_EVIDENCE_DESCRIPTOR END =====

const toRe = (spec) => new RegExp(spec.source, spec.flags || '');

// 报错文案里给一行可照抄的形状（判据与提示同源，别在两处各写一份）。
const EVIDENCE_LINE_HINT = '跑于 `<sha7>` · <门读数行原样>（图可选，附在行尾）';

// 「这张改动单算不算碰了交付形态」——运行时面与 ② 触发面共用同一判据，
// 免得「门被点亮的理由」和「门证据的失效条件」两套口径各走一路。
const RUNTIME_JSON = ['manifest.json', 'pages.json', 'platformConfig.json'];
export function isRuntimeFile(p) {
  if (p.endsWith('.uvue') || p.endsWith('.uts')) return true;
  if (!p.startsWith('training-app/')) return false;
  return RUNTIME_JSON.includes(p.split('/').pop());
}

// ④b 触发条件（ADR-0008，2026-09-11 修订）：按「交付形态」触发 —— 打包链路（AAPT2 资源合并、
// AndroidManifest 合并、R8/aar 依赖解析、证书签名）只有真打包才覆盖，本地编译门不覆盖。
// A1 下它**不判红**（正文不再写 ④b 行），只作为发版前提醒出现在 notes 里 —— 判据删了，
// 「这件事存在」的事实不该跟着消失。
function touchesPackaging(files) {
  // 这份清单与上面的 `RUNTIME_JSON` **当前恰好同集**，但概念不同：一个是「算不算运行时面」（触发 ④），
  // 一个是「算不算碰了交付形态」（触发 ④b 的 notes）。别因为看着一样就合并 —— 将来任一侧要分叉
  // （例如打包面单独加某份资源），合并的写法会把两件事绑死。
  const PACKAGE_FILES = ['manifest.json', 'pages.json', 'platformConfig.json'];
  return files.some((f) => {
    const p = f.filename;
    if (p.startsWith('training-app/') && PACKAGE_FILES.includes(p.split('/').pop())) return true;
    if (/uni_modules\/[^/]+\/utssdk\/app-android\//.test(p)) return true;
    if (/\.aar$/.test(p)) return true;
    if (/(^|\/)libs\/[^/]+\.jar$/.test(p)) return true;
    return false;
  });
}

// ② 触发条件（ADR-0008，2026-09-15 修订 #1030）：命中 MP-WEIXIN **面**，不是「文本里出现了这个词」。
//   ① 文件清单：training-app 下的 manifest.json / platformConfig.json；
//   ② 运行时面文件的 diff 里**增删了一条真正的条件编译指令行**（去掉 +/- 后行首、允许注释/引号包装，
//      是 `#ifdef|#ifndef|#if`，且表达式里含 MP-WEIXIN）。表达式**之后**的部分不判 ——
//      `// #ifdef APP-PLUS // 与 MP-WEIXIN 分支不同` 这类「真指令行 + 同行散文提及」不算命中（#1028 实证）。
// 已知边界（写实）：第 ② 条读 `patch`，GitHub 对超大 diff / 二进制不给 `patch` ⇒ 该路径 fail-open。
const MP_WEIXIN_FILES = ['manifest.json', 'platformConfig.json'];
const MP_WEIXIN_DIRECTIVE = /^\s*(?:\/{2,}|\/\*|<!--|["'])?\s*#\s*(?:ifdef|ifndef|if)\s+[A-Za-z0-9_\s|&()!-]*\bMP-WEIXIN\b/;
const hasMpWeixinDirective = (patch) =>
  String(patch || '')
    .split('\n')
    .filter((line) => /^[+-]/.test(line) && !/^(?:\+\+\+|---)/.test(line))
    .map((line) => line.slice(1))
    .some((content) => MP_WEIXIN_DIRECTIVE.test(content));
const hitsMpWeixinFace = (f) => {
  const p = f.filename;
  if (p.startsWith('training-app/') && MP_WEIXIN_FILES.includes(p.split('/').pop())) return true;
  return isRuntimeFile(p) && hasMpWeixinDirective(f.patch);
};

// sha 前缀比较：A1 正文只写 7 位，compare 回来的是 40 位 —— 严格 `!==` 会让祖先口径**永假**。
const shaMatches = (a, b) => Boolean(a) && Boolean(b) && (a.startsWith(b) || b.startsWith(a));

export async function validatePrEvidence({
  files, body, headSha = '', fetchCompare = null,
}) {
  // 错误带上门归属：例外通道要按门豁免，字符串数组做不到「只清 ②、④ 照红」。
  // 返回给调用方前统一渲染成字符串（三个消费者的接口不变）。
  const errors = [];
  const err = (gate, text) => errors.push({ gate, text });
  const notes = [];

  const runtime = files.filter((f) => isRuntimeFile(f.filename)).map((f) => f.filename);
  const needs2Why = files.filter(hitsMpWeixinFace).map((f) => f.filename);
  const needs2 = needs2Why.length > 0;
  const needs4b = touchesPackaging(files);
  const headLc = String(headSha || '').toLowerCase();

  const gates = {
    '②': { required: needs2, bound: false, commit: '', reason: '', why: needs2 ? needs2Why.slice(0, 5) : [] },
    '④': { required: runtime.length > 0, bound: false, commit: '', reason: '', why: runtime.slice(0, 5) },
  };

  // 未命中运行时面 ⇒ 整段免（分级判据，ADR-0008）。空正文也放行，这一条与改动前同义。
  if (runtime.length === 0) {
    notes.push(`未命中运行时面（改动 ${files.length} 个文件），按 ADR-0008 免人工门与 ④ 编译门。`);
    return { ok: true, errors, notes, runtime, needs4b, gates };
  }

  const clean = String(body || '').replace(/<!--[\s\S]*?-->/g, '');
  // 段名从描述符取：写死正则会让「改描述符」只动生成侧、校验侧静默不跟（第二套口径）。
  const escRe = (s) => String(s).replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const head = clean.match(new RegExp(`^${escRe(GATE_EVIDENCE.heading)}\\s*$`, 'm'));
  if (!head) {
    err('*', `缺 \`${GATE_EVIDENCE.heading}\` 段（段名固定，见 .github/PULL_REQUEST_TEMPLATE.md）。`);
    return { ok: false, errors: errors.map((e) => e.text), notes, runtime, needs4b, gates };
  }
  const rest = clean.slice(head.index + head[0].length);
  const next = rest.search(/^##\s/m);
  const section = next === -1 ? rest : rest.slice(0, next);

  // 例外通道：显式风险接受，**必须点名哪道门**（#2100 决策 4；2026-10-29 review 收窄）。
  //
  // 旧实现是 `/已接受未验证风险/.test(clean)` —— 全文任意位置命中即清空全部 errors，
  // 于是「本 PR 不适用『已接受未验证风险』通道」这种**否认句**也会命中，五个字空正文也能过。
  // 收窄的三条（都不靠加填字，全靠现有结构）：
  //   ① 只在 `## 验收证据` 段内生效 ⇒ 背景叙述 / 引用的旧例子不会误触发；
  //   ② 声明必须点名门号（`- 例外：已接受未验证风险 ②`）⇒ 写得含糊就清不掉任何东西，自动退回红；
  //   ③ 只清空**被点名门**的 errors ⇒ ④ 有缺项时写 ② 的豁免救不了它。
  const WAIVE_HINT = '- 例外：已接受未验证风险 <门号>';
  const GATE_KEY_RE = new RegExp(`${GATE_EVIDENCE.gates.map((g) => escRe(g.key)).join('|')}(?![a-z])`, 'g');
  const waived = new Set();
  for (const line of section.split('\n')) {
    if (!/已接受未验证风险/.test(line)) continue;
    for (const k of line.match(GATE_KEY_RE) || []) waived.add(k);
  }

  const LIST_ITEM = toRe(GATE_EVIDENCE.listItem);
  const LIST_ITEM_PREFIX = new RegExp(GATE_EVIDENCE.listItem.prefixStrip);
  const RAN_AT = toRe(GATE_EVIDENCE.ranAt);
  const READING_VALUE = toRe(GATE_EVIDENCE.readingValue);
  const COVERAGE_FIELD = toRe(GATE_EVIDENCE.coverage.field);
  const COVERAGE_VALUE = toRe(GATE_EVIDENCE.coverage.value);

  // ④ 全家（④ / ④a / ④b / ④c）都含「④」⇒ 按行首编号匹配，④a / ④b 行不能顶替 ④ 行。
  const matchesGate = (line, gate) => {
    const core = line.replace(LIST_ITEM_PREFIX, '');
    if (!core.startsWith(gate.key)) return false;
    if (gate.key === '④') return !/[a-z]/.test(core.charAt(1));
    return true;
  };

  // 绑定判定（ADR-0008，2026-09-12 放宽）：sha7 等于 head，或它是 head 的祖先**且**其间运行时面未变。
  // **fail-closed**：compare 取不到（未知 sha / 异常 / 截断）一律判不满足，不得因 API 抖动放行。
  const compareCache = new Map();
  const compareOnce = async (base) => {
    if (typeof fetchCompare !== 'function') return null;
    if (compareCache.has(base)) return compareCache.get(base);
    let res = null;
    try {
      res = await fetchCompare({ base, head: headLc });
    } catch {
      res = null;
    }
    compareCache.set(base, res || null);
    return res || null;
  };
  const judgeBinding = async (sha) => {
    if (!headLc) return { ok: false, reason: '本次事件没给出 head sha（fail-closed）' };
    if (shaMatches(sha, headLc)) return { ok: true, reason: `等于 head` };
    const cmp = await compareOnce(sha);
    if (!cmp) {
      return { ok: false, reason: `取不到 compare(\`${sha}\`…head) 结果（未知 sha / API 异常 / 超时）⇒ fail-closed 不满足，重跑一次并写当前 head 的前 7 位` };
    }
    const status = String(cmp.status || '');
    const mergeBase = String(cmp.mergeBaseSha || '').toLowerCase();
    if ((status !== 'ahead' && status !== 'identical') || !shaMatches(sha, mergeBase)) {
      return { ok: false, reason: `它不是当前 head 的祖先（compare status=${status || '未知'}）` };
    }
    if (cmp.filesTruncated) {
      return { ok: false, reason: '与 head 之间的 diff 过大被 GitHub 截断（files ≥300）⇒ 判不出运行时面是否变过，fail-closed 不满足' };
    }
    const changed = Array.isArray(cmp.files) ? cmp.files.filter((f) => isRuntimeFile(String(f || ''))) : [];
    if (changed.length > 0) {
      return { ok: false, reason: `之后运行时面文件已变更（${changed.slice(0, 3).join('、')}${changed.length > 3 ? ' 等' : ''}）⇒ 需重跑并把 \`跑于\` 改成当前 head 的前 7 位` };
    }
    return { ok: true, reason: `是 head 的祖先且其间运行时面（\`.uvue\` / \`.uts\` / 三份 json）未变，当前 head=${headLc.slice(0, 7)}` };
  };

  const lines = section.split('\n').filter((l) => LIST_ITEM.test(l));
  for (const gate of GATE_EVIDENCE.gates) {
    const state = gates[gate.key];
    if (!state.required) {
      notes.push(`${gate.key} ${gate.label}：未触发（${gate.requires === 'mpweixin' ? '未命中 MP-WEIXIN 面，ADR-0008' : '未命中运行时面'}）⇒ 免。`);
      continue;
    }
    const where = `${gate.key} ${gate.label}`;
    const line = lines.find((l) => matchesGate(l, gate));
    if (!line) {
      err(gate.key, `${where}：缺该行。按模板写一行 \`- ${gate.key} ${EVIDENCE_LINE_HINT}\`。`);
      continue;
    }
    const token = gate.tokens.find((t) => line.includes(t));
    if (!token) {
      err(gate.key, `${where}：该行没有${gate.tokenHint} —— 门自带的机读行原样贴进来即可，不要改写它。`);
    } else {
      // token 只是「门名」，不能单独算读数。模板曾预置 `MP_WEIXIN_RESULT ...`，只判 includes
      // 就等于「照抄模板即绿」（2026-10-29 review 的 P0）。token 之后必须跟着门真打出来的键值对。
      const after = line.slice(line.indexOf(token) + token.length);
      if (!READING_VALUE.test(after)) {
        err(gate.key, `${where}：${gate.tokenHint} 后面没有实际读数 —— ${GATE_EVIDENCE.readingValue.hint}。`);
      }
    }
    // `coverage=` 形态校验（#2101/#2102）。**只校「出现即合法」，不校「必须出现」**：
    // #2102 原文写「校存在性」，但门只在 `-Pages auto` 下产出该字段 —— `-Pages <csv>`（定向单验）与
    // `-Pages none`（旧导航组）是合法跑法且没有「改动页覆盖」语义，强制出现会把它们判成假红。
    // 这与本票「删格式税」的立场一致：能判红的只剩「写了但写错」，不是「少写了一个自报字段」。
    if (gate.key === '②') {
      const cov = line.match(COVERAGE_FIELD);
      if (cov) {
        // 读数通常被反引号包着（`… coverage=changed:2`），截到反引号前；再兜一刀剥尾反引号
        const v = cov[1].replace(/`+$/, '');
        if (!COVERAGE_VALUE.test(v)) {
          err(gate.key, `${where}：\`coverage=${v}\` 不是合法形态 —— ${GATE_EVIDENCE.coverage.hint}。`);
        }
      }
    }
    const m = line.match(RAN_AT);
    if (!m) {
      err(gate.key, `${where}：缺 \`跑于 <sha7>\`（门是在哪个 commit 上跑的，写它的前 7 位）。`);
      continue;
    }
    const sha = m[1].toLowerCase();
    const v = await judgeBinding(sha);
    if (v.ok) {
      state.bound = true;
      state.commit = sha.slice(0, 7);
      state.reason = v.reason;
      // 祖先绑定要明示（不静默放宽）：证据是在**旧 commit** 上产出的，
      // 写成「已绑」而不说是哪个 sha，读起来像「证据是在当前 head 上跑的」——这在只校结构的门里属往更可信方向失真。
      if (v.reason !== '等于 head') notes.push(`${where}：绑定 ${sha.slice(0, 7)} —— ${v.reason}。`);
    } else {
      state.reason = v.reason;
      err(gate.key, `${where}：\`跑于 ${sha.slice(0, 7)}\` 绑不上当前 head —— ${v.reason}。`);
    }
  }

  if (needs2) {
    notes.push(`② 触发面命中（MP-WEIXIN 面，ADR-0043 起为常态主门、无低风险减免）：${needs2Why.slice(0, 5).join('、')}${needs2Why.length > 5 ? ' 等' : ''}。`);
  }
  if (needs4b) {
    notes.push('④b 打包面命中（三份 json / utssdk 原生面 / *.aar / libs/*.jar）：A1 下不判红、正文不写 ④b 行，但**发版前必须跑** release 云打包（ADR-0008）。');
  }

  // 例外通道只在「点名门覆盖了**全部**缺项」时放行；否则照红，并说明差在哪。
  if (errors.length > 0 && waived.size > 0) {
    const absorbed = errors.filter((e) => waived.has(e.gate));
    const blocking = errors.filter((e) => !waived.has(e.gate));
    // 点名了但该门其实没缺项 ⇒ 记 note（防模板残留，也防「我写了豁免 = 全都免了」的误读）
    const idle = [...waived].filter((k) => !errors.some((e) => e.gate === k));
    if (blocking.length === 0) {
      return {
        ok: true,
        errors: [],
        notes: [
          `⚠️ 走「已接受未验证风险」例外通道，豁免门：${[...waived].join(' / ')}。`,
          ...absorbed.map((e) => `⚠️ 已豁免：${e.text}`),
          ...idle.map((k) => `⚠️ 例外声明点了 ${k}，但那道门本次没有缺项（多写了一句，不影响结论）。`),
          '⚠️ 例外通道下必须由人执行合并（AGENTS.md：agent 不得自行合并触及运行时面的 PR）。',
          ...notes,
        ],
        runtime,
        needs4b,
        gates,
      };
    }
    notes.push(`例外声明只覆盖了 ${[...waived].join(' / ')}，仍有未被覆盖的缺项（见下）—— 要豁免就点名对应门号（\`${WAIVE_HINT}\`）。`);
  }
  if (errors.length === 0) {
    if (waived.size > 0) notes.push(`例外声明写了 ${[...waived].join(' / ')}，但本次没有缺项 —— 可以删掉那句话。`);
    else notes.push('证据结构完整（pr-evidence 只校结构与 sha 绑定，真伪由维护者判定）。');
  }
  return {
    ok: errors.length === 0,
    errors: errors.map((e) => e.text),
    notes,
    runtime,
    needs4b,
    gates,
  };
}
