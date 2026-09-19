// 守卫 runner 单点的表驱动自测（ADR-0056 §5 / issue #1094）。
//
// 这一层是「runner 的 runner」：历史上 check-async-section 的 `--diff` 把
// `parseAddedLines(diffText)` 写成 `parseAddedLines(base, '*.vue')`，恒拿空 Map、恒 0 违规；
// CI 只跑 `--all`、自检只测判定函数 —— 假绿两侧无人接住。所以这里用**合成 diff** 端到端
// 驱动 runGuard：不碰 git、不碰真实工作树，正样本必须红、负样本必须绿、判据坏了必须非零退出，
// 并逐条钉住三个守卫原有的 CLI 面。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { isDirectRun, parseArgs, runGuard, walkFiles } from './lib/guard.mjs'
import { GUARD_SPEC as apiSeam } from './check-api-seam.mjs'
import { GUARD_SPEC as elControls } from './check-el-controls.mjs'
import { GUARD_SPEC as asyncSection } from './check-async-section.mjs'
import { GUARD_SPEC as apiConsumers } from './check-api-consumers.mjs'
import { GUARD_SPEC as aiAssistantSend } from './check-ai-assistant-send.mjs'

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..')

const PAGE = 'frontend/src/pages/student/Demo.vue'
const ADMIN = 'frontend/src/pages/admin/Demo.vue'

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
function probeDiff(spec, diff, sources = {}, overrides = {}) {
  const out = []
  const err = []
  const code = runGuard(spec, {
    argv: ['--diff', 'origin/master'],
    root: '/wave11-probe', // readDiff/readSource 都是注入的，root 只影响路径展示
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

// ===== 正样本：合成 diff 必须报出违规（历史上这三条里第三条恒绿）=====

const RED_CASES = [
  {
    name: 'api-seam：新增行直接 import 请求层',
    spec: apiSeam,
    file: ADMIN,
    added: ["import { unwrappedRequest } from '@/api/request'"],
    source: "<script setup lang=\"ts\">\nimport { unwrappedRequest } from '@/api/request'\n</script>",
    expect: /api\/request/
  },
  {
    name: 'el-controls：新增行裸用 el-dialog',
    spec: elControls,
    file: ADMIN,
    added: ['  <el-dialog v-model="visible" title="x" />'],
    source: '<template>\n  <el-dialog v-model="visible" title="x" />\n</template>',
    expect: /el-dialog/
  },
  {
    name: 'async-section：新增行手写四分支链（#1094 的病根）',
    spec: asyncSection,
    file: PAGE,
    added: ['  <UiErrorState v-if="loadError" />', '  <UiSkeleton v-else variant="list" />'],
    source: '<template>\n  <UiErrorState v-if="loadError" />\n  <UiSkeleton v-else variant="list" />\n</template>',
    expect: /UiAsyncSection/
  }
]

for (const c of RED_CASES) {
  test('正样本（必须红）：' + c.name, () => {
    const r = probeDiff(c.spec, synthDiff(c.file, 2, c.added), { [c.file]: c.source })
    assert.equal(r.code, 1, '合成 diff 命中违规必须退出 1（不得静默判绿）')
    assert.match(r.stderr, c.expect)
    assert.match(r.stderr, /共 1 处/)
    assert.doesNotMatch(r.stdout + r.stderr, /✓|通过。/, '报红时不得出现通过语')
  })
}

// ===== 负样本：不报违规 =====

const GREEN_CASES = [
  {
    name: 'api-seam：新增行只是普通 import',
    spec: apiSeam,
    file: ADMIN,
    added: ["import { ref } from 'vue'"],
    source: "<script setup lang=\"ts\">\nimport { ref } from 'vue'\n</script>",
    ok: /\[check-api-seam\] 新增行未直接引用请求层，通过。/
  },
  {
    name: 'el-controls：新增行是放行控件 el-table',
    spec: elControls,
    file: ADMIN,
    added: ['  <el-table :data="rows" />'],
    source: '<template>\n  <el-table :data="rows" />\n</template>',
    ok: /\[check-el-controls\] 新增行未裸用已收敛控件，通过。/
  },
  {
    name: 'async-section：新增行是迁移后的 UiAsyncSection',
    spec: asyncSection,
    file: PAGE,
    // 注：#1101 的 :empty= 规则只认 isEmpty（或已登记例外），样本必须用生产写法
    added: ['  <UiAsyncSection :error="loadError" :loading="loading" :empty="isEmpty" @retry="retry">'],
    source: '<template>\n  <UiAsyncSection :error="loadError" :loading="loading" :empty="isEmpty" @retry="retry">\n</template>',
    ok: /\[check-async-section\] 新增行未手写四分支链、composables\/ 无吞错第三实现，通过。/
  }
]

for (const c of GREEN_CASES) {
  test('负样本（必须绿）：' + c.name, () => {
    const r = probeDiff(c.spec, synthDiff(c.file, 2, c.added), { [c.file]: c.source })
    assert.equal(r.code, 0)
    assert.match(r.stdout, c.ok)
    assert.equal(r.stderr, '')
  })
}

test('负样本（必须绿）：违规行没被本次改动碰到 —— 只认新增行号', () => {
  const source = '<template>\n  <h1>标题</h1>\n  <UiErrorState v-if="loadError" />\n  <UiSkeleton v-else />\n</template>'
  const r = probeDiff(asyncSection, synthDiff(PAGE, 2, ['  <h1>标题</h1>']), { [PAGE]: source })
  assert.equal(r.code, 0)
  assert.match(r.stdout, /通过。/)
})

test('负样本（必须绿）：diff 成功但没有新增行 → 打印「跳过」而不是通过语', () => {
  const r = probeDiff(apiSeam, '')
  assert.equal(r.code, 0)
  assert.match(r.stdout, /相对 origin\/master 无 \.vue\/\.ts 新增行，跳过。/)
})

test('负样本（必须跳过）：纯删除的 diff（文件在、新增行集为空）→ 「跳过」而不是「通过」', () => {
  // -U0 的纯删除 hunk：+++ 头会留下 file → 空 Set 的条目；判据必须看**新增行集合为空**，
  // 只看 Map.size 会误判成「有新增行」，最后打印无中生有的通过语。
  const delDiff = [
    'diff --git a/' + PAGE + ' b/' + PAGE,
    '--- a/' + PAGE,
    '+++ b/' + PAGE,
    '@@ -3,1 +2,0 @@',
    '-  <UiErrorState v-if="e" />'
  ].join('\n')
  const r = probeDiff(asyncSection, delDiff)
  assert.equal(r.code, 0)
  assert.match(r.stdout, /相对 origin\/master 无 \.vue\/\.ts 新增行，跳过。/)
  assert.doesNotMatch(r.stdout, /通过。/)
})

test('边界（必须绿）：paths 不在守卫面的新增行不报', () => {
  // 守卫面外的路径（ui 封装层）不报
  const ui = 'frontend/src/components/ui/UiAsyncSection.vue'
  const r = probeDiff(asyncSection, synthDiff(ui, 2, ['  <UiErrorState v-if="e" />', '  <UiSkeleton v-else />']), { [ui]: 'x' })
  assert.equal(r.code, 0)
})

// ===== #1123：`--diff` 下 ALLOWLIST 收窄为行号级 =====
//
// 收窄前：例外文件在 `--diff` 里**整文件放行**（runner 按路径直接跳过 scanSource）⇒ 往存量例外
// 文件里新增违规永远不会被增量门接住（docs/agents/checks.md 登记过的已知洞）。
// 收窄后：只有**基线即违规的行号**放行；新增行上的违规照报；基线取不到即非零退出。

const ALLOWED_FILE = 'frontend/src/components/recruit/OnlineResumePdf.vue'
// 基线源码：第 2 行是已登记的存量违规（import 请求层），其余行无关
const ALLOWED_BASELINE = [
  '<script setup lang="ts">',
  "import { getValidAccessToken } from '@/api/client'",
  "import UiButton from '@/components/ui/UiButton.vue'",
  '</script>'
].join('\n')
const readBaseline = () => ALLOWED_BASELINE

test('正样本（必须红）：往 ALLOWLIST 例外文件的新增行注入违规（#1123 的核心判据）', () => {
  const injected = "import { unwrappedRequest } from '@/api/request'"
  const source = [
    '<script setup lang="ts">',
    "import { getValidAccessToken } from '@/api/client'",
    "import UiButton from '@/components/ui/UiButton.vue'",
    injected,
    '</script>'
  ].join('\n')
  const r = probeDiff(apiSeam, synthDiff(ALLOWED_FILE, 4, [injected]), { [ALLOWED_FILE]: source }, { readBaseline })
  assert.equal(r.code, 1, '例外文件的新增行违规必须判红（收窄前这里恒 ✓）')
  assert.match(r.stderr, /OnlineResumePdf\.vue:4/)
  assert.match(r.stderr, /api\/request/)
  assert.match(r.stderr, /共 1 处/)
  assert.doesNotMatch(r.stdout + r.stderr, /通过。/)
})

test('负样本（必须绿）：例外文件的存量违规行未被本次改动碰到（仍未改动的行不进判定）', () => {
  const addedLine = 'const endpoint = ref("")'
  const source = [
    '<script setup lang="ts">',
    "import { getValidAccessToken } from '@/api/client'", // 第 2 行：存量例外，未改动
    "import UiButton from '@/components/ui/UiButton.vue'",
    addedLine, // 第 4 行：本次新增，不违规
    '</script>'
  ].join('\n')
  const r = probeDiff(apiSeam, synthDiff(ALLOWED_FILE, 4, [addedLine]), { [ALLOWED_FILE]: source }, { readBaseline })
  assert.equal(r.code, 0)
  assert.match(r.stdout, /通过。/)
})

test('口径（必须绿）：基线即违规的行号仍放行 —— 例外是**行号级**的（票面口径的边界）', () => {
  // 同一行号在基线上就是违规 ⇒ 落在放行集合里。行号级口径的已知边界：原地改写一条存量例外行
  // 不会被增量门接住（内容级判定不属本票射程）；改动行号/新增行则会报。
  const rewritten = "import { getValidAccessToken } from '@/api/client' /* 仍走请求层 */"
  const source = ['<script setup lang="ts">', rewritten, '</script>'].join('\n')
  const r = probeDiff(apiSeam, synthDiff(ALLOWED_FILE, 2, [rewritten]), { [ALLOWED_FILE]: source }, { readBaseline })
  assert.equal(r.code, 0, '第 2 行在基线上就是违规 ⇒ 落在放行集合里')
  assert.match(r.stdout, /通过。/)
})

test('fail-closed（必须非零退出、不得判绿）：例外文件取不到基线内容', () => {
  const source = { [ALLOWED_FILE]: "import { unwrappedRequest } from '@/api/request'" }
  const diff = synthDiff(ALLOWED_FILE, 2, ["import { unwrappedRequest } from '@/api/request'"])
  const r = probeDiff(apiSeam, diff, source, {
    readBaseline: () => {
      throw new Error("fatal: path 'frontend/src/components/recruit/OnlineResumePdf.vue' does not exist in 'origin/master'")
    }
  })
  assert.equal(r.code, 2)
  assert.match(r.stderr, /取不到 ALLOWLIST 例外文件的基线内容: frontend\/src\/components\/recruit\/OnlineResumePdf\.vue @ origin\/master/)
  assert.doesNotMatch(r.stdout + r.stderr, /通过。|✓/)
})

test('fail-closed：基线内容不是文本（替身给错类型）→ 非零退出', () => {
  const source = { [ALLOWED_FILE]: "import { unwrappedRequest } from '@/api/request'" }
  const diff = synthDiff(ALLOWED_FILE, 2, ["import { unwrappedRequest } from '@/api/request'"])
  const r = probeDiff(apiSeam, diff, source, { readBaseline: () => undefined })
  assert.equal(r.code, 2)
  assert.match(r.stderr, /基线内容不是文本/)
})

// ===== fail-closed：判据坏了必须报错并非零退出（#1094 的另一半病根）=====

const RED_DIFF = synthDiff(PAGE, 2, ['  <UiErrorState v-if="e" />', '  <UiSkeleton v-else />'])
const RED_SRC = { [PAGE]: '<template>\n  <UiErrorState v-if="e" />\n  <UiSkeleton v-else />\n</template>' }

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
    expect: /读取新增文件失败: frontend\/src\/pages\/student\/Demo\.vue/
  }
]

for (const c of FAIL_CLOSED) {
  test('fail-closed（必须非零退出、不得判绿）：' + c.name, () => {
    const r = probeDiff(asyncSection, RED_DIFF, RED_SRC, c.overrides)
    assert.equal(r.code, 2)
    assert.match(r.stderr, c.expect)
    assert.doesNotMatch(r.stdout + r.stderr, /✓|通过。/, 'fail-closed 路径不得出现通过语')
  })
}

test('CLI fail-closed：--diff 指向不存在的 base → 退出 2，不打印 ✓', () => {
  let status = 0
  let stdout = ''
  let stderr = ''
  try {
    stdout = execFileSync(
      process.execPath,
      ['scripts/check-async-section.mjs', '--diff', 'origin/definitely-missing-ref-1094'],
      { cwd: ROOT, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] }
    )
  } catch (e) {
    status = e.status
    stdout = String(e.stdout ?? '')
    stderr = String(e.stderr ?? '')
  }
  assert.equal(status, 2, 'base 不存在必须非零退出（收敛前是 ✓ + 0）')
  assert.match(stderr, /无法解析 base ref: origin\/definitely-missing-ref-1094/)
  assert.doesNotMatch(stdout + stderr, /✓/)
})

