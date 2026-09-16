// 门判据单点化的一致性锁（ADR-0053 §10 / spec #1053）。
//
// 收敛过的「同判据两处实现」必须**不许改回双份**：这里用文本级断言盯住
//   1) 两个 workflow 都调同一份 ci-summary 聚合脚本，且不再内联那段 jq；
//   2) 三个守卫的新增行解析都来自 scripts/lib/added-lines.mjs，没有第二份实现。
// 这类断言必须存在，否则下一次「顺手复制一份」在评审里几乎看不出来。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..')

const read = (rel) => readFileSync(join(ROOT, rel), 'utf8')

test('ci-summary 聚合：两个 workflow 都调单点脚本，且不再内联那份 jq', () => {
  for (const wf of ['.github/workflows/cd.yml', '.github/workflows/testing-smoke.yml']) {
    const src = read(wf)
    assert.ok(
      src.includes('scripts/ci-summary-status.sh'),
      wf + ' 应调用 scripts/ci-summary-status.sh（聚合的单点实现）'
    )
    assert.ok(
      !/select\(\.name=="ci-summary"\)/.test(src),
      wf + ' 不应再内联 ci-summary 的 jq（改回双份即报红）'
    )
  }
  // 那个脚本依赖 checkout 才存在：cd.yml 的 gate 原本就有，testing-smoke 是为它补的
  // （该 job 原先刻意不 checkout，脚本一上线它就 127——这条锁把这一课钉住）
  for (const wf of ['.github/workflows/cd.yml', '.github/workflows/testing-smoke.yml']) {
    const src = read(wf)
    assert.ok(/actions\/checkout@v4/.test(src), wf + ' 需要 checkout（聚合脚本在工作区里）')
  }
})

test('聚合脚本的判定分支与 ADR 记录的口径一致（failure > success > missing > other）', () => {
  const src = read('scripts/ci-summary-status.sh')
  for (const needle of ['"failure"', '"success"', '"missing"', '"other"']) {
    assert.ok(src.includes(needle), '聚合脚本缺分支 ' + needle)
  }
  assert.ok(src.includes('ci-summary'), '聚合脚本应筛 ci-summary 记录')
})

test('新增行解析：三个消费面都走 lib/added-lines.mjs，没有本地第二份实现', () => {
  const consumers = [
    'scripts/check-el-controls.mjs',
    'scripts/check-api-seam.mjs',
    'scripts/check-bare-hex.sh'
  ]
  for (const f of consumers) {
    const src = read(f)
    assert.ok(
      src.includes('added-lines.mjs'),
      f + ' 应引用 scripts/lib/added-lines.mjs（新增行解析的单点实现）'
    )
    // 本地再写一遍解析器的特征：自己匹配 hunk 头
    assert.ok(
      !/\^@@ -\[0-9\]\+\+/.test(src) && !/\+\[0-9\]\+/.test(src),
      f + ' 不应再有本地的 hunk 解析实现'
    )
  }
})

test('新增行解析的宿主是 Node（前端 job 没有 Go 工具链，ADR §10 的取舍）', () => {
  const ci = read('.github/workflows/ci.yml')
  // frontend-check job 只有 setup-node；引入 Go 工具会让该 job 多一套工具链时间
  assert.ok(ci.includes('node scripts/check-api-seam.mjs --all'), '前端 job 跑 api seam 守卫')
  assert.ok(ci.includes('node --test scripts/check-api-seam.test.mjs'), '自检 job 跑守卫自检')
  assert.ok(
    ci.includes('node --test scripts/lib/added-lines.test.mjs'),
    '自检 job 跑新增行解析的表驱动单测'
  )
})
