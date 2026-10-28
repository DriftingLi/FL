import { execFileSync } from 'node:child_process';
import { existsSync, readFileSync, readdirSync } from 'node:fs';
import { join, sep } from 'node:path';
// ============================================================================
// 票面引用完整性 lint —— 判据的**单一来源**（#2053）
//
// 为什么要有它：「票面只引已合库文档」是散文规（docs/agents/issue-tracker.md 2026-10-24 立），
// 立规当天开的 #1861 三天后又命中同型漂移（写死「新移动端 ADR 0044」三日内被占、引用的
// 调研文档从未入库）。失效模式是机械的 ⇒ 做成确定性检查，不再加散文规则。
//
// 形态同 gate-validator.mjs：纯函数判据，输入票/PR 正文文本 + 注入的仓内事实，
// 输出 { ok, reds, notes }；不打网络、不碰 fs（事实由调用方注入，测试注桩）。
//   消费者：① preflight-pr.mjs 对 PR 正文跑它（与证据段判据同层，不新增门）；
//           ② to-tickets / triage 落票最后一步预检（本文件 CLI 形态，报红项、不自动改）。
// 判据三红线：反引号内白名单前缀路径必须存在于仓；ADR 号位引用必须有对应文件在库
// （「编号 = 执行时 docs/adr 最大号 + 1」这类不写死具体号位的合规写法天然不在捕获面）；
// `#<N>` 必须能在 DriftingLi/FL 解析。**小节号引用（§6.3 类）显式不判**（机器可判性低）。
// ============================================================================

// 反引号内按路径判的白名单前缀 = 仓顶层目录全集（票面圈定的 docs/design、docs/adr、scripts 之外，
// 评审轴实证「.github/scripts 与移动端承载面同样高频」；裸 lib/、utils/ 这类非顶层前缀属示意形态，不判）。
const PATH_PREFIXES = ['docs/', 'scripts/', '.github/', 'training-app/', 'deploy/', 'frontend/', 'backend/'];
// 占位/示意形态：含 <>…*?U+2026 或连续两个以上 N（`docs/adr/NNNN-中文标题.md` 这类模板写法）⇒ 不判。
const PLACEHOLDER = /[<>*?\u2026]|N{2,}/;