// ===== CLI 面不变（三个守卫各自的入口形态）=====

// ===== 真实 git：runner ↔ added-lines 的默认连线（#1094 的病根）=====

test('真实 git（不注入 readDiff）：--diff <历史提交> 报违规/跳过，而不是恒绿', () => {
  // 收敛前 check-async-section 把 parseAddedLines(diffText) 写成 parseAddedLines(base, pathspec)，
  // --diff 恒拿空 Map；本用例不注入 readDiff/readSource，只给一个真仓库 + 真实历史提交，
  // 让 runGuard 走默认的 gitDiff/parseAddedLines 连线 —— 那条连线此后有回归锁。
  const repo = mkdtempSync(join(tmpdir(), 'guard-real-git-'))
  try {
    const git = (...args) =>
      execFileSync('git', args, { cwd: repo, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] })
    git('init', '-q')
    git('config', 'user.email', 'guard-test@example.com')
    git('config', 'user.name', 'guard-test')
    const rel = 'frontend/src/api/demo.ts'
    mkdirSync(join(repo, 'frontend/src/api'), { recursive: true })
    writeFileSync(join(repo, rel), 'line1\nline2\n')
    git('add', '.')
    git('-c', 'commit.gpgsign=false', 'commit', '-qm', 'base')
    const red = "await unwrappedRequest.get('/admin/never-declared-1094')"
    writeFileSync(join(repo, rel), 'line1\nline2\n' + red + '\n')
    git('add', '.')
    git('-c', 'commit.gpgsign=false', 'commit', '-qm', 'add undeclared consumer')

    const out = []
    const err = []
    const code = runGuard(apiConsumers, {
      argv: ['--diff', 'HEAD~1'],
      root: repo,
      stdout: (l) => out.push(l),
      stderr: (l) => err.push(l)
    })
    assert.equal(code, 1, '真实 git 链路上新增的未登记消费必须判红（收敛前这里是恒绿的死路径）')
    assert.match(err.join('\n'), /never-declared-1094/)
    assert.doesNotMatch(out.join('\n') + err.join('\n'), /跳过。|通过。/)

    // 同一链路的另一侧：base = HEAD（无新增行）→ 合法的绿是「跳过」，不是「通过」。
    const out2 = []
    const err2 = []
    const code2 = runGuard(apiConsumers, {
      argv: ['--diff', 'HEAD'],
      root: repo,
      stdout: (l) => out2.push(l),
      stderr: (l) => err2.push(l)
    })
    assert.equal(code2, 0)
    assert.match(out2.join('\n'), /相对 HEAD 无 \.ts 新增行，跳过。/)
  } finally {
    rmSync(repo, { recursive: true, force: true })
  }
})

