// 契约消费面覆盖锁的判定面自测（ADR-0056 §11 / issue #1100）。
//
// 这一层的存在理由与其余守卫自测一致：守卫坏了 = 规则静默失效 = 全绿假象。本票的假绿面尤其窄：
//   - 泛型实参（get<PagedResult<T>>）少跳一层，调用点整条读不到（实测初版原型就漏了 5/8 个端点）；
//   - 路径由变量给出时不报错，守卫就成了「写个变量绕过去」的摆设；
//   - ALLOWLIST 条目挂在已登记的端点上（销账不删条目）→ 存量债永远不清。
// 所以这里用**合成源码 + 合成 diff** 端到端驱动 runGuard，正样本必须红、负样本必须绿、
// 判据坏了必须非零退出，并逐条钉住 ALLOWLIST 的卫生（不许有死条目）。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { runGuard } from './lib/guard.mjs'
import {
  ALLOWLIST,
  CONSUMER_DIR,
  DECLARATION_FILE,
  GUARD_SPEC,
  isAnchoredPattern,
  isGuardedPath,
  matchDeclared,
  parseDeclaredEndpoints,
  scanSource
} from './check-api-consumers.mjs'

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const BT = String.fromCharCode(96)
const D = String.fromCharCode(36)
/** 造一个模板字符串字面量（本测试自身不在守卫面，但源码里要出现反引号）。 */
const tpl = (body) => BT + body + BT
/** 造一个插值（模板字符串里的 ${expr}）。 */
const hole = (expr) => D + '{' + expr + '}'

const API_FILE = 'frontend/src/api/demo.ts'

/** 造一段合成 diff（-U0 hunk：从 startLine 起新增 lines）。 */
function synthDiff(file, startLine, lines) {
  return [
    'diff --git a/' + file + ' b/' + file,
    '--- a/' + file,
    '+++ b/' + file,
    '@@ -1,0 +' + startLine + ',' + lines.length + ' @@',
    ...lines.map((l) => '+' + l)
  ].join('\n')
}

/** 端到端驱动一次 --diff：合成 diff + 假源码；overrides 可注入失败面。 */
function probeDiff(diff, sources = {}, overrides = {}) {
  const out = []
  const err = []
  const code = runGuard(GUARD_SPEC, {
    argv: ['--diff', 'origin/master'],
    root: '/wave11-consumers-probe',
    stdout: (line) => out.push(line),
    stderr: (line) => err.push(line),
    resolveBase: () => true,
    readDiff: () => diff,
    readSource: (file) => {
      if (Object.prototype.hasOwnProperty.call(sources, file)) return sources[file]
      const e = new Error('ENOENT: ' + file)
      e.code = 'ENOENT'
      throw e
    },
    ...overrides
  })
  return { code, stdout: out.join('\n'), stderr: err.join('\n') }
}

// ===== 判定面：正样本（必须报违规）=====

const UNDECLARED = '/admin/never-declared-1100'

