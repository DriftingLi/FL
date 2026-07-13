import type { Directive, DirectiveBinding } from 'vue'

/**
 * v-reveal —— 滚动入场动画指令
 *
 * 用法：
 *   v-reveal                        → fadeInUp，无延迟（默认）
 *   v-reveal.fade                   → fadeIn
 *   v-reveal.slide-left             → slideInLeft
 *   v-reveal.slide-right            → slideInRight
 *   v-reveal.zoom                   → zoomIn
 *   v-reveal="100"                  → fadeInUp，延迟 100ms（数字 = 延迟毫秒，用于 v-for 错落）
 *   v-reveal.slide-left="200"       → slideInLeft，延迟 200ms
 *   v-reveal="{ delay: 150, animation: 'zoom', threshold: 0.2 }"  → 完整配置
 *
 * 动画类与关键帧定义在 global.css。使用 CSS animation（非 transition），
 * 以避免与组件 scoped 的 hover transition 特异性冲突。
 * 动画结束后通过 animationend 清理 reveal 类，使元素回归原生 CSS，恢复 hover 交互。
 */

interface RevealOptions {
  delay?: number
  animation?: string
  threshold?: number
  rootMargin?: string
}

interface RevealMeta {
  observer: IntersectionObserver | null
  onAnimEnd: (() => void) | null
}

// 动画名 → 变体类名（默认 fade-up 无需变体类，使用 .reveal.is-visible 的默认动画名）
const ANIMATION_VARIANTS: Record<string, string> = {
  'fade-up': '',
  'fade': 'reveal--fade',
  'slide-left': 'reveal--slide-left',
  'slide-right': 'reveal--slide-right',
  'zoom': 'reveal--zoom'
}

const metaMap = new WeakMap<HTMLElement, RevealMeta>()

function prefersReducedMotion(): boolean {
  return (
    typeof window !== 'undefined' &&
    typeof window.matchMedia === 'function' &&
    window.matchMedia('(prefers-reduced-motion: reduce)').matches
  )
}

function parseBinding(binding: DirectiveBinding): {
  animation: string
  delay: number
  threshold: number
  rootMargin: string
} {
  let animation = 'fade-up'
  let delay = 0
  let threshold = 0.15
  let rootMargin = '0px 0px -50px 0px'

  // 修饰符优先决定动画类型
  for (const key of Object.keys(binding.modifiers)) {
    if (key in ANIMATION_VARIANTS) {
      animation = key
    }
  }

  const value = binding.value
  if (typeof value === 'number') {
    delay = value
  } else if (typeof value === 'string') {
    if (value in ANIMATION_VARIANTS) animation = value
  } else if (value && typeof value === 'object') {
    const opts = value as RevealOptions
    if (typeof opts.delay === 'number') delay = opts.delay
    if (typeof opts.animation === 'string' && opts.animation in ANIMATION_VARIANTS) {
      animation = opts.animation
    }
    if (typeof opts.threshold === 'number') threshold = opts.threshold
    if (typeof opts.rootMargin === 'string') rootMargin = opts.rootMargin
  }

  return { animation, delay, threshold, rootMargin }
}

function removeRevealClasses(el: HTMLElement) {
  el.classList.remove('reveal', 'is-visible')
  // 移除所有 reveal-- 变体类
  const variants: string[] = []
  el.classList.forEach((c) => {
    if (c.startsWith('reveal--')) variants.push(c)
  })
  variants.forEach((c) => el.classList.remove(c))
  el.style.animationDelay = ''
}

function cleanupReveal(el: HTMLElement) {
  const meta = metaMap.get(el)
  if (!meta) return
  if (meta.observer) meta.observer.disconnect()
  if (meta.onAnimEnd) el.removeEventListener('animationend', meta.onAnimEnd)
  metaMap.delete(el)
}

export const vReveal: Directive<HTMLElement> = {
  mounted(el, binding) {
    // 无障碍：用户请求减少动画时，不添加任何 reveal 类，元素保持原生可见状态
    if (prefersReducedMotion()) return

    const { animation, delay, threshold, rootMargin } = parseBinding(binding)

    el.classList.add('reveal')
    const variant = ANIMATION_VARIANTS[animation]
    if (variant) el.classList.add(variant)
    if (delay > 0) el.style.animationDelay = `${delay}ms`

    // 动画播放完毕后清理 reveal 类，使元素回归原生 CSS，恢复 hover transition
    const onAnimEnd = () => {
      removeRevealClasses(el)
    }

    // 浏览器不支持 IntersectionObserver 时直接显示（无动画基础设施）
    if (typeof IntersectionObserver === 'undefined') {
      el.classList.add('is-visible')
      el.addEventListener('animationend', onAnimEnd, { once: true })
      metaMap.set(el, { observer: null, onAnimEnd })
      return
    }

    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            el.classList.add('is-visible')
            observer.unobserve(el) // 一次性触发，不重复动画
          }
        })
      },
      { threshold, rootMargin }
    )

    observer.observe(el)
    el.addEventListener('animationend', onAnimEnd, { once: true })
    metaMap.set(el, { observer, onAnimEnd })
  },
  unmounted(el) {
    cleanupReveal(el)
  }
}