test('真实 git（不注入 readBaseline）：例外文件的新增违规判红、存量违规行不进报告；缺基线则非零退出', () => {
  const repo = mkdtempSync(join(tmpdir(), 'guard-allowlist-baseline-'))
  try {
    const git = (...args) =>
      execFileSync('git', args, { cwd: repo, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] })
    git('init', '-q')
    git('config', 'user.email', 'guard-test@example.com')
    git('config', 'user.name', 'guard-test')
    writeFileSync(join(repo, 'README.md'), 'base\n')
    git('add', '.')
    git('-c', 'commit.gpgsign=false', 'commit', '-qm', 'base')
    mkdirSync(join(repo, 'frontend/src/components/recruit'), { recursive: true })
    const rel = ALLOWED_FILE
    const existing = "import { getValidAccessToken } from '@/api/client'"
    writeFileSync(join(repo, rel), '<script setup lang="ts">\n' + existing + '\n</script>\n')
    git('add', '.')
    git('-c', 'commit.gpgsign=false', 'commit', '-qm', '存量例外')
    const injected = "import { unwrappedRequest } from '@/api/request'"
    writeFileSync(join(repo, rel), '<script setup lang="ts">\n' + existing + '\n' + injected + '\n</script>\n')
    git('add', '.')
    git('-c', 'commit.gpgsign=false', 'commit', '-qm', '新增违规')

    const probe = (base) => {
      const out = []
      const err = []
      const code = runGuard(apiSeam, {
        argv: ['--diff', base],
        root: repo,
        stdout: (l) => out.push(l),
        stderr: (l) => err.push(l)
      })
      return { code, stdout: out.join('\n'), stderr: err.join('\n') }
    }
    const r = probe('HEAD~1') // base 树上有例外文件的基线（默认 readBaseline = git show）
    assert.equal(r.code, 1, '真实 git 链路上，例外文件的新增行违规必须判红')
    assert.match(r.stderr, /OnlineResumePdf\.vue:3/)
    assert.doesNotMatch(r.stderr, /OnlineResumePdf\.vue:2/, '存量例外行不进报告')
    assert.match(r.stderr, /共 1 处/)

    const r2 = probe('HEAD~2') // base 树上没有这个文件（例外路径 + 无基线）
    assert.equal(r2.code, 2, '例外文件缺基线必须 fail-closed（不得退回整文件放行）')
    assert.match(r2.stderr, /取不到 ALLOWLIST 例外文件的基线内容/)
    assert.doesNotMatch(r2.stdout + r2.stderr, /通过。/)
  } finally {
    rmSync(repo, { recursive: true, force: true })
  }
})