const RED_CASES = [
  {
    name: '消费未登记端点（普通字符串实参）',
    source: "await unwrappedRequest.get('" + UNDECLARED + "')",
    expect: /未落在 backend\/internal\/apitypes\/domains\.go 的域端点集内/
  },
  {
    name: '消费未登记端点（嵌套泛型实参，历史上漏读的形态）',
    source: 'await unwrappedRequest.get<PagedResult<Row>>(' + tpl(UNDECLARED) + ', { params })',
    expect: /未落在 /,
    expectPattern: new RegExp('^' + UNDECLARED + '$')
  },
  {
    name: '消费未登记端点（模板插值 + 拼接混用）',
    source: "await unwrappedRequest.post(" + tpl('/admin/jobs/' + hole('id') + '/never-1100') + ', body)',
    expect: /未落在 /,
    expectPattern: /^\/admin\/jobs\/\{\}\/never-1100$/
  },
  {
    name: '路径由变量给出（拒绝静态判定，不放行）',
    source: 'await unwrappedRequest.get(dictBase, { params })',
    expect: /路径由变量\/表达式拼装，无法静态判定/
  },
  {
    name: '全部路径段都是插值（无法静态判定）',
    source: 'await unwrappedRequest.put(' + tpl(hole('adminBase') + '/' + hole('id')) + ', payload)',
    expect: /路径由变量\/表达式拼装，无法静态判定/
  },
  {
    // 「实参不是字面量」与上一条同因不同形（store.get(key) 一类）；这样从严是防「写个变量绕过去」。
    name: '实参是变量（路径完全不可静态判定）',
    source: 'const cached = store.get(key)',
    expect: /路径由变量\/表达式拼装，无法静态判定/
  },
  {
    // 单段字面量：没有可锚定的首段，尾段比较只能靠声明侧 {} 兜住 —— 历史上被
    // 尾段通配吃进 /contributions/{id} 而判绿（'page' / 'cache' 这类分页缓存键误读）。
    name: '单段字面量（.get(\'page\')：尾段通配的假绿逃生口）',
    source: "await unwrappedRequest.get('page')",
    expect: /路径不是绝对路径（不以 \/ 开头、也无前导 base 占位），无法静态判定/
  },
  {
    name: '单段字面量（.delete(\'cache\')）',
    source: "await unwrappedRequest.delete('cache')",
    expect: /无法静态判定/
  }
]

for (const c of RED_CASES) {
  test('正样本（必须红）：' + c.name, () => {
    const violations = scanSource(c.source, API_FILE)
    assert.equal(violations.length, 1, '期望恰好 1 处违规，实得 ' + JSON.stringify(violations))
    assert.match(violations[0].reason, c.expect)
    if (c.expectPattern) assert.match(violations[0].pattern, c.expectPattern)
    assert.equal(violations[0].file, API_FILE)
  })
}

// ===== 判定面：负样本（必须绿）=====

const GREEN_SOURCES = [
  {
    name: '已登记端点（精确路径 + 参数名不同）',
    source: 'await unwrappedRequest.post(' + tpl('/admin/jobs/' + hole('jobId') + '/force-offline') + ', { reason })'
  },
  {
    name: '已登记端点（消费者少掉 baseURL 前缀：估值模块的 /dictionaries/*）',
    source: "await client.get('/dictionaries/brands')"
  },
  {
    name: '已登记端点（前导 base 插值：featured.ts 的 ${baseURL}/...）',
    source: 'await axios.post(' + tpl(hole('baseURL') + '/admin/featured-content/upload-image') + ', fd)'
  },
  {
    name: '已登记端点（字符串拼接 + 插值混合）',
    source: "await unwrappedRequest.post('/points/tasks/' + code + '/claim', undefined)"
  },
  {
    name: '请求层里没有路径字面量的调用（本次不命中，由上面的从严用例兜底）',
    source: 'const rows = new Map()'
  }
]

for (const c of GREEN_SOURCES) {
  test('负样本（必须绿）：' + c.name, () => {
    assert.deepEqual(scanSource(c.source, API_FILE), [])
  })
}

test('边界（必须绿）：测试 / 生成物 / 非请求层目录不进判定面', () => {
  const bad = "await unwrappedRequest.get('" + UNDECLARED + "')"
  assert.deepEqual(scanSource(bad, 'frontend/src/api/__tests__/demo.spec.ts'), [])
  assert.deepEqual(scanSource(bad, 'frontend/src/api/generated/job.ts'), [])
  assert.deepEqual(scanSource(bad, 'frontend/src/pages/admin/Demo.vue'), [])
  assert.equal(isGuardedPath('frontend/src/api/inspection.ts'), true)
  assert.equal(isGuardedPath('frontend/src/api/__tests__/inspection.spec.ts'), false)
  assert.equal(isGuardedPath(CONSUMER_DIR + '/generated/valuation.ts'), false)
})

