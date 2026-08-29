/**
 * 列表错峰进场（方案 11.5）
 *
 * 给每一项挂 `--i`（索引），配合全局的 `.stagger-in` 使用
 * `animation-delay: calc(var(--i) * 40ms)` 实现依次进场。
 *
 * 超过 `max` 后索引不再累加：长列表若逐项累加，末尾几项的延迟会拖到几百毫秒，
 * 看起来像卡住；封顶后末尾元素同时进场，视觉上仍是「有层次」而非「排队」。
 */
export const STAGGER_STEP_MS = 40
export const STAGGER_MAX_INDEX = 8

/**
 * 返回按索引生成内联变量的函数，供 `:style` 绑定。
 *
 * @param max 索引封顶值，默认 8（第 9 项起与第 8 项同时进场）
 */
export function useStagger(max: number = STAGGER_MAX_INDEX) {
  return (index: number): Record<string, string> => ({
    '--i': String(Math.min(Math.max(index, 0), max))
  })
}
