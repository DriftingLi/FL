// 主题圆形揭示的判据：圈心/半径是纯函数；浏览器能力与「减弱动效」的降级都在这里锁住。
// 视觉动画本身由无头浏览器观测（见 PR 证据），单测只守「用什么起点、什么时候不该动」。
import { afterEach, describe, expect, it, vi } from 'vitest'
import { circleRevealRadius, revealOriginOf, revealThemeFrom } from '../themeReveal'

const originalStart = Object.getOwnPropertyDescriptor(document, 'startViewTransition')
const originalAnimate = Object.getOwnPropertyDescriptor(document.documentElement, 'animate')

function fakeTransition(ready = true) {
  return {
    ready: ready ? Promise.resolve() : Promise.reject(new Error('skip')),
    finished: Promise.resolve()
  } as unknown as ViewTransition
}

afterEach(() => {
  vi.restoreAllMocks()
  if (originalStart) Object.defineProperty(document, 'startViewTransition', originalStart)
  else Reflect.deleteProperty(document, 'startViewTransition')
  if (originalAnimate) Object.defineProperty(document.documentElement, 'animate', originalAnimate)
  else Reflect.deleteProperty(document.documentElement, 'animate')
})

describe('圆形揭示的几何', () => {
  it('半径取圈心到视口最远角的距离（四个角都覆盖）', () => {
    expect(circleRevealRadius({ x: 100, y: 100 }, { width: 1000, height: 800 })).toBeCloseTo(
      Math.hypot(900, 700)
    )
    // 右下角对称：取 max，不是到固定角的距离
    expect(circleRevealRadius({ x: 900, y: 700 }, { width: 1000, height: 800 })).toBeCloseTo(
      Math.hypot(900, 700)
    )
    // 正中心 = 对角线一半
    expect(circleRevealRadius({ x: 500, y: 400 }, { width: 1000, height: 800 })).toBeCloseTo(
      Math.hypot(500, 400)
    )
  })

  it('圈心取元素中心；元素缺失 / 零尺寸退到视口中心', () => {
    const el = document.createElement('button')
    el.getBoundingClientRect = () =>
      ({ left: 100, top: 20, width: 20, height: 40 }) as DOMRect
    expect(revealOriginOf(el, { width: 1000, height: 800 })).toEqual({ x: 110, y: 40 })
    expect(revealOriginOf(null, { width: 1000, height: 800 })).toEqual({ x: 500, y: 400 })

    const zero = document.createElement('button')
    zero.getBoundingClientRect = () => ({ left: 0, top: 0, width: 0, height: 0 }) as DOMRect
    expect(revealOriginOf(zero, { width: 1000, height: 800 })).toEqual({ x: 500, y: 400 })
  })
})

describe('圆形揭示的降级', () => {
  it('没有 startViewTransition 时直接切换', () => {
    Reflect.deleteProperty(document, 'startViewTransition')
    const apply = vi.fn()
    revealThemeFrom(null, apply)
    expect(apply).toHaveBeenCalledTimes(1)
  })

  it('系统要求减弱动效时即使有 API 也直接切换', () => {
    vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: true } as MediaQueryList)
    const start = vi.fn(() => fakeTransition())
    Object.defineProperty(document, 'startViewTransition', {
      configurable: true,
      value: start
    })
    const apply = vi.fn()
    revealThemeFrom(null, apply)
    expect(start).not.toHaveBeenCalled()
    expect(apply).toHaveBeenCalledTimes(1)
  })

  it('有 API 时先跑 apply，再在 ::view-transition-new(root) 上挂圆扩散', async () => {
    vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: false } as MediaQueryList)
    const animate = vi.fn()
    Object.defineProperty(document.documentElement, 'animate', {
      configurable: true,
      value: animate
    })
    const start = vi.fn((cb: () => void | Promise<void>) => {
      // 真实 API 会在内部调用更新回调，ready 在回调完成后才 resolve
      const done = Promise.resolve().then(() => cb())
      return { ready: done, finished: Promise.resolve() } as unknown as ViewTransition
    })
    Object.defineProperty(document, 'startViewTransition', {
      configurable: true,
      value: start
    })

    const el = document.createElement('button')
    el.getBoundingClientRect = () =>
      ({ left: 100, top: 50, width: 20, height: 20 }) as DOMRect
    const apply = vi.fn()
    revealThemeFrom(el, apply)

    expect(start).toHaveBeenCalledTimes(1)
    // 圈心必须在 startViewTransition 之前落进 CSS 变量：新快照的初始 clip 读它
    expect(document.documentElement.style.getPropertyValue('--theme-reveal-x')).toBe('110px')
    expect(document.documentElement.style.getPropertyValue('--theme-reveal-y')).toBe('60px')
    expect(document.documentElement.classList.contains('is-theme-revealing')).toBe(true)
    await new Promise(resolve => setTimeout(resolve, 0))
    expect(apply).toHaveBeenCalledTimes(1)

    const [keyframes, options] = animate.mock.calls[0]
    expect(keyframes.clipPath[0]).toBe('circle(0px at 110px 60px)')
    expect(keyframes.clipPath[1]).toBe('circle(' + circleRevealRadius({ x: 110, y: 60 }, { width: window.innerWidth, height: window.innerHeight }) + 'px at 110px 60px)')
    expect(options).toMatchObject({
      pseudoElement: '::view-transition-new(root)',
      // 结束态保持满圆，避免回落到初始 0 半径
      fill: 'both'
    })
    expect(options.duration).toBe(420)

    // 过渡结束后清掉内联变量（rAF 后执行）
    await new Promise(resolve => setTimeout(resolve, 40))
    expect(document.documentElement.style.getPropertyValue('--theme-reveal-x')).toBe('')
    expect(document.documentElement.style.getPropertyValue('--theme-reveal-y')).toBe('')
    expect(document.documentElement.classList.contains('is-theme-revealing')).toBe(false)
  })
})