test('真实 inspection.ts：8 个端点（职位治理 4 + 巡检 4）全部已登记 → 0 违规', () => {
  const file = CONSUMER_DIR + '/inspection.ts'
  assert.deepEqual(scanSource(readFileSync(resolve(ROOT, file), 'utf8'), file), [])
})

test('销账信号（必须红）：已登记端点仍挂在 ALLOWLIST 里', () => {
  // 合成一条「已登记端点 + 欠条」的情形：把 inspection.ts 里的 GET /admin/jobs 当成欠条键。
  const file = CONSUMER_DIR + '/inspection.ts'
  const key = file + '::GET /admin/jobs'
  ALLOWLIST[key] = '探针：已登记端点不得再挂欠条'
  try {
    const violations = scanSource("await unwrappedRequest.get('/admin/jobs')", file)
    assert.equal(violations.length, 1)
    assert.match(violations[0].reason, /ALLOWLIST 欠条已可销账/)
  } finally {
    delete ALLOWLIST[key]
  }
})

// ===== ALLOWLIST 卫生：不许有死条目（文件里已经没有这个消费）=====

/**
 * 摘掉条目后，该消费是否仍在这份源码里报红（true = 条目还活着：文件里确实还有这个消费，
 * 且没有豁免时它本身就是违规）。sourceOverride 只给下面的合成探针用。
 */
function allowlistEntryIsLive(key, sourceOverride) {
  const sep = key.indexOf('::')
  assert.ok(sep > 0, 'ALLOWLIST 键格式必须是 "<文件>::<METHOD> <模式>"：' + key)
  const file = key.slice(0, sep)
  const rest = key.slice(sep + 2)
  assert.equal(isGuardedPath(file), true, 'ALLOWLIST 文件不在守卫面：' + file)
  assert.match(rest, /^(GET|POST|PUT|DELETE|PATCH) \S/, 'ALLOWLIST 键缺 METHOD/模式：' + key)
  const source = sourceOverride ?? readFileSync(resolve(ROOT, file), 'utf8')
  const reason = ALLOWLIST[key]
  delete ALLOWLIST[key]
  try {
    return scanSource(source, file).some((v) => file + '::' + v.method + ' ' + v.pattern === key)
  } finally {
    ALLOWLIST[key] = reason
  }
}

test('ALLOWLIST 清零：#1120 销掉最后 4 条 createCrud 动态欠条', () => {
  assert.deepEqual(
    Object.keys(ALLOWLIST),
    [],
    'ALLOWLIST 应为空表；新增欠条要逐条写明理由（补一个销一个，见脚本头部）'
  )
})

test('ALLOWLIST 卫生：逐条可解释且不许有死条目（摘掉条目后该消费必须真的报红）', () => {
  for (const key of Object.keys(ALLOWLIST)) {
    assert.ok(allowlistEntryIsLive(key), 'ALLOWLIST 死条目（文件里已经没有这个消费，删掉它）：' + key)
  }
})

// 空表之后，「不许有死条目」这条判据靠下面两个合成探针继续钉住：判定面对「消费已消失」的
// 条目必须仍然敏感（否则将来补的欠条可以永远挂着不被发现）。
test('ALLOWLIST 死条目判定仍然有效（合成探针）：活条目判活、死条目判死', () => {
  const file = CONSUMER_DIR + '/inspection.ts'
  // 活条目：源码里确有这条消费，且摘掉欠条后它本身就是违规（未登记端点）→ 必须判活。
  const live = file + '::GET /admin/never-declared-1120'
  ALLOWLIST[live] = '探针：合成源码里消费了这个未登记端点'
  try {
    assert.equal(
      allowlistEntryIsLive(live, "await unwrappedRequest.get('/admin/never-declared-1120')"),
      true,
      '活条目必须判活'
    )
  } finally {
    delete ALLOWLIST[live]
  }
  // 死条目：真实源码里没有这个消费 → 必须判死（这是「不许有死条目」判据的底线）。
  const dead = file + '::GET /admin/never-consumed-1120'
  ALLOWLIST[dead] = '探针：这份源码里没有这个消费'
  try {
    assert.equal(allowlistEntryIsLive(dead), false, '死条目必须判死')
  } finally {
    delete ALLOWLIST[dead]
  }
})