test('CLI 面不变：argv 解析表', () => {
  // 无参数：api-seam / el-controls 默认全量；async-section 打印用法（0）
  assert.deepEqual(parseArgs(apiSeam, []), { mode: 'all', scanDir: null })
  assert.deepEqual(parseArgs(elControls, []), { mode: 'all', scanDir: null })
  assert.deepEqual(parseArgs(asyncSection, []), { mode: 'usage', stream: 'stdout', code: 0 })
  // --all [目录]：前两个收目录，async-section 忽略
  assert.deepEqual(parseArgs(apiSeam, ['--all', 'frontend/src/api']), { mode: 'all', scanDir: 'frontend/src/api' })
  assert.deepEqual(parseArgs(elControls, ['--all', 'frontend/src/api']), { mode: 'all', scanDir: 'frontend/src/api' })
  assert.deepEqual(parseArgs(asyncSection, ['--all', 'frontend/src/api']), { mode: 'all', scanDir: null })
  // --diff [base]：缺省 origin/master
  assert.equal(parseArgs(apiSeam, ['--diff']).base, 'origin/master')
  assert.equal(parseArgs(apiSeam, ['--diff', 'origin/release']).base, 'origin/release')
  assert.equal(parseArgs(elControls, ['--diff', 'HEAD~1']).base, 'HEAD~1')
  assert.equal(parseArgs(asyncSection, ['--diff', 'HEAD~1']).base, 'HEAD~1')
  // --help：async-section 认（用法 + 0）；另两个按未知参数处理
  assert.deepEqual(parseArgs(asyncSection, ['--help']), { mode: 'usage', stream: 'stdout', code: 0 })
  assert.deepEqual(parseArgs(apiSeam, ['--help']), { mode: 'invalid', arg: '--help' })
})

