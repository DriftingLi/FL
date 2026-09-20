// 端点错误面守卫的抽取式单测（ADR-0060 决策 1 / 第十三波票 1b）。
//
// 守卫是「规则的执行面」，它自己坏了必须报红 —— 否则规则退化为假绿。
// 运行：node --test scripts/check-render-error-face.test.mjs
//
// 判定面只认一条：`Render:` 函数字面量的词法体内出现「渲染错误」的调用。这组用例把这些边界
// （闭包内 / 闭包外 / 字符串 / 注释 / 路径射程 / 名单与 response 包互等）钉住。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { spawnSync } from 'node:child_process'
import { runGuard } from './lib/guard.mjs'
import {
  ALLOWLIST,
  ERROR_ENVELOPE_FNS,
  FORBIDDEN,
  GUARDED_DIR_PREFIX,
  GUARD_SPEC,
  SKELETON_FILE,
  SUCCESS_ENVELOPE_FNS,
  exportedEnvelopeFns,
  isGuardedPath,
  isTestFile,
  scanSource,
  stripCommentsAndStrings
} from './check-render-error-face.mjs'

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const API_FILE = 'backend/internal/api/forum.go'

/** 收口后的形态（票1b）：Render 只写成功面，错误面挂在本端点的 ErrStatus 上。 */
const FIXED_SRC = [
  '\tEndpoint[listTopicsReq, service.ForumTopicPageResult]{',
  '\t\tParse: parseListTopicsReq,',
  '\t\tInvoke: h.svc.ListTopics,',
  '\t\tErrStatus: forumErrStatus,',
  '\t\tRender: func(c *gin.Context, _ *listTopicsReq, resp *service.ForumTopicPageResult) {',
  '\t\t\tresponse.Success(c, resp)',
  '\t\t},',
  '\t}.Handle(c)'
].join('\n')

/** 收口前的形态：错误分支写在 Render 里（本守卫的靶子）。 */
function legacySrc(bodyLines) {
  return [
    '\tEndpoint[req, resp]{',
    '\t\tInvoke: h.svc.Do,',
    '\t\tRender: func(c *gin.Context, _ *req, resp *resp, err error) {',
    ...bodyLines.map((l) => '\t\t\t' + l),
    '\t\t\tresponse.Success(c, resp)',
    '\t\t},',
    '\t}.Handle(c)'
  ].join('\n')
}

test('正例：五个错误信封写在 Render 闭包里都报，行号精确', () => {
  ERROR_ENVELOPE_FNS.forEach((fn, idx) => {
    const src = legacySrc(['if err != nil {', 'response.' + fn + '(c, err.Error())', 'return', '}'])
    const v = scanSource(src, API_FILE)
    assert.equal(v.length, 1, 'response.' + fn + ' 应报一处')
    assert.equal(v[0].line, 5, 'response.' + fn + ' 的行号')
    assert.match(v[0].why, new RegExp('response\\.' + fn))
    assert.equal(v[0].renderLine, 3, '报告要指认它属于哪个闭包')
  })
})

test('正例：renderStatus 与 .renderError（旧「闭包自查域表」）同样判红', () => {
  const a = scanSource(legacySrc(['renderStatus(c, http.StatusBadRequest, err.Error())']), API_FILE)
  assert.equal(a.length, 1)
  assert.match(a[0].why, /单一咽喉/)
  const b = scanSource(legacySrc(['if err != nil {', 'forumErrStatus.renderError(c, err)', 'return', '}']), API_FILE)
  assert.equal(b.length, 1)
  assert.match(b[0].why, /不再自查域表/)
})

test('正例：一个闭包里的多处违规逐处报（不按闭包折叠）', () => {
  const src = legacySrc([
    'if errors.Is(err, service.ErrX) {',
    'response.NotFound(c, "X 不存在")',
    'return',
    '}',
    'response.ServerError(c, err.Error())'
  ])
  const v = scanSource(src, API_FILE)
  assert.deepEqual(
    v.map((x) => x.line),
    [5, 8]
  )
})

test('负例：收口后的 Render（只写成功面）逐行不报', () => {
  assert.deepEqual(scanSource(FIXED_SRC, API_FILE), [])
})