// ===== 判据本身：声明表解析 fail-closed + 匹配语义 =====

test('parseDeclaredEndpoints：解析不出端点即抛错（fail-closed）', () => {
  assert.throws(() => parseDeclaredEndpoints('package apitypes\n// 没有任何端点'), /未解析出任何端点/)
})

test('parseDeclaredEndpoints：从声明表源码解出 method/path/域', () => {
  const declared = parseDeclaredEndpoints('var Domains = []Domain{\n{\n\t\tName:  "job",\n\t\tEndpoints: []Endpoint{\n\t\t\t{Method: "GET", Path: "/admin/jobs"},\n\t\t},\n},\n}')
  assert.equal(declared.size, 1)
  const ep = declared.get('GET /admin/jobs')
  assert.equal(ep.domain, 'job')
  assert.equal(ep.path, '/admin/jobs')
})

test('matchDeclared：参数名不同/前缀缺失/前导通配可命中；全通配与空模式不可判定', () => {
  const declared = parseDeclaredEndpoints('var Domains = []Domain{\n{\n\t\tName:  "job",\n\t\tEndpoints: []Endpoint{\n\t\t\t{Method: "GET", Path: "/recruit/jobs/{id}"},\n\t\t},\n},\n}')
  assert.ok(matchDeclared('GET', '/recruit/jobs/{}', declared))
  assert.ok(matchDeclared('GET', '/jobs/{}', declared), '少掉 base 前缀也要命中')
  assert.ok(matchDeclared('GET', '{}/recruit/jobs/{}', declared), '前导通配是 base 前缀')
  assert.equal(matchDeclared('GET', '{}', declared), null)
  assert.equal(matchDeclared('GET', '{}/{}', declared), null)
  assert.equal(matchDeclared('POST', '/recruit/jobs/{}', declared), null, '方法必须一致')
})

test('matchDeclared：声明侧参数位不得被消费侧字面量顶上（单段字面量的假绿逃生口）', () => {
  const declared = parseDeclaredEndpoints('var Domains = []Domain{\n{\n\t\tName:  "contribution",\n\t\tEndpoints: []Endpoint{\n\t\t\t{Method: "GET", Path: "/contributions/{id}"},\n\t\t},\n},\n}')
  // 'page' 只有一段、且声明末段是参数位：旧实现靠 decl === '{}' 通配判绿（假绿逃生口）。
  assert.equal(matchDeclared('GET', 'page', declared), null, '单段字面量不得被尾段通配吃进 /contributions/{id}')
  assert.equal(matchDeclared('GET', '/page', declared), null, '同形但带 / 前缀也只是「没命中」，不是「通配命中」')
  // 消费侧 {} 仍是通配（运行时才定的值），声明侧参数位不变。
  assert.ok(matchDeclared('GET', '/contributions/{}', declared))
  assert.ok(matchDeclared('GET', '{}/contributions/{}', declared), '前导 base 占位 + 消费侧参数位')
})

test('isAnchoredPattern：绝对路径 / 前导 base 占位可锚定；单段相对字面量不可', () => {
  assert.equal(isAnchoredPattern('/admin/jobs'), true)
  assert.equal(isAnchoredPattern('{}/admin/jobs'), true)
  assert.equal(isAnchoredPattern('page'), false)
  assert.equal(isAnchoredPattern('admin/jobs'), false)
})