test('CLI 面不变：用法 / 未知参数的出口流与退出码', () => {
  const cap = (spec, argv) => {
    const out = []
    const err = []
    const code = runGuard(spec, { argv, stdout: (l) => out.push(l), stderr: (l) => err.push(l) })
    return { code, stdout: out.join('\n'), stderr: err.join('\n') }
  }
  let r = cap(asyncSection, [])
  assert.equal(r.code, 0)
  assert.match(r.stdout, /用法: node scripts\/check-async-section\.mjs --all \| --diff \[base\]/)
  assert.equal(r.stderr, '')
  r = cap(apiSeam, ['--help'])
  assert.equal(r.code, 2)
  assert.match(r.stderr, /用法: node scripts\/check-api-seam\.mjs --all \[目录\] \| --diff \[base\]/)
  assert.equal(r.stdout, '')
  r = cap(asyncSection, ['--frobnicate'])
  assert.equal(r.code, 2)
  assert.match(r.stderr, /未知参数: --frobnicate/)
})

// ===== runner 的其余面（走查 / isDirectRun / 守卫契约）=====

test('走查：后缀过滤 / node_modules 跳过 / tolerateWalkErrors 开关', () => {
  const dir = mkdtempSync(join(tmpdir(), 'guard-walk-'))
  try {
    mkdirSync(join(dir, 'sub'))
    mkdirSync(join(dir, 'node_modules'))
    writeFileSync(join(dir, 'a.vue'), '')
    writeFileSync(join(dir, 'b.ts'), '')
    writeFileSync(join(dir, 'sub', 'c.vue'), '')
    writeFileSync(join(dir, 'node_modules', 'd.vue'), '')
    const rel = (files) => files.map((f) => f.slice(dir.length + 1).split('\\').join('/')).sort()
    assert.deepEqual(rel(walkFiles(dir, { extensions: ['.vue'] })), ['a.vue', 'node_modules/d.vue', 'sub/c.vue'])
    assert.deepEqual(rel(walkFiles(dir, { extensions: ['.vue'], skipNodeModules: true })), ['a.vue', 'sub/c.vue'])
    assert.deepEqual(rel(walkFiles(dir, { extensions: ['.vue', '.ts'], skipNodeModules: true })), ['a.vue', 'b.ts', 'sub/c.vue'])
    assert.deepEqual(walkFiles(join(dir, 'missing'), { extensions: ['.vue'], tolerateWalkErrors: true }), [])
    assert.throws(() => walkFiles(join(dir, 'missing'), { extensions: ['.vue'] }))
  } finally {
    rmSync(dir, { recursive: true, force: true })
  }
})