test('负例：错误信封写在闭包之外（Parse / Invoke / raw handler）不进射程', () => {
  const src = [
    'func (h *Handler) Raw(c *gin.Context) {',
    '\tif err != nil {',
    '\t\tresponse.ServerError(c, err.Error())',
    '\t}',
    '\tEndpoint[req, resp]{',
    '\t\tParse: func(c *gin.Context) (*req, error) { return bindJSON[req](c) },',
    '\t\tInvoke: h.svc.Do,',
    '\t\tRender: func(c *gin.Context, _ *req, resp *resp) { response.SuccessWithMsg(c, "成功", resp) },',
    '\t}.Handle(c)',
    '}'
  ].join('\n')
  assert.deepEqual(scanSource(src, API_FILE), [])
})

test('负例：成功面调用与同名前缀的哨兵不误伤', () => {
  const cases = [
    'response.Success(c, resp)',
    'response.SuccessWithMsg(c, "修改成功", resp)',
    'response.Created(c, "发布成功", resp)',
    'if errors.Is(err, service.ErrForumReportNotFound) {',
    'return &resp{ErrNotFound: true}, nil',
    'return nil, badRequest("请求参数错误")'
  ]
  for (const line of cases) {
    assert.deepEqual(scanSource(legacySrc([line]), API_FILE), [], line + ' 不该报')
  }
})

test('负例：字符串与注释里的错误信封不算调用（且不打乱闭包边界）', () => {
  const src = [
    '// response.ServerError(c, err.Error()) 是旧写法（已收编）',
    '/*',
    ' Endpoint[X, Y]{Render: func(...){ response.NotFound(c, "x") }',
    '*/',
    'Endpoint[req, resp]{',
    '\t\tRender: func(c *gin.Context, _ *req, resp *resp) {',
    '\t\t\t// @Param body body object true "邮箱" example({"channel":"email","code":"1234"})',
    '\t\t\tresponse.SuccessWithMsg(c, "已删除 {占位大括号}", resp)',
    '\t\t},',
    '}'
  ].join('\n')
  assert.deepEqual(scanSource(src, API_FILE), [])
  // 剥壳面单测：字符串/行注释/块注释跨行
  const st = { inBlock: false }
  assert.equal(stripCommentsAndStrings('\t\t// response.ServerError(c, "x")', st).trim(), '')
  // 含反引号的字符串会被**保守地抹到行尾**（Go 原始字符串可跨行，宁多抹不漏抹），
  // 所以这里断言「调用文本不存在」而不是逐字符等值——后者会把保守性误报成缺陷。
  assert.equal(stripCommentsAndStrings('\tx := "response.NotFound(c, `a`) "', st).includes('NotFound'), false)
  assert.equal(stripCommentsAndStrings('/* a{b', st).trim(), '')
  assert.equal(st.inBlock, true)
  assert.equal(stripCommentsAndStrings('c) */ response.ServerError(c, err.Error())', st).trim(), 'response.ServerError(c, err.Error())')
  assert.equal(st.inBlock, false)
})

test('负例：闭包之后的错误信封不再算在闭包内（深度回到 0 即出闭包）', () => {
  const src = [
    FIXED_SRC,
    '\tresponse.ServerError(c, "闭包外")',
    '\tresponse.NotFound(c, "闭包外")'
  ].join('\n')
  assert.deepEqual(scanSource(src, API_FILE), [])
})

test('负例：射程外的路径整体放行（骨架自身 / 测试 / 其它层 / 非 Go）', () => {
  const line = 'response.ServerError(c, err.Error())'
  for (const p of [
    SKELETON_FILE,
    'backend/internal/api/endpoint_test.go',
    'backend/internal/api/forum_contract_test.go',
    'backend/internal/service/forum_service.go',
    'backend/pkg/response/response.go',
    'frontend/src/api/forum.ts'
  ]) {
    assert.equal(isGuardedPath(p), false, p + ' 不在守卫面')
    assert.deepEqual(scanSource(legacySrc([line]), p), [], p + ' 不进判定面')
  }
  for (const p of [API_FILE, 'backend/internal/api/admin.go', 'backend/internal/api/training_catalog.go']) {
    assert.equal(isGuardedPath(p), true, p + ' 应在守卫面')
  }
  assert.equal(isTestFile('backend/internal/api/endpoint_test.go'), true)
  assert.equal(GUARDED_DIR_PREFIX, 'backend/internal/api/')
})