// ADR 号位引用：ADR-0043 / ADR 0043 / 「移动端 ADR 00NN」——号位恰好 4 位数字才进捕获面。
const ADR_REF = /ADR[-\s]*(\d{4})/g;
// 「规划新建」语头**紧贴** ADR 词头（中间只隔 移动端/引号/空白 这类连接符）⇒ 指尚未存在的新 ADR（#1861 形态）。
// 收紧判据（评审轴实证）：松窗会把「销账与新登 #25、ADR-0012」这类合法引用误判规划——连接符白名单外的字一律断链。
const ADR_NEW_MARK = /(?:新|待建|拟)(?:[\s的「『“"（(【]|移动端){0,8}$/;
// 两套编号互不相关（根仓 ADR-0001+ / 移动端 0001+）：词头前 12 字符内出现「移动端」⇒ 判移动端号位。
const ADR_MOBILE_MARK = /移动端/;

// 票/PR 号：#1845、范围 #1845-#1848、Closes #2053、[#1838](链接)。GitHub 上 issue 与 PR 共用号空间。
// hex 色值（#0d9458/#41ba41/#f43146）不误捕：数字串后紧跟字母时 \b 不成立；前置字符排除词字符与 /（URL 尾号）。
const ISSUE_REF = /(?:^|[^\w/])#(\d{1,6})\b/g;

/** 正文预处理：剥 HTML 注释与围栏代码块（历史票号塞满模板注释，#1770 同源教训）。 */
function cleanBody(body) {
  return String(body || '')
    .replace(/<!--[\s\S]*?-->/g, '')
    .replace(/\`{3}[\s\S]*?\`{3}/g, '');
}

/**
 * ①路径判据：反引号内、白名单前缀、非占位；`:35`/`:24-30` 行号后缀先剥再判；
 * 目录写法（docs/adr/）去尾斜杠判目录。同一文件重复引用只报一次（seen）。
 */
function lintPaths(text, deps) {
  const reds = [];
  const seen = new Set();
  for (const m of text.matchAll(/\`([^\`\n]+)\`/g)) {
    const token = m[1];
    if (/\s/.test(token) || !PATH_PREFIXES.some((p) => token.startsWith(p))) continue;
    const stripped = token.replace(/:\d+(?:-\d+)?$/, '').replace(/\/+$/, '');
    if (!stripped || PLACEHOLDER.test(stripped) || seen.has(stripped)) continue;
    seen.add(stripped);
    if (!deps.existsPath(stripped)) {
      reds.push({ category: 'missing-file', ref: stripped, message: '反引号路径不存在于仓：' + stripped });
    }
  }
  return reds;
}

/**
 * ②ADR 号位判据：引用必须有对应文件在库；「新 ADR 写死号」且号位已被占 ⇒ 红（#1861 实证形态）。
 * 裸 ADR-00NN 判根仓号位；前窗含「移动端」判移动端号位——同号不互相豁免（两套编号互不相关）。
 */
function lintAdr(text, deps) {
  const reds = [];
  const seen = new Set();
  for (const m of text.matchAll(ADR_REF)) {
    const num = m[1];
    const before = text.slice(Math.max(0, m.index - 12), m.index);
    const mobile = ADR_MOBILE_MARK.test(before);
    const isNew = ADR_NEW_MARK.test(before);
    const scope = mobile ? 'mobile' : 'root';
    const key = scope + ':' + num;
    if (seen.has(key)) continue;
    seen.add(key);
    const after = text.slice(m.index + m[0].length, m.index + m[0].length + 12);
    // 陈述形态豁免：尾窗含「已被占用/已占/已在库/复盘…」⇒ 这是对既成事实的叙述（血账票面就长这样），不是把新号写死。
    const narrated = isNew && /(已(被)?占|占用|已在库|已合库|已落地|退场|退役|复盘|血账)/.test(after);
    const exists = deps.adrNumbers(scope).has(num);
    const scopeLabel = mobile ? '移动端' : '根仓';
    if (isNew && exists && !narrated) {
      reds.push({ category: 'adr', ref: num, message: '新 ' + scopeLabel + ' ADR ' + num + ' 写死号已被占用（合规写法：编号 = 执行时 docs/adr 最大号 + 1）' });
    } else if (!exists) {
reds.push({ category: 'adr', ref: num, message: scopeLabel + ' ADR 号位无对应文件在库：' + num });
    }
  }
  return reds;
}

/**
 * ③票号可解析判据：每个 `#<N>` 必须能在 DriftingLi/FL 解析。
 * resolveNumber 为注入回调（生产走 gh api、测试注桩不打网络）；唯一号去重、升序报红。
 */
async function lintNumbers(text, deps) {
  const nums = new Set();
  for (const m of text.matchAll(ISSUE_REF)) nums.add(m[1]);
  const reds = [];
  for (const n of [...nums].sort((a, b) => Number(a) - Number(b))) {
    if (!(await deps.resolveNumber(Number(n)))) {
      reds.push({ category: 'issue-number', ref: n, message: 'DriftingLi/FL 解析不到的票/PR 号：#' + n });
    }
  }
  return reds;
}

/**
 * 票/PR 正文引用完整性 lint（纯函数）。
 * deps 注入的仓内事实：{ existsPath(rel)->bool, adrNumbers(scope)->Set<string>, resolveNumber(n)->Promise<bool> }
 * 某项事实缺省 ⇒ 对应判据跳过并在 notes 说明（fail-visible，不假装判过）。
 */
export async function lintIssueReferences(body, deps = {}) {
  const text = cleanBody(body);
  const notes = [];
  const reds = [];
  if (deps.existsPath) reds.push(...lintPaths(text, deps));
  else notes.push('未注入 existsPath，跳过路径存在性判据');
  if (deps.adrNumbers) reds.push(...lintAdr(text, deps));
  else notes.push('未注入 adrNumbers，跳过 ADR 号位判据');
  if (deps.resolveNumber) reds.push(...(await lintNumbers(text, deps)));
  else notes.push('未注入 resolveNumber，跳过票号可解析判据');
  return { ok: reds.length === 0, reds, notes };
}

// —— 默认事实构建器（生产接线用；判据函数本身不碰 fs/网络）————————————————

// 两套 ADR 目录（编号互不相关）：根仓 ADR-0001+ / 移动端 0001+，引用侧靠「移动端」前缀分流。
const MOBILE_ROOT = 'training-app/叉车维修培训学员端跨端应用';

const ADR_DIRS = {
  root: 'docs/adr',
  mobile: 'training-app/叉车维修培训学员端跨端应用/docs/adr',
};

/**
 * fs 事实：existsPath **双承载面探**——同一相对路径在仓根或移动端根任一处存在即算存在
 * （评审轴实证：移动端票高频写移动端根相对路径，单根判会把在库文件假红一片）。
 * adrNumbers 扫目录取号位（缓存；目录缺失 ⇒ 空集，fail-closed）。
 */
export function makeFsFacts(repoRoot) {
  const adrCache = new Map();
  function adrNumbers(scope) {
    if (adrCache.has(scope)) return adrCache.get(scope);
    let set = new Set();
    try {
      set = new Set(
        readdirSync(join(repoRoot, ADR_DIRS[scope] || ADR_DIRS.root))
          .map((f) => (/^(?:ADR-)?(\d{4})-/.exec(f) || [])[1])
          .filter(Boolean),
      );
    } catch {
      set = new Set();
    }
    adrCache.set(scope, set);
    return set;
  }
  return {
    existsPath: (rel) => {
      const tail = String(rel).replace(/^\.\//, '').split('/').join(sep);
      return existsSync(join(repoRoot, tail)) || existsSync(join(repoRoot, MOBILE_ROOT, tail));
    },
    adrNumbers,
  };
}

/**
 * gh 票号解析器：`gh api repos/<repo>/issues/<N>` 共用号空间覆盖 issue 与 PR。
 * 404 ⇒ false；其他失败（网络/权限）抛错——调用方按环境错误处理，不混进「票号不存在」。
 * run 仅为测试注桩；同号缓存只查一次（正文里同一个 #1861 常出现多次）。
 */
export function makeGhNumberResolver(repo, { run } = {}) {
  const exec = run || ((cmd, args) => execFileSync(cmd, args, { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] }));
  const cache = new Map();
  return async (n) => {
    if (cache.has(n)) return cache.get(n);
    let ok;
    try {
      ok = Boolean(String(exec('gh', ['api', 'repos/' + repo + '/issues/' + n, '--jq', '.number'])).trim());
    } catch (e) {
      const msg = String((e && e.stderr) || (e && e.message) || e);
      if (/(404|not found)/i.test(msg)) ok = false;
      else throw new Error('解析 #' + n + ' 失败（gh api）：' + msg.split('\n')[0]);
    }
    cache.set(n, ok);
    return ok;
  };
}

// —— CLI：to-tickets / triage 落票最后一步预检（报红项、不自动改）——————————————

const CLI_USAGE = [
  '用法: node .github/scripts/lib/lint-issue-references.mjs --body-file <正文文件> [选项]',
  '',
  '  --body-file <path>  待落库的票/PR 正文（必填）',
  '  --repo-root <dir>   仓根（默认当前目录）',
  '  --repo <slug>       远端仓名（默认 gh repo view 取）',
  '  --no-numbers        跳过 #N 票号解析判据（无 gh 时用；notes 会如实标注跳过）',
  '  --json              输出 JSON（红项机器可读）',
  '  -h, --help          本帮助',
  '',
  '退出码：0 = 绿；1 = 有红项（只报不改）；2 = 用法/环境错误。',
].join('\n');

class CliUsageError extends Error {}

function cliNeed(argv, i, flag) {
  const v = argv[i];
  if (v === undefined || v.startsWith('--')) throw new CliUsageError(flag + ' 缺参数');
  return v;
}

export function parseCliArgs(argv) {
  const out = { bodyFile: '', repoRoot: process.cwd(), repo: '', noNumbers: false, json: false, help: false };
  for (let i = 0; i < argv.length; i += 1) {
    const a = argv[i];
    if (a === '-h' || a === '--help') out.help = true;
    else if (a === '--json') out.json = true;
    else if (a === '--no-numbers') out.noNumbers = true;
    else if (a === '--body-file') out.bodyFile = cliNeed(argv, ++i, a);
    else if (a === '--repo-root') out.repoRoot = cliNeed(argv, ++i, a);
    else if (a === '--repo') out.repo = cliNeed(argv, ++i, a);
    else throw new CliUsageError('未知参数：' + a);
  }
  if (!out.help && !out.bodyFile) throw new CliUsageError('--body-file 必填（落票预检的对象是写好的正文文件）');
  return out;
}

async function cliMain(argv) {
  const args = parseCliArgs(argv);
  if (args.help) {
    console.log(CLI_USAGE);
    return 0;
  }
  const body = readFileSync(args.bodyFile, 'utf8');
  const deps = { ...makeFsFacts(args.repoRoot) };
  if (!args.noNumbers) {
    let repo = args.repo;
    if (!repo) {
      const view = JSON.parse(execFileSync('gh', ['repo', 'view', '--json', 'nameWithOwner'], { encoding: 'utf8' }));
      repo = view.nameWithOwner;
    }
    deps.resolveNumber = makeGhNumberResolver(repo);
  }
  const r = await lintIssueReferences(body, deps);
  if (args.json) {
    console.log(JSON.stringify(r, null, 2));
  } else {
    console.log('引用完整性预检（#2053）：' + args.bodyFile);
    for (const x of r.reds) console.log('  ❌ [' + x.category + '] ' + x.message);
    for (const n of r.notes) console.log('  · ' + n);
    console.log(r.ok ? '结论：绿 —— 可以落票。' : '结论：红 —— ' + r.reds.length + ' 个漂移项，修正正文后重跑（本检查不自动改）。');
  }
  return r.ok ? 0 : 1;
}

const invokedDirectly = Boolean(process.argv[1]) && process.argv[1].replace(/\\/g, '/').endsWith('.github/scripts/lib/lint-issue-references.mjs');
if (invokedDirectly) {
  cliMain(process.argv.slice(2)).then(
    (code) => process.exit(code),
    (err) => {
      if (err instanceof CliUsageError) {
        process.stderr.write(err.message + '\n\n' + CLI_USAGE + '\n');
        process.exit(2);
      }
      process.stderr.write('预检出错：' + (err && err.message ? err.message : err) + '\n');
      process.exit(2);
    },
  );
}

