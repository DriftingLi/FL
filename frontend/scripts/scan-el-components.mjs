#!/usr/bin/env node
/**
 * 扫描每个 spec 需要注册的 EP 组件清单（#765 / #769，只读不改文件）。
 *
 * 用法：node scripts/scan-el-components.mjs [spec 路径...]
 *   无参数 = 扫 src 下全部 *.spec.ts
 *
 * 逻辑：对每个 spec，收集它 import 的本地 .vue 组件，再递归收集这些组件
 * import 的本地 .vue（spec 测页面、页面引封装层），从各 .vue 的 <template>
 * 里提取 <el-*，映射成 EP 的 El* 组件名输出。
 *
 * 产物供 T3/T4 迁移消费：清单即该 spec 的 epLite(...) 注册列表（epLite 内置
 * 白名单已覆盖全部，清单用于未来按需化与核对遗漏）。
 */
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, relative, dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const SRC = join(ROOT, 'src')

const kebabToPascal = (s) =>
  s.replace(/^el-/, '').replace(/-([a-z])/g, (_, c) => c.toUpperCase()).replace(/^./, (c) => c.toUpperCase())

function listSpecFiles(dir) {
  const out = []
  for (const name of readdirSync(dir)) {
    const p = join(dir, name)
    const st = statSync(p)
    if (st.isDirectory()) out.push(...listSpecFiles(p))
    else if (name.endsWith('.spec.ts')) out.push(p)
  }
  return out
}

/** 从一个 .vue/.ts/.spec.ts 文件里提取 import 的本地 .vue 路径（相对解析） */
function localVueImports(file) {
  const s = readFileSync(file, 'utf-8')
  const out = new Set()
  const re = /from\s+['"]([^'"]+\.vue)['"]/g
  let m
  while ((m = re.exec(s))) {
    const spec = m[1]
    if (spec.startsWith('@')) out.add(join(SRC, spec.slice(1) + (spec.endsWith('.vue') ? '' : '.vue')))
    else out.add(resolve(dirname(file), spec))
  }
  return [...out]
}

/** 提取 .vue 模板里的 el-* 组件名（El* 形式，去重） */
function elComponentsInVue(file) {
  const s = readFileSync(file, 'utf-8')
  const names = new Set()
  const re = /<el-[a-z-]+/g
  let m
  while ((m = re.exec(s))) names.add(kebabToPascal(m[0].slice(1)))
  return names
}

/** 递归收集 spec 可达的全部 .vue 及其模板里的 el-* */
function collectForSpec(specFile) {
  const seen = new Set()
  const queue = [specFile, ...localVueImports(specFile)]
  const names = new Set()
  while (queue.length) {
    const f = queue.pop()
    if (seen.has(f)) continue
    seen.add(f)
    if (f.endsWith('.vue')) {
      for (const n of elComponentsInVue(f)) names.add(n)
      queue.push(...localVueImports(f))
    }
  }
  return { files: seen.size, names: [...names].sort() }
}

const args = process.argv.slice(2)
const specs = args.length
  ? args.map((a) => resolve(a))
  : listSpecFiles(SRC)

for (const spec of specs) {
  const { files, names } = collectForSpec(spec)
  const rel = relative(ROOT, spec)
  console.log(`${rel}  (${files} files)`)
  console.log(`  ${names.join(', ') || '(无 el-* 组件)'}`)
}