test('一致性锁：错误/成功信封两份名单与 pkg/response 的导出函数互等（判据不许改回双份）', () => {
  const exported = new Set(exportedEnvelopeFns(readFileSync(join(ROOT, 'backend/pkg/response/response.go'), 'utf8')))
  const mine = new Set([...ERROR_ENVELOPE_FNS, ...SUCCESS_ENVELOPE_FNS])
  for (const fn of exported) assert.ok(mine.has(fn), 'response 包新增了未登记的信封函数: ' + fn)
  for (const fn of mine) assert.ok(exported.has(fn), '名单里有死条目（response 包已无此函数）: ' + fn)
  assert.equal(ERROR_ENVELOPE_FNS.includes('ServerError'), true)
  assert.equal(SUCCESS_ENVELOPE_FNS.includes('SuccessWithMsg'), true)
})

test('禁列与名单一致：每个错误信封都有一条禁列规则', () => {
  for (const fn of ERROR_ENVELOPE_FNS) {
    assert.ok(FORBIDDEN.some((f) => f.why.includes('response.' + fn)), '禁列缺少 response.' + fn)
  }
})

test('白名单：当前为空表（规则绝对执行）；例外文件必须仍在判定面内且理由非空', () => {
  // #1123：豁免从 isGuardedPath 移交给 runner（--all 整体放行 / --diff 只放行基线违规行号）。
  assert.deepEqual(ALLOWLIST, {}, '本守卫的存量是 0，加例外要先想清楚：--all 下例外是整文件放行')
  for (const [p, reason] of Object.entries(ALLOWLIST)) {
    assert.equal(isGuardedPath(p), true, p + ' 必须在判定面内（豁免交给 runner 逐行判）')
    assert.ok(scanSource(legacySrc(['response.ServerError(c, err.Error())']), p).length === 1, p + ' 的判定面必须真的扫得动')
    assert.ok(typeof reason === 'string' && reason.length > 10, p + ' 的理由要写清')
  }
})

test('判定面契约：GUARD_SPEC 用的就是本文件导出的判定函数（#1094 runner 可直接收）', () => {
  assert.equal(GUARD_SPEC.isGuardedPath, isGuardedPath)
  assert.equal(GUARD_SPEC.scanSource, scanSource)
  assert.equal(GUARD_SPEC.allowlist, ALLOWLIST)
  assert.equal(GUARD_SPEC.name, 'check-render-error-face')
  assert.deepEqual(GUARD_SPEC.all.extensions, ['.go'])
  assert.equal(typeof GUARD_SPEC.all.ok, 'function')
  assert.equal(typeof GUARD_SPEC.diff.ok, 'function')
})

test('端到端：守卫在当前工作树上判绿（票1b 已收编全部 Render 闭包）', () => {
  const out = spawnSync(process.execPath, ['scripts/check-render-error-face.mjs', '--all'], {
    cwd: ROOT,
    encoding: 'utf8'
  })
  assert.equal(out.status, 0, out.stdout + out.stderr)
  assert.match(out.stdout, /无违规/)
  // 守卫面非空：守卫坏了（例如把扫描目录或文件名判据写错）时不能靠「一个文件都没扫到」判绿
  assert.match(out.stdout, /[1-9][0-9]* 个 api 文件的 Render 闭包内均未出现错误信封调用/)
})

test('端到端：合成违规 api 文件必须报红（守卫真的扫得动，不是恒绿）', () => {
  const dir = mkdtempSync(join(tmpdir(), 'check-render-error-face-'))
  try {
    const rel = join('backend', 'internal', 'api')
    mkdirSync(join(dir, rel), { recursive: true })
    writeFileSync(
      join(dir, rel, 'stub.go'),
      'package api\n\nfunc (h *StubHandler) Get(c *gin.Context) {\n' + legacySrc(['response.ServerError(c, err.Error())', 'return', '}']) + '\n}\n',
      'utf8'
    )
    const lines = []
    const code = runGuard(GUARD_SPEC, {
      root: dir,
      argv: ['--all'],
      stdout: (l) => lines.push(l),
      stderr: (l) => lines.push(l)
    })
    assert.equal(code, 1, '有违规必须非零退出：' + lines.join('\n'))
    assert.match(lines.join('\n'), /api[/\\]stub\.go:7: Render 闭包（起于 :6）内错误信封 response\.ServerError/)
    assert.match(lines.join('\n'), /共 1 处/)
  } finally {
    rmSync(dir, { recursive: true, force: true })
  }
})

test('端到端：--diff fail-closed —— base 解析不了即非零退出，绝不落「通过」', () => {
  const out = spawnSync(
    process.execPath,
    ['scripts/check-render-error-face.mjs', '--diff', 'no-such-ref-for-guard-selftest'],
    { cwd: ROOT, encoding: 'utf8' }
  )
  assert.equal(out.status, 2)
  assert.match(out.stderr, /无法解析 base ref/)
})