test('真实声明表可解析（端点规模非零）', () => {
  const declared = parseDeclaredEndpoints(readFileSync(resolve(ROOT, DECLARATION_FILE), 'utf8'))
  assert.ok(declared.size > 100, '域声明表端点数 = ' + declared.size)
})

// ===== 端到端：--diff 只看新增行 =====

const RED_LINE = "await unwrappedRequest.get('" + UNDECLARED + "')"

test('--diff 正样本（必须红）：新增行消费未登记端点 → 退出 1', () => {
  const r = probeDiff(synthDiff(API_FILE, 3, [RED_LINE]), { [API_FILE]: 'line1\nline2\n' + RED_LINE })
  assert.equal(r.code, 1)
  assert.match(r.stderr, new RegExp(UNDECLARED.replace(/\//g, '\\/')))
  assert.match(r.stderr, /共 1 处/)
  assert.doesNotMatch(r.stdout + r.stderr, /通过。/)
})

test('--diff 正样本（必须红）：新增行是单段字面量（尾段通配的假绿逃生口）', () => {
  const line = "await unwrappedRequest.get('page')"
  const r = probeDiff(synthDiff(API_FILE, 3, [line]), { [API_FILE]: 'line1\nline2\n' + line })
  assert.equal(r.code, 1)
  assert.match(r.stderr, /无法静态判定/)
  assert.match(r.stderr, /共 1 处/)
})

test('--diff 负样本（必须绿）：新增行消费已登记端点', () => {
  const line = "await unwrappedRequest.get('/admin/jobs')"
  const r = probeDiff(synthDiff(API_FILE, 3, [line]), { [API_FILE]: 'line1\nline2\n' + line })
  assert.equal(r.code, 0)
  assert.match(r.stdout, /新增行消费的端点均已在域声明表内，通过。/)
})

test('--diff 负样本（必须绿）：违规行没被本次改动碰到 —— 只认新增行号', () => {
  const source = 'line1\n' + RED_LINE + '\nline3'
  const r = probeDiff(synthDiff(API_FILE, 1, ['line1']), { [API_FILE]: source })
  assert.equal(r.code, 0)
  assert.match(r.stdout, /通过。/)
})

test('--diff：diff 成功但没有新增行 → 打印「跳过」而不是通过语', () => {
  const r = probeDiff('')
  assert.equal(r.code, 0)
  assert.match(r.stdout, /相对 origin\/master 无 \.ts 新增行，跳过。/)
})

const FAIL_CLOSED = [
  { name: 'base 解析不了', overrides: { resolveBase: () => false }, expect: /无法解析 base ref: origin\/master/ },
  {
    name: 'git diff 失败',
    overrides: {
      readDiff: () => {
        throw new Error('git diff 失败: no merge base')
      }
    },
    expect: /git diff 失败: no merge base/
  },
  {
    name: '新增文件读不出来（非 ENOENT）',
    overrides: {
      readSource: () => {
        const e = new Error('EACCES: permission denied')
        e.code = 'EACCES'
        throw e
      }
    },
    expect: /读取新增文件失败/
  }
]

for (const c of FAIL_CLOSED) {
  test('fail-closed（必须非零退出、不得判绿）：' + c.name, () => {
    const r = probeDiff(synthDiff(API_FILE, 3, [RED_LINE]), { [API_FILE]: RED_LINE }, c.overrides)
    assert.equal(r.code, 2)
    assert.match(r.stderr, c.expect)
    assert.doesNotMatch(r.stdout + r.stderr, /通过。/)
  })
}

test('CLI 面：未知参数 → 用法 + 退出 2', () => {
  const out = []
  const err = []
  const code = runGuard(GUARD_SPEC, {
    argv: ['--bogus'],
    root: ROOT,
    stdout: (line) => out.push(line),
    stderr: (line) => err.push(line)
  })
  assert.equal(code, 2)
  assert.match(err.join('\n'), /用法: node scripts\/check-api-consumers\.mjs/)
})