test('isDirectRun：只在 argv[1] 与模块 URL 相同时为真', () => {
  const saved = process.argv[1]
  try {
    process.argv[1] = join(tmpdir(), 'guard-direct-run-probe.mjs')
    assert.equal(isDirectRun(pathToFileURL(process.argv[1]).href), true)
    assert.equal(isDirectRun(pathToFileURL(join(tmpdir(), 'other.mjs')).href), false)
  } finally {
    process.argv[1] = saved
  }
})

test('契约：三个守卫都声明了 runner 需要的面（新增守卫只需实现 scanSource）', () => {
  for (const spec of [apiSeam, elControls, asyncSection]) {
    for (const key of ['name', 'usage', 'cli', 'all', 'diff']) {
      assert.ok(spec[key], spec.name + ' 缺 GUARD_SPEC.' + key)
    }
    assert.equal(typeof spec.isGuardedPath, 'function', spec.name + ' 需要 isGuardedPath')
    assert.equal(typeof spec.scanSource, 'function', spec.name + ' 需要 scanSource')
    assert.ok(spec.all.scanDir, spec.name + ' 需要 --all 的扫描目录')
    assert.ok(spec.all.extensions.length, spec.name + ' 需要走查后缀')
    assert.ok(spec.diff.pathspec.length, spec.name + ' 需要 --diff 的 pathspec')
    assert.equal(typeof spec.diff.defaultBase, 'string')
    assert.equal(typeof spec.all.violation, 'function')
    assert.equal(typeof spec.diff.violation, 'function')
  }
})

test('契约：#1123 —— 非空 ALLOWLIST 的例外文件必须在判定面内（否则 scanSource 恒空 = 行号级收窄静默失效）', () => {
  for (const spec of [apiSeam, elControls, asyncSection, apiConsumers, aiAssistantSend]) {
    for (const p of Object.keys(spec.allowlist ?? {})) {
      assert.equal(spec.isGuardedPath(p), true, spec.name + ' 的 ALLOWLIST 例外不在判定面内: ' + p)
    }
  }
})

test('集成：三个守卫的 --all 在当前工作树上为绿（不假红），措辞锁逐条对上', () => {
  const run = (script) => execFileSync(process.execPath, [script, '--all'], { cwd: ROOT, encoding: 'utf8' })
  assert.match(run('scripts/check-api-seam.mjs'), /无违规。[1-9][0-9]* 个文件均未直接引用请求层。/)
  assert.match(run('scripts/check-el-controls.mjs'), /无违规。[1-9][0-9]* 个单文件组件的模板均未裸用已收敛控件。/)
  // #1101/#1102 给 async-section 加了 :empty= 与 admin 两档两条规则，通过语随之扩展（措辞锁）
  assert.equal(run('scripts/check-async-section.mjs'), '✓ 未发现手写四分支链；:empty= 判据均来自 isEmpty（或已登记例外）；admin 页两档档位登记齐备；composables/ 无吞错的第三份列表实现\n')
})
