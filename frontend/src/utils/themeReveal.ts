/**
 * 主题切换的圆形揭示（View Transitions API）。
 *
 * 圈心 = 触发按钮中心，半径 = 圈心到视口最远角的距离 —— 所以无论入口在顶栏工具位、
 * 右上角固定位还是 AI 助手浮动胶囊里，扩散圆都能正好覆盖整页。
 *
 * 降级是刻意的：浏览器不支持 `document.startViewTransition`，或用户开启
 * `prefers-reduced-motion: reduce` 时，直接执行切换（不留下半截动画）。
 * 旧快照的整页交叉淡入由 global.css 关掉，否则圆外区域会先淡出再被圆盖住。
 */
import { THEME_REVEAL_DURATION_MS, THEME_REVEAL_EASING } from '@/config/motion'

export interface RevealOrigin {
  x: number
  y: number
}

export interface RevealViewport {
  width: number
  height: number
}

/** 圆心到视口四角的最大距离（覆盖整页所需半径）。 */
export function circleRevealRadius(origin: RevealOrigin, viewport: RevealViewport): number {
  const dx = Math.max(origin.x, viewport.width - origin.x)
  const dy = Math.max(origin.y, viewport.height - origin.y)
  return Math.hypot(dx, dy)
}

/** 触发元素中心；取不到元素或零尺寸时退到视口中心，保证始终有可用的圈心。 */
export function revealOriginOf(
  el: Element | null | undefined,
  viewport: RevealViewport
): RevealOrigin {
  if (!el) return { x: viewport.width / 2, y: viewport.height / 2 }
  const rect = el.getBoundingClientRect()
  if (rect.width === 0 && rect.height === 0) {
    return { x: viewport.width / 2, y: viewport.height / 2 }
  }
  return { x: rect.left + rect.width / 2, y: rect.top + rect.height / 2 }
}

export interface ThemeRevealOptions {
  /** 扩散时长（ms），缺省由 config/motion.ts 单点给。 */
  duration?: number
  /** 缓动，缺省与 `--ease-default` 同一条曲线（单点同 config/motion.ts）。 */
  easing?: string
}

/** 同一次揭示期间的守卫：连点时后一次直接切换，不排队叠两层快照。 */
let running = false

/**
 * 从 `originEl` 中心扩散一个圆到整页，并在回调里完成真正的主题切换。
 * `apply` 可以异步（等 Vue 把图标等 DOM 刷完再让浏览器拍新快照）。
 */
export function revealThemeFrom(
  originEl: Element | null | undefined,
  apply: () => void | Promise<void>,
  options: ThemeRevealOptions = {}
): void {
  const doc = typeof document === 'undefined' ? undefined : document
  const win = typeof window === 'undefined' ? undefined : window
  const reduced = win?.matchMedia?.('(prefers-reduced-motion: reduce)').matches === true
  const start = doc?.startViewTransition

  if (!doc || !win || reduced || typeof start !== 'function' || running) {
    void apply()
    return
  }

  const viewport = { width: win.innerWidth, height: win.innerHeight }
  const origin = revealOriginOf(originEl, viewport)
  const radius = circleRevealRadius(origin, viewport)

  // 先把圈心写进 CSS 变量：::view-transition-new(root) 的初始 clip 读它，
  // 保证新快照出现的第一帧就是「半径 0」，而不是整页新主题。
  const root = doc.documentElement
  root.style.setProperty('--theme-reveal-x', `${origin.x}px`)
  root.style.setProperty('--theme-reveal-y', `${origin.y}px`)
  // 主题菜单在选中后要淡出；让它立刻从过渡快照里消失，否则白底的菜单会在
  // 圆里停留一百多毫秒，看起来像闪了一下。
  root.classList.add('is-theme-revealing')

  let transition: ViewTransition
  try {
    transition = start.call(doc, async () => {
      await apply()
    })
  } catch {
    // 浏览器给了方法却拒绝启动（例如另一个 transition 正在跑）：直接切换，不吞掉主题变更
    root.classList.remove('is-theme-revealing')
    void apply()
    return
  }

  running = true
  transition.ready
    .then(() => {
      doc.documentElement.animate(
        {
          clipPath: [
            `circle(0px at ${origin.x}px ${origin.y}px)`,
            `circle(${radius}px at ${origin.x}px ${origin.y}px)`
          ]
        },
        {
          duration: options.duration ?? THEME_REVEAL_DURATION_MS,
          easing: options.easing ?? THEME_REVEAL_EASING,
          // fill: both —— 动画结束后保持满圆，直到浏览器移除过渡层；
          // 否则会回落到上面的初始 clip（0 半径），在收尾处再闪一帧。
          fill: 'both',
          pseudoElement: '::view-transition-new(root)'
        }
      )
    })
    .catch(() => {
      /* ready 被跳过（页面被导航等）：主题已在回调里切换，无需补偿 */
    })

  void transition.finished
    .catch(() => {
      /* 过渡被浏览器取消属于正常路径 */
    })
    .finally(() => {
      running = false
      // 过渡层已移除后再清变量，避免收尾帧读回 0 半径
      win.requestAnimationFrame(() => {
        root.style.removeProperty('--theme-reveal-x')
        root.style.removeProperty('--theme-reveal-y')
        root.classList.remove('is-theme-revealing')
      })
    })
}
