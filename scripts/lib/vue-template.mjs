/**
 * Vue 单文件组件「根 <template> 可见行」的唯一实现。
 *
 * 为什么单独成一个模块：`check-el-controls` 与 `check-icons` 各抄过一遍，`check-ui-intent`（#1982）
 * 是第三个消费面 —— 再抄一次就是三份「什么算模板」的定义，而判据分叉的代价是静默假绿
 * （同一份源码在 A 守卫里算模板、在 B 守卫里不算，谁也没法解释为什么）。
 *
 * 判据：
 *   - 只取根 `<template>` 块，`<script>` / `<style>` 不进面；
 *   - `<!-- -->` 注释按行剥离（跨行注释按状态机吃掉，注释里的控件名不算代码）；
 *   - 收尾取**最后一个**闭合 `</template>`：具名插槽的 `<template #footer>` 也会出现
 *     `</template>`，取第一个会在插槽处提前收尾、把文件后半段的模板静默漏检。
 */

/**
 * 取根 <template> 块的可见行（1-based 行号 + 去掉注释后的文本）。
 * 无根模板（如纯 script 组件）→ 空数组，即无判定面。
 */
export function visibleTemplateLines(source) {
  const lines = String(source).split('\n')
  let start = -1
  let end = -1
  for (let i = 0; i < lines.length; i++) {
    if (start < 0) {
      if (/^\s*<template[\s>]/.test(lines[i])) start = i + 1
      continue
    }
    if (/^\s*<\/template>\s*$/.test(lines[i])) end = i
  }
  if (start < 0) return []
  if (end < 0) end = lines.length

  const out = []
  let inComment = false
  for (let i = start; i < end; i++) {
    const raw = lines[i]
    let text = raw
    if (inComment) {
      const close = text.indexOf('-->')
      if (close < 0) continue
      text = text.slice(close + 3)
      inComment = false
    }
    // 同一行可能有多段注释：循环剥离
    for (;;) {
      const open = text.indexOf('<!--')
      if (open < 0) break
      const close = text.indexOf('-->', open + 4)
      if (close < 0) {
        text = text.slice(0, open)
        inComment = true
        break
      }
      text = text.slice(0, open) + ' ' + text.slice(close + 3)
    }
    out.push({ line: i + 1, text, raw })
  }
  return out
}
