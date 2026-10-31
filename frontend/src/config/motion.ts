/**
 * 前端动效参数的**单点**（ADR-0079）。
 *
 * CSS 过渡仍走 `design-tokens.css` 的 `--duration-*` / `--ease-*`；这里只放
 * requestAnimationFrame / Web Animations API 驱动的动效参数，避免同一个手感散落在各组件里。
 */

/** 弹簧：morphicons 的预设名，或自定义刚度/阻尼。 */
export type UiSpring = 'snappy' | 'smooth' | 'bouncy' | { stiffness: number; damping: number }

/** 默认弹簧：snappy（≈420ms 收口、2.7% 过冲）。主题三态这类轻量切换用它。 */
export const UI_SPRING_DEFAULT: UiSpring = 'snappy'

/**
 * 降级口径：
 * - `user`（默认）：跟随系统 `prefers-reduced-motion`，开启时形变退化为瞬时切换；
 * - `always`：永远瞬时（用于高频、位置固定的切换）；
 * - `never`：无视系统设置（只在演示与测试里用）。
 *
 * 分工：CSS 过渡由 `global.css` 的媒体查询压平，rAF 动画由 `UiIcon` 自己压平 ——
 * 两者各自负责自己的动画面，不互相依赖，也不互相兜底。
 */
export type UiReducedMotion = 'never' | 'user' | 'always'

/** 默认降级口径。 */
export const UI_REDUCED_MOTION_DEFAULT: UiReducedMotion = 'user'

/**
 * 主题切换的圆形揭示（View Transitions API）：从触发图标中心扩散到整页。
 * 时长取 420ms、缓动与 --ease-default 同一条曲线；降低动效偏好由 utils/themeReveal 先行短路。
 */
export const THEME_REVEAL_DURATION_MS = 420
export const THEME_REVEAL_EASING = 'cubic-bezier(0.4, 0, 0.2, 1)'
